// The adapter between the workbench and the browser harness. Tag-less, so a
// native test can assert the contract is satisfied and drive every method.

package main

import "github.com/go-widgets/webcanvas"

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
