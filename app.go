// The adapter between the workbench and the browser harness. Tag-less, so a
// native test can assert the contract is satisfied and drive every method.

package main

import (
	"github.com/go-opentype/fonts/ibmplexsans"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/webcanvas"
)

// workbench adapts the scene to the harness that owns the canvas and the
// events. Each method forwards to one handler: the mapping is a rename, not a
// rewrite.
type workbench struct {
	s *state
}

// maxRatio bounds how large the WIDGETS are drawn. A ratio is 1, 2 or 3 on
// every screen anyone has; a page can set it to anything through zoom, and a
// metric scale of 40 would be one control filling the window. The framebuffer
// is webcanvas's to size, and it bounds that itself.
const maxRatio = 3

// uiFontPx is the size the workbench sets its text at. Large enough to read on
// a dense toolbar, small enough that a button label still fits beside four
// others.
const uiFontPx = 13

// init installs the face before anything is laid out.
//
// ⛔ toolkit.SetFont is PROCESS-WIDE. Called from a constructor, as it was
// first, it leaks into every test that runs after the first one to build a
// workbench -- so the suite's result depended on its order, and a package that
// ships vector text was partly tested against a bitmap. One call, before main
// and before any test, is the only placement that makes the measured layout
// the shipped layout.
func init() { useVectorText() }

// useVectorText installs an anti-aliased, shaped face for every widget, at the
// metric scale currently installed.
//
// ⛔ The toolkit's compiled-in default is a 5x7 BITMAP font, and anti-aliased
// text is an explicit opt-in -- which this app had never made. So every label
// here was drawn from a bitmap: no antialiasing to be had at any pixel ratio,
// and no amount of drawing it at the screen's own resolution could make it
// look like type. Sharpening the canvas made the bitmap sharper, which is not
// the same thing.
//
// IBM Plex Sans, and the reason is a MEASUREMENT rather than a preference.
//
// ⛔ The face here was Inter, and a person reading the workbench said its
// lowercase g read as a 9. Deciding that by recalling which faces have a
// double-storey g is recall, not measurement, and it is wrong about half the
// time -- Arial, Lato, Open Sans and Source Sans are each misremembered one
// way or the other. So every family the fleet embeds was rendered, each glyph
// cropped to its own ink and scaled into one square, and the two compared:
//
//	sourcesans3           16.7%   ink shared between g and 9
//	montserrat            21.6%
//	ibmplexsans           24.2%
//	lato                  29.7%
//	atkinsonhyperlegible  53.3%
//	inter                 56.7%
//
// Inter is among the worst of the fifty-odd families, which is the complaint
// stated as a number. Two results were worth having:
//
//   - Atkinson Hyperlegible, the toolkit's bundled face and the obvious reach
//     for "most legible", would barely have helped. It is drawn to separate
//     the characters that are confused at a distance, and g against 9 is not
//     one of the pairs it separates.
//
//   - The sweep's winner is not the right answer either. A ranking of shapes
//     says nothing about what a face COSTS, and at the size where Source Sans
//     3 is as wide on the body as Inter 13px it is 23 pixels tall against
//     Inter's 16 -- seven pixels on every row, which is more than the panels
//     have to give.
//
// IBM Plex Sans separates them 2.3 times better than Inter, is the same width
// at the same size, and is one pixel taller on the line. Its g carries a
// pronounced hooked tail where the 9's descends straight, which is the
// difference a person actually sees.
//
// A parse failure leaves the bitmap in place rather than failing to start: a
// workbench that opens with plain text beats one that does not open.
func useVectorText() {
	f, err := toolkit.NewTrueTypeFont(uiFace, toolkit.Scaled(uiFontPx))
	if err != nil {
		return
	}
	toolkit.SetFont(f)
}

// uiFace is the typeface itself, a variable so a test can hand it something
// that is not one and watch the workbench carry on.
var uiFace = ibmplexsans.TTF

// newWorkbench builds the scene and wraps it.
//
// ⛔ It takes no pixel ratio any more. The ratio is the DISPLAY's, not the
// workbench's, and reading it here read it once — at startup, before the
// window had been dragged to another screen or the page zoomed. webcanvas
// tells the workbench through SetPixelRatio instead, and tells it again.
func newWorkbench(h host) workbench {
	return workbench{s: newState(surfaceW, surfaceH, h)}
}

