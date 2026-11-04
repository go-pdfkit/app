package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// splitterIn finds the splitter the view band is laid out with, which is what
// a reader drags. ⛔ Found by walking the band rather than kept in a field: a
// test that read a field would agree with a workbench that built the splitter
// and never put it on the screen.
func splitterIn(w toolkit.Widget) *toolkit.Splitter {
	if sp, ok := w.(*toolkit.Splitter); ok {
		return sp
	}
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, c := range p.Children() {
			if sp := splitterIn(c); sp != nil {
				return sp
			}
		}
	}
	return nil
}

func TestTheRailHasADividerAReaderCanMove(t *testing.T) {
	s, _ := opened(t, 3)
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("the band beside the page has no splitter, so the rail cannot be resized at all")
	}
	if sp.Handles() == 0 {
		t.Fatal("the splitter has no divider")
	}
	// The divider is a real control: it has a resize cursor and an accessible
	// name, which is what tells a reader it can be moved at all.
	r, ok := sp.HandleRect(0)
	if !ok || r.W <= 0 {
		t.Fatalf("the divider has no rectangle to grab: %+v", r)
	}
	// ⛔ HandleRect answers in SURFACE coordinates and Cursor asks for ones
	// LOCAL to the splitter, which it adds its own origin to. Passing the
	// surface point straight through counts the origin twice and lands
	// outside every divider — the mistake the toolkit's PageRect comment
	// exists to warn about, made here first and caught by this test.
	b := sp.Bounds()
	if got := sp.Cursor(r.X+r.W/2-b.X, r.Y+r.H/2-b.Y); got != "col-resize" {
		t.Errorf("the divider offers the cursor %q, not col-resize", got)
	}
}

// dragDivider does to the divider what a person does: press on it, move, let
// go. ⛔ Through the workbench's own three entry points, not by Setting the
// width — which is the difference between testing the rail and testing the
// gesture, and the difference this file shipped without.
func dragDivider(t *testing.T, s *state, by int) {
	t.Helper()
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("the band beside the page has no splitter")
	}
	r, ok := sp.HandleRect(0)
	if !ok || r.W <= 0 {
		t.Fatalf("the divider has no rectangle to grab: %+v", r)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	s.handleClick(x, y)
	s.handleMove(x+by, y)
	s.handleRelease(x+by, y)
}

func TestTheDividerCanActuallyBeDragged(t *testing.T) {
	// ⛔⛔ THE TEST THIS FEATURE SHIPPED WITHOUT, and the defect it hid was
	// total: the workbench sent EventMouseMove for every pointer move and
	// never EventMouseDrag, so a Splitter grabbed its divider on the press,
	// was told nothing while the pointer moved, and let go. The bar could not
	// be shifted a pixel.
	//
	// Every test around this one passed, because every one of them moved the
	// width by Setting the observable the splitter writes — the one thing a
	// person cannot do.
	s, _ := opened(t, 3)
	s.draw(buffer())
	before := s.railWidth.Get()

	dragDivider(t, s, toolkit.Scaled(40))

	if s.railWidth.Get() <= before {
		t.Errorf("the divider was dragged 40 to the right and the rail is still %d wide "+
			"(was %d)", s.railWidth.Get(), before)
	}
}

func TestDraggingTheDividerBackNarrowsItAgain(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	dragDivider(t, s, toolkit.Scaled(40))
	wide := s.railWidth.Get()
	s.refresh()
	s.draw(buffer())

	dragDivider(t, s, -toolkit.Scaled(30))
	if s.railWidth.Get() >= wide {
		t.Errorf("the divider was dragged back and the rail is %d wide (was %d)",
			s.railWidth.Get(), wide)
	}
}

func TestAPointerMoveWithNoButtonDownIsNotADrag(t *testing.T) {
	// ⛔ The other half of the same thing. A pointer passing over the divider
	// on its way somewhere must not move it, or the rail would follow the
	// mouse around the window.
	s, _ := opened(t, 3)
	s.draw(buffer())
	sp := splitterIn(s.view)
	r, _ := sp.HandleRect(0)
	before := s.railWidth.Get()

	s.handleMove(r.X+r.W/2, r.Y+r.H/2)
	s.handleMove(r.X+r.W/2+toolkit.Scaled(60), r.Y+r.H/2)
	if s.railWidth.Get() != before {
		t.Errorf("the pointer passed over the divider and the rail went from %d to %d",
			before, s.railWidth.Get())
	}
}

