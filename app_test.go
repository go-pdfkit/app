package main

import (
	"math"
	"testing"
)

func TestTheAdapterForwardsEveryEvent(t *testing.T) {
	h := &fakeHost{name: "sample.pdf", file: samplePDF(t, 2)}
	a := newWorkbench(h, 1)
	if w, hgt := a.Size(); w != surfaceW || hgt != surfaceH {
		t.Errorf("Size() = %dx%d", w, hgt)
	}
	buf := buffer()
	a.Draw(buf)
	if inked(buf, a.s.theme.Background) == 0 {
		t.Error("Draw painted nothing")
	}
	// Every event method reaches the scene and says whether it mattered.
	if !a.Click(10, 10) || !a.Move(10, 10) || !a.Release(10, 10) || !a.Context(10, 10) {
		t.Error("a pointer event was not taken")
	}
	if a.Char("x") {
		t.Error("the workbench claimed a character it has nowhere to put")
	}
	a.s.open()
	if !a.KeyDown("ArrowRight") {
		t.Error("the right arrow was not taken")
	}
	if a.s.at != 2 {
		t.Errorf("the arrow took it to page %d", a.s.at)
	}
	if a.KeyDown("KeyQ") {
		t.Error("a key with nothing to do was claimed")
	}
}

func TestAFileThatArrivesOnItsOwnIsShown(t *testing.T) {
	// The browser hands a file over well after the press that asked for it,
	// with no event of its own. The harness asks for a frame instead, and
	// the workbench has to say that something changed — once, and not again
	// while nothing does.
	h := &fakeHost{name: "sample.pdf", file: samplePDF(t, 2)}
	a := newWorkbench(h, 1)
	a.AnimationStep(0.016) // whatever building it changed
	if a.AnimationStep(0.016) {
		t.Error("a still workbench asked to be repainted")
	}
	a.s.open()
	if !a.AnimationStep(0.016) {
		t.Error("a document that arrived on its own was not shown")
	}
	if a.AnimationStep(0.016) {
		t.Error("the same document asked to be shown twice")
	}
}

// ⛔ The box arrives in CSS pixels and the framebuffer is in DEVICE pixels.
// Drawing at one device pixel per CSS pixel on a 2x screen means the browser
// stretches every glyph, which is what made the text look soft; multiplying by
// the ratio is the whole of the fix.
func TestResizeDrawsAtTheScreensOwnPixels(t *testing.T) {
	h := &fakeHost{}
	a := newWorkbench(h, 2)

	w, hh := a.Resize(600, 500)
	if w != 1200 || hh != 1000 {
		t.Errorf("Resize(600,500) at ratio 2 = %d x %d, want 1200 x 1000", w, hh)
	}
	if a.s.w != 1200 || a.s.h != 1000 {
		t.Errorf("the scene was laid out on %d x %d", a.s.w, a.s.h)
	}
	// and the bands follow it, which is what "fills the page" means
	if got := a.s.viewW(); got != 1200-2*margin {
		t.Errorf("viewW = %d, want the new width less the margins", got)
	}
	if got := a.s.viewH(); got != 1000-viewTop-statusH-margin {
		t.Errorf("viewH = %d, want the new height less the bands", got)
	}
}

// A ratio of one changes nothing, which is what a native caller and an old
// display both get.
func TestResizeAtRatioOneIsTheBoxItself(t *testing.T) {
	a := newWorkbench(&fakeHost{}, 1)
	if w, h := a.Resize(820, 640); w != 820 || h != 640 {
		t.Errorf("= %d x %d, want 820 x 640", w, h)
	}
}

// ⛔ A surface too small to hold the bands is REFUSED, and the size reported
// back is the one the scene is still laid out on -- not the one that was
// asked for. Reporting the asked-for size would have the harness allocate a
// framebuffer the scene never draws into, which is a window of uninitialised
// pixels.
func TestASurfaceTooSmallLeavesTheLastGoodLayoutStanding(t *testing.T) {
	a := newWorkbench(&fakeHost{}, 1)
	a.Resize(900, 700)

	for _, c := range []struct{ w, h int }{{1, 700}, {900, 1}, {0, 0}} {
		w, h := a.Resize(c.w, c.h)
		if w != 900 || h != 700 {
			t.Errorf("Resize(%d,%d) reported %d x %d, want the layout that still stands", c.w, c.h, w, h)
		}
		if a.s.w != 900 || a.s.h != 700 {
			t.Errorf("Resize(%d,%d) relaid the scene to %d x %d", c.w, c.h, a.s.w, a.s.h)
		}
	}
}

// Asking for the size it already has is not a relayout: the harness would
// reallocate the framebuffer and repaint for nothing on every resize event a
// browser fires while a window is being dragged.
func TestResizingToTheSameSizeChangesNothing(t *testing.T) {
	a := newWorkbench(&fakeHost{}, 1)
	a.Resize(900, 700)
	if a.s.resize(900, 700) {
		t.Error("the same size was reported as a change")
	}
	if w, h := a.Resize(900, 700); w != 900 || h != 700 {
		t.Errorf("= %d x %d, want the size it already had", w, h)
	}
}

// A browser that says nothing, and a page zoomed past anything real.
func TestTheRatioIsKeptUsable(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want float64
	}{
		{0, 1}, {-2, 1}, {0.5, 1}, {1, 1}, {2, 2}, {3, 3},
		{4, maxRatio}, {1e9, maxRatio}, {math.NaN(), 1},
	} {
		if got := clampRatio(c.in); got != c.want {
			t.Errorf("clampRatio(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// and it reaches the workbench
	if a := newWorkbench(&fakeHost{}, 0); a.ratio != 1 {
		t.Errorf("ratio = %v, want 1", a.ratio)
	}
	if a := newWorkbench(&fakeHost{}, 99); a.ratio != maxRatio {
		t.Errorf("ratio = %v, want %v", a.ratio, maxRatio)
	}
}
