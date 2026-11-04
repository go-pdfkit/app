package main

import (
	"errors"
	"testing"
	"time"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// landscape builds a one-page document wider than it is tall, which is the
// shape every "fit" decision reverses on.
func landscape(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(800), reader.Integer(200)},
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 0 0 50 50 re f")}),
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEveryPageOfTheDocumentIsLaidOutEvenThoughFewAreDrawn(t *testing.T) {
	// ⛔ The whole point of declaring sizes and filling bitmaps separately. A
	// viewer that only knew about the pages it had drawn would give the reader
	// a scrollbar describing a six-page document and a page number that meant
	// nothing.
	s, _ := opened(t, 12)
	if got := s.pv.PageCount(); got != 12 {
		t.Fatalf("the viewer lays out %d pages of a 12-page document", got)
	}
	drawn := 0
	for i := 1; i <= 12; i++ {
		if s.drawn[i] > 0 {
			drawn++
		}
	}
	if drawn == 0 {
		t.Error("twelve pages laid out and not one of them drawn")
	}
	if drawn == 12 {
		t.Error("every page of the document was drawn: the window is not bounding anything")
	}
}

func TestTheFirstRepaintOfADocumentDrawsIt(t *testing.T) {
	// The plainest thing a viewer has to do, and the one a viewer rebuilt
	// around a cache can stop doing: open a document and have a page in it.
	// Both shapes, because the second is the case that survives when the first
	// is drawn against geometry left over from another document.
	s, _ := opened(t, 3)
	if !s.pageShown() {
		t.Fatal("a freshly opened document drew no page at all")
	}
	// And again for a different document, which is the case that fails when
	// the stale geometry happens to fit.
	h := &fakeHost{name: "wide.pdf", file: landscape(t)}
	s2 := newState(surfaceW, surfaceH, h)
	s2.open()
	if !s2.pageShown() {
		t.Error("a document of a different shape drew no page")
	}
}

func TestScrollingTheViewerMovesTheWorkbench(t *testing.T) {
	// ⛔ The direction a plain field could not have. The viewer's own prev/next,
	// a nav key and a wheel notch all move the reader without going through any
	// verb, so without this the rail would light the wrong tile and every verb
	// would act on a page somebody had scrolled away from.
	s, _ := opened(t, 5)
	if s.at != 1 {
		t.Fatalf("precondition: at page %d", s.at)
	}
	s.pv.CurrentPage().Set(4)
	if s.at != 4 {
		t.Errorf("the viewer moved to page 4 and the workbench is on %d", s.at)
	}
	if s.railView != nil {
		t.Error("the rail was kept after the page moved, so it still lights the old tile")
	}
	// Back to a page the workbench is already on: nothing to do, and no
	// pretending something happened.
	s.dirty = false
	s.pv.CurrentPage().Set(4)
	if s.dirty {
		t.Error("setting the page it was already on asked for a repaint")
	}
}

func TestZoomingInDrawsThePageAgainRatherThanEnlargingIt(t *testing.T) {
	// ⛔ This is the difference between a viewer and a picture, and it is
	// invisible in a screenshot taken at one zoom: PagedView scales a bitmap to
	// the card it declared, so a page drawn once and then zoomed is that first
	// drawing enlarged — soft, and softest exactly when somebody has zoomed in
	// to look closely.
	s, _ := opened(t, 1)
	before := s.drawn[1]
	if before <= 0 {
		t.Fatal("precondition: page 1 should have been drawn")
	}
	s.pv.SetFit(toolkit.PagedFitNone)
	s.pv.Zoom().Set(s.pv.Zoom().Get() * 2)
	s.pv.SetBounds(s.pv.Bounds()) // what a repaint does
	s.showPages(s.src, s.pvDoc)

	if after := s.drawn[1]; after <= before {
		t.Errorf("the page was drawn %d wide, zoomed, and is still drawn %d wide: "+
			"zooming enlarged a rendering instead of making one", before, after)
	}
}

func TestRepaintingWithoutChangingAnythingDrawsNothingAgain(t *testing.T) {
	// ⛔ The contract the whole window rests on, and the one a screenshot can
	// never show: a repaint that changes nothing must RE-USE what it drew. The
	// scene repaints on a pointer move, a tooltip clock tick and every frame
	// the rail is still filling — so a viewer that re-rendered its pages each
	// time would re-render a slow page several times a second while the reader
	// did nothing but move the mouse.
	s, _ := opened(t, 3)
	was := drawPage
	t.Cleanup(func() { drawPage = was })
	renders := 0
	drawPage = func(d *reader.Document, i int, o render.Options) (*raster.Image, error) {
		renders++
		return was(d, i, o)
	}
	s.showPages(s.src, s.pvDoc) // everything visible is already drawn
	if renders != 0 {
		t.Errorf("a repaint that changed nothing re-rendered %d page(s)", renders)
	}
}

func TestAPageThatLeavesTheWindowGivesItsPictureBack(t *testing.T) {
	// The window has to let go or it is not a window: a five-hundred-page
	// document read end to end would otherwise hold every page it passed.
	s, _ := opened(t, 12)
	s.pv.CurrentPage().Set(1)
	s.refresh()
	if s.drawn[1] <= 0 {
		t.Fatal("precondition: page 1 should be drawn")
	}
	// Walk far away, redrawing as the workbench does.
	for p := 2; p <= 12; p++ {
		s.pv.CurrentPage().Set(p)
		s.showPages(s.src, s.pvDoc)
	}
	if s.drawn[1] > 0 {
		t.Error("page 1 is still held after the reader walked eleven pages away")
	}
	if s.pv.PageCount() != 12 {
		t.Errorf("letting a picture go changed the page count to %d", s.pv.PageCount())
	}
}

func TestForgettingAPageThatWasNeverDrawnDoesNothing(t *testing.T) {
	s, _ := opened(t, 2)
	s.invalidatePages()
	s.forgetPage(1) // not held
	if len(s.drawn) != 0 {
		t.Errorf("forgetting an undrawn page left %d drawings", len(s.drawn))
	}
}

func TestAPageThatCannotBeDrawnLeavesItsCardBlankWithoutRetryingForever(t *testing.T) {
	// A renderer that fails on a page must not make the scene ask for another
	// repaint to try again: that is a window that never stops redrawing.
	s, _ := opened(t, 1)
	was := drawPage
	t.Cleanup(func() { drawPage = was })
	drawPage = func(*reader.Document, int, render.Options) (*raster.Image, error) {
		return nil, errors.New("this page will not draw")
	}
	s.refresh()
	if s.pageShown() {
		t.Error("a page was shown although the renderer refused it")
	}
	if s.pageMore {
		t.Error("a page that cannot be drawn left the scene asking to be repainted, " +
			"which is a loop rather than a retry")
	}
}

func TestAPageTooNarrowToDrawIsLeftAlone(t *testing.T) {
	// pageSize can report a zero width for a box that is not a box. Dividing
	// the card's width by it would be a division by zero.
	s, _ := opened(t, 1)
	s.drawPageInto(s.src, 99, 100) // must not panic, and must draw nothing
	if s.drawn[99] > 0 {
		t.Error("a page that is not in the document was drawn")
	}
}

// onePointPage builds a document whose page is the given size in points, for
// the two degenerate shapes a real file can carry.
func onePointPage(t *testing.T, w, h int) []byte {
	t.Helper()
	wr := reader.NewWriter("1.7")
	pagesRef := wr.Reserve()
	page := wr.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(w), reader.Integer(h)},
		"Contents": wr.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 0 0 1 1 re f")}),
	})
	wr.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := wr.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := wr.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAPageWhoseWidthIsZeroKeepsItsPlaceAndIsNotDrawn(t *testing.T) {
	// ⛔ A box of the right shape whose width is nil: pageSize reports zero,
	// and the page is still a page. It gets a place in the stack at the paper
	// size — a page missing from the stack would shift every number after it —
	// and no picture, because there is nothing to divide the card's width by.
	s := newState(surfaceW, surfaceH, &fakeHost{name: "flat.pdf", file: onePointPage(t, 0, 100)})
	s.open()
	if s.src == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	if s.pv.PageCount() != 1 {
		t.Errorf("a page with no width was dropped from the stack: %d pages", s.pv.PageCount())
	}
	if s.drawn[1] > 0 {
		t.Error("a page with no width was drawn anyway")
	}
}

