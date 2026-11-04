package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// atRatio builds a workbench laid out for a display of the given pixel ratio,
// and puts the toolkit back afterwards.
func atRatio(t *testing.T, ratio float64, w, h int) workbench {
	t.Helper()
	t.Cleanup(restoreScale(t))
	a := newWorkbench(&fakeHost{})
	a.SetPixelRatio(ratio)
	a.Resize(w, h)
	return a
}

// bands are the workbench's own measurements at one pixel ratio.
type bands struct{ ribbon, rail, status, text, panel int }

// measure lays the workbench out at a ratio and reads it THERE.
//
// ⛔ MEASURED WHILE THE SCALE IS INSTALLED, not afterwards. The metric scale
// is one process-wide number: holding two workbenches and asking them both at
// the end asks them both at whichever scale was set last, and the answer comes
// back equal — which is exactly what a layout that ignores the display looks
// like, so the test would pass the defect it was written to catch.
func measure(t *testing.T, ratio float64) bands {
	t.Helper()
	w, h := int(float64(surfaceW)*ratio), int(float64(surfaceH)*ratio)
	a := atRatio(t, ratio, w, h)
	return bands{
		ribbon: a.s.toolbarH(),
		rail:   a.s.railPx(),
		status: statusH(),
		text:   toolkit.CurrentFont().Height(),
		panel:  panelW,
	}
}

// TestTheLayoutFollowsTheDisplaysPixelRatio is the test this workbench did not
// have, and the defect it did not have it for was an interface drawn at half
// the size it should be on every Retina screen.
//
// ⛔ It sized its FRAMEBUFFER by the ratio and scaled nothing else: the
// surface doubled and every widget on it stayed where it was. Sharp, and half
// as large. Its sibling playground had the complementary half of the same
// mistake — widgets scaled, surface not — and showed nothing at all.
func TestTheLayoutFollowsTheDisplaysPixelRatio(t *testing.T) {
	one := measure(t, 1)
	two := measure(t, 2)

	for _, c := range []struct {
		what     string
		at1, at2 int
	}{
		{"the ribbon", one.ribbon, two.ribbon},
		{"the minipage rail", one.rail, two.rail},
		{"the status line", one.status, two.status},
		{"the type", one.text, two.text},
		{"the tool panel", one.panel, two.panel},
	} {
		if c.at2 <= c.at1 {
			t.Errorf("%s is %d at ratio 1 and %d at ratio 2: it does not follow the display",
				c.what, c.at1, c.at2)
		}
	}
}

// TestEveryBandFitsTheSurfaceAtAnyRatio is the invariant, stated as a relation
// rather than as a number so it is true of any display.
func TestEveryBandFitsTheSurfaceAtAnyRatio(t *testing.T) {
	for _, ratio := range []float64{1, 1.5, 2, 3} {
		w, h := int(float64(surfaceW)*ratio), int(float64(surfaceH)*ratio)
		a := atRatio(t, ratio, w, h)
		s := a.s

		if s.viewTop() < s.toolbarH() {
			t.Errorf("ratio %g: the page band starts at %d and the ribbon ends at %d",
				ratio, s.viewTop(), s.toolbarH())
		}
		if s.viewH() <= 0 || s.viewW() <= 0 {
			t.Errorf("ratio %g: the page band is %dx%d", ratio, s.viewW(), s.viewH())
		}
		if s.viewTop()+s.viewH() > s.h-statusH() {
			t.Errorf("ratio %g: the page band ends at %d and the status line starts at %d",
				ratio, s.viewTop()+s.viewH(), s.h-statusH())
		}
		// The rail and the tool panel each have to leave the page something.
		if s.pageW() <= 0 {
			t.Errorf("ratio %g: the rail and the panel left the page %d pixels", ratio, s.pageW())
		}
	}
}

// TestTheWorkbenchIsDrawnAtEveryRatio paints it, because a rectangle of the
// right shape and a surface with something on it are different claims.
func TestTheWorkbenchIsDrawnAtEveryRatio(t *testing.T) {
	for _, ratio := range []float64{1, 2} {
		w, h := int(float64(surfaceW)*ratio), int(float64(surfaceH)*ratio)
		a := atRatio(t, ratio, w, h)
		a.s.host = &fakeHost{name: "sample.pdf", file: samplePDF(t, 3)}
		a.s.open()

		buf := make([]byte, 4*a.s.w*a.s.h)
		a.Draw(buf)
		if n := inked(buf, a.s.theme.Background); n == 0 {
			t.Fatalf("ratio %g: nothing was drawn at all", ratio)
		}
		// ⛔ The RIBBON's own band. Ink anywhere else — the page, the rail —
		// would otherwise stand in for the commands being drawn.
		band := inkIn(buf, a.s.w, 0, 0, a.s.w, a.s.toolbarH(), a.s.theme.Background)
		if band == 0 {
			t.Errorf("ratio %g: the ribbon band is bare", ratio)
		}
	}
}

// inkIn counts the pixels in a rectangle that are not the background.
func inkIn(buf []byte, w, x0, y0, rw, rh int, bg toolkit.RGBA) int {
	n := 0
	for y := y0; y < y0+rh; y++ {
		for x := x0; x < x0+rw; x++ {
			o := (y*w + x) * 4
			if o+3 >= len(buf) {
				continue
			}
			if buf[o] != bg.R || buf[o+1] != bg.G || buf[o+2] != bg.B {
				n++
			}
		}
	}
	return n
}

// TestTheSurfaceFloorFollowsTheDisplay is the original defect in one
// dimension: a floor of raw pixels would refuse a small window on a Retina
// screen and leave the workbench laid out larger than its own canvas.
func TestTheSurfaceFloorFollowsTheDisplay(t *testing.T) {
	one := func() int { defer restoreScale(t)(); return minSurfaceW() }()
	a := newWorkbench(&fakeHost{})
	t.Cleanup(restoreScale(t))
	a.SetPixelRatio(2)
	if two := minSurfaceW(); two <= one {
		t.Errorf("the smallest surface is %d at ratio 1 and %d at ratio 2", one, two)
	}
}

// TestTheRailIsRedrawnWhenTheDisplayChanges covers what a ratio change has to
// throw away: a minipage made for a card of the old size, stretched over the
// new one, is the one case where a bigger card is a blurrier one.
func TestTheRailIsRedrawnWhenTheDisplayChanges(t *testing.T) {
	t.Cleanup(restoreScale(t))
	a := newWorkbench(&fakeHost{name: "sample.pdf", file: samplePDF(t, 4)})
	a.Resize(surfaceW, surfaceH)
	a.s.open()
	a.Draw(make([]byte, 4*a.s.w*a.s.h))
	before, ok := a.s.thumbs.size[1]
	if !ok {
		t.Fatal("no minipage was drawn to begin with")
	}

	a.SetPixelRatio(2)
	a.Resize(2*surfaceW, 2*surfaceH)
	a.Draw(make([]byte, 4*a.s.w*a.s.h))
	after, ok := a.s.thumbs.size[1]
	if !ok {
		t.Fatal("the rail drew no minipage after the display changed")
	}
	// ⛔ MADE BIGGER, not stretched. The card is twice as wide in real pixels
	// now; a bitmap kept from the old one would be a 1x picture scaled up,
	// which is the one case where a bigger card really is a blurrier one.
	if after[0] <= before[0] {
		t.Errorf("a minipage was %d pixels wide at ratio 1 and %d at ratio 2",
			before[0], after[0])
	}
}