// clampRatio keeps a ratio usable. A browser that reports nothing gives 0, and
// a zoomed page can report something large.
func clampRatio(r float64) float64 {
	switch {
	case !(r >= 1): // also catches NaN
		return 1
	case r > maxRatio:
		return maxRatio
	}
	return r
}

// Size reports the surface the workbench is laid out on before the page has
// said how big it is.
func (a workbench) Size() (int, int) { return surfaceW, surfaceH }

// SetPixelRatio tells the workbench how many device pixels the display puts in
// a CSS pixel, and is webcanvas's [webcanvas.PixelRatioAware] contract.
//
// ⛔ IT SCALES THE WIDGETS, WHICH IS WHAT THIS DID NOT DO. The workbench read
// the ratio itself and multiplied its FRAMEBUFFER by it, and stopped there:
// the surface doubled on a Retina display and every widget on it stayed the
// size it was, so the whole interface came out sharp and half as large as it
// should be. Its sibling playground had the complementary half of the same
// mistake — it scaled the widgets and not the surface, and showed nothing at
// all — which is how both were found.
//
// Saying so through the interface also gets the ratio CHANGING for free: a
// browser zoom, or a window dragged to a display of another density. The
// number this used to read at startup could never follow that.
func (a workbench) SetPixelRatio(ratio float64) {
	r := clampRatio(ratio)
	if r == toolkit.MetricScale() {
		return
	}
	toolkit.SetMetricScale(r)
	// The face is part of the scale, or the type is the one thing on the
	// surface that does not grow.
	useVectorText()
	rescaleLayout()
	// Every minipage already drawn was made for a card of the old size, so the
	// rail starts again rather than stretching bitmaps over bigger tiles.
	a.s.thumbs = nil
	a.s.refresh()
}

// Resize lays the workbench out on the box the page gave it and reports the
// framebuffer size to allocate.
//
// ⛔ The box arrives in DEVICE pixels now, because the scene claims
// [webcanvas.PixelRatioAware]: webcanvas sizes the backing store to the CSS box
// times the ratio and hands that on. This used to multiply by a ratio of its
// own, which was the right arithmetic reached the wrong way — nothing told the
// widgets about it.
func (a workbench) Resize(w, h int) (int, int) {
	if !a.s.resize(w, h) {
		// Refused, or already that size: keep what is laid out, and report it
		// so the harness allocates a framebuffer that matches the layout
		// rather than one the scene is not drawing into.
		return a.s.w, a.s.h
	}
	return w, h
}

// Draw paints the whole workbench.
func (a workbench) Draw(buf []byte) { a.s.draw(buf) }

// Click forwards a press.
func (a workbench) Click(x, y int) bool { return a.s.handleClick(x, y) }

// Move forwards a pointer move.
func (a workbench) Move(x, y int) bool { return a.s.handleMove(x, y) }

// Release forwards a release.
func (a workbench) Release(x, y int) bool { return a.s.handleRelease(x, y) }

// Context forwards a secondary press, which opens the menu of the verbs that
// act on the page under it.
//
// ⛔ It used to forward to handleClick — a right-click did whatever a
// left-click would, which on a minipage means it TURNED THE PAGE. The seam was
// here and wired to the wrong thing, which is worse than absent: the browser
// had already been told not to show its own menu, so the gesture did something
// surprising instead of nothing.
func (a workbench) Context(x, y int) bool { return a.s.handleContext(x, y) }

// Char forwards a printable character to whatever box is being typed into.
func (a workbench) Char(text string) bool { return a.s.handleChar(text) }

// KeyDown forwards a named key, which is how the pages are turned.
func (a workbench) KeyDown(key string) bool { return a.s.handleKeyDown(key) }

// AnimationStep repaints when the workbench has changed since the last frame.
// Nothing on this canvas moves by itself, so most frames change nothing and ask
// for nothing; what this is for is the file the browser hands over long after
// the press that asked for it, which no event follows.
func (a workbench) AnimationStep(dt float64) bool { return a.s.tick(dt) }

// the workbench satisfies the harness contract, and asks for a clock so that a
// document that arrives on its own is shown.
var (
	_ webcanvas.App             = workbench{}
	_ webcanvas.Animator        = workbench{}
	_ webcanvas.Resizer         = workbench{}
	_ webcanvas.PixelRatioAware = workbench{}
)