func TestAFrameThatRunsOutOfTimeAsksForAnother(t *testing.T) {
	// ⛔ The budget is what keeps a document of slow pages from freezing the
	// window, and the SIGNAL is what keeps the rest of it from never being
	// drawn: what is not finished in one repaint has to leave the scene asking
	// for the next, or the pages below the fold stay blank for ever.
	s, _ := opened(t, 6)
	s.invalidatePages()
	was := timeNow
	t.Cleanup(func() { timeNow = was })
	at := was()
	timeNow = func() time.Time {
		at = at.Add(viewerFrame) // every reading is already past the deadline
		return at
	}
	s.refresh()
	if !s.pageMore {
		t.Fatal("a repaint that ran out of time before drawing anything did not say so")
	}
	// ⛔ Asserted on pageMore and not on s.dirty: refresh sets dirty at the end
	// whatever happened, so a test that read dirty back would pass on a
	// workbench that had lost the signal entirely.
	//
	// And the frame loop must then make progress, or "there is more" is a flag
	// nobody acts on.
	timeNow = was
	s.fillPages()
	if s.drawn[s.at] <= 0 {
		t.Error("the frame after one that ran out of time drew nothing either")
	}
}

func TestFitScaleTakesTheSmallerOfTheTwo(t *testing.T) {
	// ⛔ Still used by the blank-page scan and by the page-as-a-picture verb,
	// and both of them want the whole page. A landscape page is bound by its
	// WIDTH where a portrait one is bound by its height, so a fit tested on one
	// shape only is a fit tested on one branch only.
	tall, _ := opened(t, 1)
	wide := newState(surfaceW, surfaceH, &fakeHost{name: "wide.pdf", file: landscape(t)})
	wide.open()
	if wide.src == nil {
		t.Fatalf("the landscape document did not open: %q", wide.note)
	}
	for _, c := range []struct {
		what string
		s    *state
	}{{"portrait", tall}, {"landscape", wide}} {
		page, err := c.s.src.Page(1)
		if err != nil {
			t.Fatal(err)
		}
		w, h := pageSize(c.s.src, page)
		got := c.s.fitScale(c.s.src)
		if got*w > float64(c.s.pageW()) || got*h > float64(c.s.viewH()) {
			t.Errorf("%s: a %gx%g page at %g spills a %dx%d band",
				c.what, w, h, got, c.s.pageW(), c.s.viewH())
		}
	}
}
