package main

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// tilesIn collects the minipages laid out under a widget, in order.
func tilesIn(w toolkit.Widget) []*toolkit.Thumbnail {
	var out []*toolkit.Thumbnail
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if w == nil {
			return
		}
		if th, ok := w.(*toolkit.Thumbnail); ok {
			out = append(out, th)
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(w)
	return out
}

func tiles(t *testing.T, s *state) []*toolkit.Thumbnail {
	t.Helper()
	s.draw(buffer())
	return tilesIn(s.view)
}

func TestOneSheetHasNoRail(t *testing.T) {
	// A document of one page says where you are by being one page, so a rail
	// of one minipage would be a column of furniture saying nothing.
	s, _ := opened(t, 1)
	if got := tiles(t, s); len(got) != 0 {
		t.Errorf("a one page document was given %d minipages", len(got))
	}
	if s.pageW() != s.viewW() {
		t.Errorf("the page was given %d of %d pixels across, though there is no rail beside it",
			s.pageW(), s.viewW())
	}
}

func TestEverySheetGetsANumberedMinipageAndTheOneYouAreOnIsMarked(t *testing.T) {
	const pages = 5
	s, _ := opened(t, pages)
	got := tiles(t, s)
	if len(got) != pages {
		t.Fatalf("a %d page document was given %d minipages", pages, len(got))
	}
	for i, th := range got {
		if want := strconv.Itoa(i + 1); th.Label != want {
			t.Errorf("minipage %d is captioned %q", i+1, th.Label)
		}
		if on := th.Selected().Get(); on != (i == 0) {
			t.Errorf("minipage %d is marked %v while page %d is being shown", i+1, on, s.at)
		}
	}
	// And the mark follows the page.
	s.step(1)
	got = tiles(t, s)
	if !got[1].Selected().Get() || got[0].Selected().Get() {
		t.Error("turning the page did not move the mark to the minipage of the page now shown")
	}
}

func TestPressingAMinipageGoesToThatSheet(t *testing.T) {
	s, _ := opened(t, 5)
	got := tiles(t, s)
	r := got[3].Bounds()
	if r.W == 0 || r.H == 0 {
		t.Fatal("the minipages have no bounds, so none of them can be pressed")
	}
	press(s, r.X+r.W/2, r.Y+r.H/2)
	if s.at != 4 {
		t.Errorf("pressing the fourth minipage left page %d on the screen", s.at)
	}
}

func TestTheRailLeavesThePageLessRoomAndTheToolsLessStill(t *testing.T) {
	s, _ := opened(t, 5)
	s.draw(buffer())
	withRail := s.pageW()
	if withRail >= s.viewW() {
		t.Errorf("the page was given %d pixels of the %d across though a rail is beside it",
			withRail, s.viewW())
	}
	openGroup(t, s, "Pages")
	if s.pageW() >= withRail {
		t.Errorf("opening a panel did not narrow the page further: %d, was %d", s.pageW(), withRail)
	}
	// What matters is that the sum is honest: the three bands and the gaps
	// between them are the whole width, and nothing is drawn over anything.
	if got, want := s.pageW()+railW+panelW+2*gap, s.viewW(); got != want {
		t.Errorf("rail, page and panel come to %d across, in a band %d wide", got, want)
	}
}

func TestTheMinipagesAreDrawnOnceAndForgottenWhenTheDocumentChanges(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	first := s.thumbs.pix[1]
	if first == nil {
		t.Fatal("no minipage was drawn for the page on the screen")
	}
	// A repaint that changes nothing must not draw them again: the pictures
	// are the same bytes, not merely equal ones.
	s.refresh()
	s.draw(buffer())
	if got := s.thumbs.pix[1]; &got[0] != &first[0] {
		t.Error("a repaint of an unchanged document drew its minipages again")
	}
	// Turning a page is a change, and what was drawn before it is of a
	// document that no longer exists.
	s.rotate()
	s.draw(buffer())
	if got := s.thumbs.pix[1]; got != nil && len(got) > 0 && &got[0] == &first[0] {
		t.Error("the minipage of a page that has been turned is the picture from before it was")
	}
}

func TestTheMinipagesOfALongDocumentAreBoundedButItsNumbersAreNot(t *testing.T) {
	// ⛔ The rail must not hold a bitmap per page. A tile with no picture
	// still draws its frame and its number, which is most of what the rail is
	// for, so far from the page you are on numbers do the work.
	const pages = thumbWindow*2 + 9
	s, _ := opened(t, pages)
	s.at = 1
	s.refresh()
	got := tiles(t, s)
	if len(got) != pages {
		t.Fatalf("a %d page document was given %d minipages", pages, len(got))
	}
	if n := len(s.thumbs.pix); n > thumbWindow+1 {
		t.Errorf("%d pictures were kept for a document of %d pages, with a window of %d either side",
			n, pages, thumbWindow)
	}
	// The last page is beyond the window, so it is a numbered frame.
	if last := got[pages-1]; last.Pixels != nil {
		t.Error("a page far outside the window was drawn anyway")
	} else if last.Label != strconv.Itoa(pages) {
		t.Errorf("the minipage beyond the window is captioned %q, so it says nothing at all", last.Label)
	}
}

func TestTheMinipagesTheBudgetStopsAreDrawnOnTheNextFrame(t *testing.T) {
	// ⛔ The clock ADVANCES rather than standing still. Freezing it is the
	// obvious thing and it is backwards: a deadline computed from a stopped
	// clock never arrives, so the budget would never stop anything and the
	// test would pass by drawing everything on the first frame. Each reading
	// here moves on by more than half the frame, so the deadline is set, one
	// minipage is drawn, and the next reading is past it.
	at := time.Now()
	timeNow = func() time.Time {
		at = at.Add(thumbFrame/2 + time.Millisecond)
		return at
	}
	t.Cleanup(func() { timeNow = time.Now })

	s, _ := opened(t, 6)
	s.draw(buffer())
	if !s.railMore {
		t.Fatal("the budget did not stop anything, so there is nothing to continue")
	}
	if n := len(s.thumbs.pix); n != 1 {
		t.Fatalf("%d minipages were drawn on a frame that had time for one", n)
	}
	// Each frame draws another, and the scene asks for the next by going
	// dirty -- until there is nothing left, when it stops asking.
	for i := 0; i < 20 && s.railMore; i++ {
		s.takeDirty()
		if !s.tick() {
			t.Fatalf("the rail still has %d minipages to draw and asked for no repaint", 6-len(s.thumbs.pix))
		}
	}
	if s.railMore {
		t.Fatalf("after twenty frames %d minipages are drawn of 6", len(s.thumbs.pix))
	}
	if n := len(s.thumbs.pix); n != 6 {
		t.Errorf("%d minipages were drawn for a document of 6 pages", n)
	}
	s.takeDirty()
	if s.tick() {
		t.Error("the rail asked for another repaint with everything drawn")
	}
}

func TestTheNearestSheetsAreDrawnFirst(t *testing.T) {
	// Nearest first is the order somebody needs them in: a rail that filled
	// from page one would draw twenty pages you cannot see before the one
	// under your eyes.
	for _, c := range []struct {
		at, n, reach int
		want         []int
	}{
		{at: 1, n: 3, reach: 5, want: []int{1, 2, 3}},
		{at: 3, n: 5, reach: 1, want: []int{3, 2, 4}},
		{at: 5, n: 5, reach: 2, want: []int{5, 4, 3}},
		{at: 2, n: 4, reach: 2, want: []int{2, 1, 3, 4}},
		{at: 1, n: 1, reach: 3, want: []int{1}},
		{at: 9, n: 3, reach: 1, want: nil}, // a page off the end asks for nothing
	} {
		if got := aroundWithin(c.at, c.n, c.reach); !reflect.DeepEqual(got, c.want) && len(got)+len(c.want) > 0 {
			t.Errorf("around page %d of %d within %d: %v, want %v", c.at, c.n, c.reach, got, c.want)
		}
	}
}

func TestThePicturesYouHaveMovedAwayFromAreLetGo(t *testing.T) {
	// The window follows the page you are on, so walking through a long
	// document must not accumulate its pages: the rail's cost is the window,
	// not the document.
	const pages = thumbWindow*3 + 2
	s, _ := opened(t, pages)
	s.goTo(1)
	s.draw(buffer())
	if _, have := s.thumbs.pix[1]; !have {
		t.Fatal("the page being shown has no picture")
	}
	s.goTo(pages)
	s.draw(buffer())
	if _, have := s.thumbs.pix[1]; have {
		t.Errorf("the picture of page 1 was kept while page %d is being shown, %d pages away",
			pages, pages-1)
	}
	if n := len(s.thumbs.pix); n > thumbWindow+1 {
		t.Errorf("%d pictures are held after walking a %d page document", n, pages)
	}
}

func TestAMinipageCannotTakeYouOffTheDocument(t *testing.T) {
	s, _ := opened(t, 3)
	s.goTo(2)
	for _, to := range []int{0, -1, 4, 99} {
		s.goTo(to)
		if s.at != 2 {
			t.Fatalf("going to page %d of a three page document left page %d on the screen", to, s.at)
		}
	}
	// Going where you already are is not a change, and a refresh would redraw
	// the document for nothing.
	s.takeDirty()
	s.goTo(2)
	if s.takeDirty() {
		t.Error("pressing the minipage of the page already shown redrew the document")
	}
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	empty.goTo(1) // with no document at all there is nowhere to go
	if empty.at != 1 {
		t.Errorf("a workbench with no document went to page %d", empty.at)
	}
}

func TestASheetThatCannotBeDrawnLeavesANumberedFrame(t *testing.T) {
	// ⛔ A rail is an aid to finding your place. A page the renderer cannot
	// draw must cost its number and nothing else -- not a message, which would
	// push aside what the page itself has to say, and not a gap, which would
	// make the numbers lie about where you are.
	was := drawPage
	drawPage = func(*reader.Document, int, render.Options) (*raster.Image, error) {
		return nil, errors.New("no")
	}
	t.Cleanup(func() { drawPage = was })

	s, _ := opened(t, 4)
	s.refresh()
	got := tiles(t, s)
	if len(got) != 4 {
		t.Fatalf("a four page document that cannot be drawn was given %d minipages", len(got))
	}
	for i, th := range got {
		if th.Pixels != nil {
			t.Errorf("minipage %d has a picture though no page could be drawn", i+1)
		}
		if want := strconv.Itoa(i + 1); th.Label != want {
			t.Errorf("minipage %d is captioned %q", i+1, th.Label)
		}
	}
	if s.note != "" && strings.Contains(s.note, "minipage") {
		t.Errorf("the status line was given over to the rail: %q", s.note)
	}
}

func TestAFaceThatCannotBeReadLeavesTheOneAlreadyThere(t *testing.T) {
	// A workbench that opens with plain text beats one that does not open.
	was, wasFace := toolkit.CurrentFont(), uiFace
	t.Cleanup(func() { uiFace = wasFace; toolkit.SetFont(was) })
	uiFace = []byte("this is not a typeface")
	useVectorText()
	if toolkit.CurrentFont() != was {
		t.Error("a face that cannot be read replaced the one that works")
	}
}

// shapedPDF is a document whose pages are the shapes asked for, in points.
func shapedPDF(t *testing.T, boxes ...[2]int) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	kids := make(reader.Array, 0, len(boxes))
	for _, b := range boxes {
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 1 1 2 2 re f")})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
				reader.Integer(b[0]), reader.Integer(b[1])},
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count": reader.Integer(len(boxes))})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAMinipageIsTheShapeOfItsSheet(t *testing.T) {
	// A report with a landscape table in the middle of it is exactly the
	// document somebody needs a rail to find their way around, and a tile the
	// shape of its page says "that one" before any picture in it is legible.
	h := &fakeHost{name: "shapes.pdf", file: shapedPDF(t,
		[2]int{400, 300},  // landscape
		[2]int{300, 400},  // portrait
		[2]int{100, 9000}, // a cutting plan, which the cap is for
		[2]int{0, 0},      // and a page that says nothing about its size
	)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	got := tiles(t, s)
	if len(got) != 4 {
		t.Fatalf("a four page document was given %d minipages", len(got))
	}
	strip := textH() + 2*toolkit.ThumbnailLabelPad
	land, port := got[0].Bounds().H-strip, got[1].Bounds().H-strip
	if land >= port {
		t.Errorf("the landscape page's minipage is %d tall and the portrait one's %d", land, port)
	}
	if want := thumbPicW * 300 / 400; land != want {
		t.Errorf("a 400x300 page at %d across gave a picture %d tall, not %d", thumbPicW, land, want)
	}
	if tall := got[2].Bounds().H - strip; tall != thumbTallest {
		t.Errorf("a page 90 times taller than it is wide gave a tile %d tall, and the cap is %d",
			tall, thumbTallest)
	}
	// A page whose box says nothing is a square rather than nothing at all:
	// a tile of no height would make the numbers lie about where you are.
	if none := got[3].Bounds().H - strip; none != thumbPicW {
		t.Errorf("a page with no usable size gave a tile %d tall, and a square would be %d",
			none, thumbPicW)
	}
}

func TestAMinipageIsDrawnSmallRatherThanShrunkAfterwards(t *testing.T) {
	// ⛔ The rail asks the renderer for a SMALL picture. Drawing each page at
	// its natural size and letting the tile shrink it looks identical and is
	// the thing that makes the rail unaffordable: the pictures are what the
	// window of pages is sized against, and a full-size one is some forty
	// times the bytes of the tile it ends up in. Nothing else in this file
	// notices the difference, because every other property of the rail holds
	// either way -- which is exactly why it is asserted here.
	s, _ := opened(t, 3)
	s.draw(buffer())
	for i := 1; i <= 3; i++ {
		w, h := s.thumbs.size[i][0], s.thumbs.size[i][1]
		if w == 0 {
			t.Fatalf("page %d has no picture", i)
		}
		// Within a pixel of the rail's width: the scale is derived from the
		// page's own size in points, so rounding is the only slack there is.
		if w < thumbPicW-1 || w > thumbPicW+1 {
			t.Errorf("the picture of page %d is %dx%d, and the rail is %d across",
				i, w, h, thumbPicW)
		}
		if got, want := w*h*4, (thumbPicW+2)*(thumbTallest+2)*4; got > want {
			t.Errorf("the picture of page %d is %d bytes, and the largest tile is %d", i, got, want)
		}
	}
}
