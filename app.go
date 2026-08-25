// The adapter between the workbench and the browser harness. Tag-less, so a
// native test can assert the contract is satisfied and drive every method.

package main

import "github.com/go-widgets/webcanvas"

// workbench adapts the scene to the harness that owns the canvas and the
// events. Each method forwards to one handler: the mapping is a rename, not a
// rewrite.
type workbench struct{ s *state }

// newWorkbench builds the scene and wraps it.
func newWorkbench(h host) workbench {
	return workbench{s: newState(surfaceW, surfaceH, h)}
}

// Size reports the surface the workbench is laid out on.
func (a workbench) Size() (int, int) { return surfaceW, surfaceH }

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

// Char forwards a printable character; the workbench has nothing to type into.
func (a workbench) Char(string) bool { return false }

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
)
