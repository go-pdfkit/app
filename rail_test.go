package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// splitterIn lives in controls.go now: the workbench publishes the divider, so
// finding it is no longer something only a test needs to do.

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
	dragHandle(t, s, splitterIn(s.view), by, 0)
}

// dragHandle presses sp's first divider and moves it by (dx, dy).
func dragHandle(t *testing.T, s *state, sp *toolkit.Splitter, dx, dy int) {
	t.Helper()
	if sp == nil {
		t.Fatal("there is no splitter to drag")
	}
	r, ok := sp.HandleRect(0)
	if !ok || r.W <= 0 || r.H <= 0 {
		t.Fatalf("the divider has no rectangle to grab: %+v", r)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	s.handleClick(x, y)
	s.handleMove(x+dx, y+dy)
	s.handleRelease(x+dx, y+dy)
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

// ⛔⛔ THE HALF THE TEST ABOVE CALLS refresh() FOR, AND THE RUNNING WORKBENCH
// DOES NOT. Ending the drag drops the pictures so they can be made again at the
// new width; the test above then calls s.refresh() itself and watches them come
// back. Nothing in the application does: refresh is what a VERB runs, and
// widening the rail is not a verb. So for a release the reader was left with the
// pictures made for the old width, stretched, and the workbench published that
// it was holding no rail at all.
//
// The browser check found it, on the first run that could perform a drag. This
// is the in-process statement of it, and it uses the frame the workbench
// actually gets -- tick -- and never refresh.
func TestTheMinipagesComeBackByThemselvesAfterADrag(t *testing.T) {
	s, _ := opened(t, 6)
	s.draw(buffer())
	narrow := s.picW()

	dragDivider(t, s, toolkit.Scaled(40))
	if s.railView != nil {
		t.Fatal("precondition: ending the drag should have thrown the tiles away")
	}
	if s.picW() <= narrow {
		t.Fatalf("precondition: the rail is still %d wide after the drag (was %d)", s.picW(), narrow)
	}

	// Frames, which is all the running workbench ever gets. The rail fills in
	// over several of them, so it gets several — and no refresh.
	for i := 0; i < 8 && s.railView == nil; i++ {
		s.tick(1.0 / 60)
		s.draw(buffer())
	}
	if s.railView == nil {
		t.Fatal("the minipages never came back after the divider was dragged, " +
			"so the reader keeps the pictures made for the old width")
	}
	if len(s.thumbs.size) == 0 {
		t.Fatal("the rail came back holding no pictures at all")
	}
	for i, size := range s.thumbs.size {
		if w := size[0]; w < s.picW()-1 || w > s.picW()+1 {
			t.Errorf("minipage %d came back %d across, and the rail is now %d", i, w, s.picW())
		}
	}
}

// ⛔⛔ A FRAME BETWEEN TWO MOVES OF ONE DRAG. Every other test here sends press,
// move and release back to back, so the workbench never gets a frame in the
// middle of a gesture — and the running one gets a frame between every pair of
// pointer events. Rebuilding the band on such a frame builds a NEW Splitter, and
// the one holding the drag goes with the old one: the pointer keeps moving and
// nothing is listening.
//
// Measured in the browser, which is the only place a drag could be made at all:
// a 60-pixel drag moved the divider 7 pixels, one frame's worth, and stopped.
func TestAFrameInTheMiddleOfADragDoesNotCancelIt(t *testing.T) {
	s, _ := opened(t, 6)
	s.draw(buffer())
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("there is no splitter to drag")
	}
	r, ok := sp.HandleRect(0)
	if !ok {
		t.Fatal("the divider has no rectangle to grab")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	before := s.railWidth.Get()

	const steps = 6
	by := toolkit.Scaled(60)
	s.handleClick(x, y)
	for i := 1; i <= steps; i++ {
		s.handleMove(x+by*i/steps, y)
		// The frame. It is what the browser gives and what no other test here
		// gives, and it is the whole of this test.
		s.tick(1.0 / 60)
		s.draw(buffer())
	}
	s.handleRelease(x+by, y)

	// Logical pixels on both sides: railWidth is what the splitter writes, and
	// `by` was Scaled for the surface.
	if got := s.railWidth.Get() - before; got < 45 {
		t.Fatalf("a 60-pixel drag with a frame between each move moved the divider %d; "+
			"the gesture was cancelled by the repaint under it", got)
	}
}

// The control in the other direction: a document with no rail must not have one
// built for it every frame, or widening nothing would repaint for ever.
func TestASingleSheetIsNotGivenARailByTheFrameLoop(t *testing.T) {
	s, _ := opened(t, 1)
	s.draw(buffer())
	s.takeDirty()
	for i := 0; i < 3; i++ {
		if s.tick(1.0 / 60) {
			t.Fatal("a single sheet asked to be repainted with nothing happening to it")
		}
	}
	if s.railView != nil {
		t.Error("a single sheet was given a rail of one minipage")
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