func TestWideningTheRailWidensTheMinipagesAndNarrowsThePage(t *testing.T) {
	// ⛔ The whole of what "resizable" has to mean. A rail that got wider while
	// its tiles stayed the same size would be a rail with a margin, and a page
	// that kept its width would be a page drawn under the rail.
	s, _ := opened(t, 3)
	beforePic, beforePage := s.picW(), s.pageW()
	if beforePic <= 0 || beforePage <= 0 {
		t.Fatalf("precondition: picture %d, page %d", beforePic, beforePage)
	}

	s.railWidth.Set(railLogicalW * 3 / 2)
	s.refresh()

	if s.picW() <= beforePic {
		t.Errorf("the rail was widened and its minipages are still %d across (were %d)",
			s.picW(), beforePic)
	}
	if s.pageW() >= beforePage {
		t.Errorf("the rail was widened and the page still has %d (had %d): they overlap",
			s.pageW(), beforePage)
	}
	// And the tiles themselves, not merely the arithmetic behind them.
	got := tiles(t, s)
	if len(got) != 3 {
		t.Fatalf("the rail has %d minipages", len(got))
	}
	if w := got[0].Bounds().W; w <= 0 {
		t.Fatalf("the first minipage is %d across", w)
	}
}

func TestNarrowingTheRailIsBoundedSoATileStaysWorthLookingAt(t *testing.T) {
	// A fixed pane the splitter will not take below its Min. Past the floor the
	// rail says less than its own scrollbar costs.
	s, _ := opened(t, 3)
	s.railWidth.Set(4) // far below the floor
	s.refresh()
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("no splitter")
	}
	// ⛔ Asserted on what was LAID OUT, not on the value that was set. The
	// observable is the reader's own number and nothing clamps it on the way
	// in; the floor is a property of the pane, and the pane is where it has to
	// show.
	if got, want := sp.PaneSize(0), toolkit.Scaled(railMinW); got < want {
		t.Errorf("the rail was laid out %d across, past its floor of %d", got, want)
	}
}

func TestThePicturesAreRemadeWhenTheDragEndsAndNotDuringIt(t *testing.T) {
	// ⛔ Each minipage is a page RENDERED at the rail's width. Re-rendering them
	// on every frame of a drag would render the whole window's worth of pages
	// dozens of times on the way to a width the reader has not chosen yet — so
	// moving the width alone must NOT throw the pictures away, and finishing
	// the gesture must.
	s, _ := opened(t, 3)
	s.draw(buffer())
	if len(s.thumbs.pix) == 0 {
		t.Fatal("precondition: the rail should hold some pictures")
	}

	s.railWidth.Set(railLogicalW + 20) // mid-drag
	if len(s.thumbs.pix) == 0 {
		t.Error("moving the divider threw the pictures away before the reader had finished")
	}
	if s.railView != nil {
		t.Error("moving the divider left the tiles laid out at the old width")
	}

	// ⛔ Through the splitter's own OnResized, not by calling remakeRail here.
	// Calling it directly tests the function and leaves the WIRING untested —
	// and a hook that was never attached, or attached to the wrong thing, does
	// nothing while looking exactly like one that works.
	sp := splitterIn(s.view)
	if sp == nil || sp.OnResized == nil {
		t.Fatal("the splitter tells nobody when a drag ends, so the pictures are never remade")
	}
	sp.OnResized([]int{0})
	if len(s.thumbs.pix) != 0 {
		t.Error("finishing the drag kept pictures made for the old width")
	}

	// And they come back at the new width.
	s.refresh()
	s.draw(buffer())
	for i, size := range s.thumbs.size {
		if w := size[0]; w < s.picW()-1 || w > s.picW()+1 {
			t.Errorf("minipage %d came back %d across, and the rail is now %d", i, w, s.picW())
		}
	}
}

func TestASingleSheetWithThePanelOpenStillHasADivider(t *testing.T) {
	// ⛔ The combination the band is easiest to get wrong on: no rail, because
	// one sheet is its own map, but a panel beside the page all the same. The
	// splitter is then built from two panes and one divider rather than three
	// and two, and the divider belongs to the PANEL.
	s, _ := opened(t, 1)
	openGroup(t, s, "Pages")
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("a single sheet with the panel open has no splitter at all")
	}
	if got := sp.Handles(); got != 1 {
		t.Errorf("two panes gave %d dividers", got)
	}
	if s.pageW() >= s.viewW() {
		t.Errorf("the page has %d of a band %d wide, and a panel is beside it",
			s.pageW(), s.viewW())
	}
}

func TestRemakingARailThatWasNeverBuiltIsSafe(t *testing.T) {
	// A one-page document has no rail at all, and a drag can be reported
	// against a band that has since changed.
	s, _ := opened(t, 1)
	if s.thumbs != nil && len(s.thumbs.pix) > 0 {
		t.Fatal("precondition: a single-sheet document should have no rail")
	}
	s.remakeRail() // must not panic
	if !s.dirty {
		t.Error("remaking the rail did not ask for a repaint")
	}
}
