// The adapter between the workbench and the browser harness. Tag-less, so a
// native test can assert the contract is satisfied and drive every method.

package main

import (
	"github.com/go-opentype/fonts/inter"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/webcanvas"
)

// workbench adapts the scene to the harness that owns the canvas and the
// events. Each method forwards to one handler: the mapping is a rename, not a
// rewrite.
type workbench struct {
	s *state
	// ratio is the screen's device pixels per CSS pixel. The harness hands
	// Resize a box in CSS pixels; what we draw into is a framebuffer, and a
	// framebuffer smaller than the screen is one the browser stretches. That
	// stretch is what made the text look soft on a high-density display.
	ratio float64
}

// maxRatio bounds the framebuffer. A ratio is 1, 2 or 3 on every screen anyone
// has; a page can set it to anything through zoom, and 4 bytes a pixel over a
// large window adds up.
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

// useVectorText installs an anti-aliased, shaped face for every widget.
//
// ⛔ The toolkit's compiled-in default is a 5x7 BITMAP font, and anti-aliased
// text is an explicit opt-in -- which this app had never made. So every label
// here was drawn from a bitmap: no antialiasing to be had at any pixel ratio,
// and no amount of drawing it at the screen's own resolution could make it
// look like type. Sharpening the canvas made the bitmap sharper, which is not
// the same thing.
//
// Inter rather than the toolkit's bundled Atkinson Hyperlegible: Atkinson is
// designed by the Braille Institute for maximum character distinction, which
// is the right default for a toolkit that cannot know its app, and reads as
// deliberately unusual in a dense tool. Inter is drawn for user interfaces,
// and is the face this fleet already sets its own marks in.
//
// A parse failure leaves the bitmap in place rather than failing to start: a
// workbench that opens with plain text beats one that does not open.
func useVectorText() {
	f, err := toolkit.NewTrueTypeFont(inter.TTF, uiFontPx)
	if err != nil {
		return
	}
	toolkit.SetFont(f)
}

// newWorkbench builds the scene and wraps it. ratio is the screen's device
// pixels per CSS pixel; a native caller passes 1.
func newWorkbench(h host, ratio float64) workbench {
	return workbench{s: newState(surfaceW, surfaceH, h), ratio: clampRatio(ratio)}
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

// Resize lays the workbench out on the box the page gave it, at the screen's
// own pixels, and reports the framebuffer size to allocate.
//
// The box arrives in CSS pixels. Multiplying by the ratio is the whole of the
// difference between text the browser stretches and text drawn where it is
// shown.
func (a workbench) Resize(w, h int) (int, int) {
	pw := int(float64(w) * a.ratio)
	ph := int(float64(h) * a.ratio)
	if !a.s.resize(pw, ph) {
		// Refused, or already that size: keep what is laid out, and report it
		// so the harness allocates a framebuffer that matches the layout
		// rather than one the scene is not drawing into.
		return a.s.w, a.s.h
	}
	return pw, ph
}

// Draw paints the whole workbench.
func (a workbench) Draw(buf []byte) { a.s.draw(buf) }

// Click forwards a press.
func (a workbench) Click(x, y int) bool { return a.s.handleClick(x, y) }

// Move forwards a pointer move.
func (a workbench) Move(x, y int) bool { return a.s.handleMove(x, y) }

// Release forwards a release.
func (a workbench) Release(x, y int) bool { return a.s.handleRelease(x, y) }

// Context forwards a secondary press, which the workbench treats as a primary
// one: nothing here has a second meaning.
func (a workbench) Context(x, y int) bool { return a.s.handleClick(x, y) }

// Char forwards a printable character to whatever box is being typed into.
func (a workbench) Char(text string) bool { return a.s.handleChar(text) }

// KeyDown forwards a named key, which is how the pages are turned.
func (a workbench) KeyDown(key string) bool { return a.s.handleKeyDown(key) }

// AnimationStep repaints when the workbench has changed since the last frame.
// Nothing on this canvas moves by itself, so most frames change nothing and ask
// for nothing; what this is for is the file the browser hands over long after
// the press that asked for it, which no event follows.
func (a workbench) AnimationStep(float64) bool { return a.s.takeDirty() }

// the workbench satisfies the harness contract, and asks for a clock so that a
// document that arrives on its own is shown.
var (
	_ webcanvas.App      = workbench{}
	_ webcanvas.Animator = workbench{}
	_ webcanvas.Resizer  = workbench{}
)
