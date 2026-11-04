// The page band: a real viewer rather than one picture.
//
// ⛔ THE VIEWER OUTLIVES THE DOCUMENT IT IS SHOWING, and that is the whole
// shape of this file. Everything else in the workbench is rebuilt by refresh --
// the status line, the rail, the panel -- because everything else is a function
// of the document as it now stands. A viewer is not: its zoom, its layout mode
// and where it is scrolled to are the READER's, and rebuilding it on every edit
// would hand back a page at 100% and at the top after each crop, which is the
// moment somebody most wants to still be looking at what they were looking at.
//
// So s.pv is made once, in newState, and only its CONTENTS are replaced.

package main

import (
	"errors"
	"image"
	"time"

	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// pageWindow is how many pages either side of the one being read are drawn.
//
// ⛔ The same bound the minipage rail keeps, and for the same reason: a tile is
// small and a page is not. At the zoom a page is usually read at, one of them
// is a few megabytes, so a five-hundred-page document held whole would be more
// memory than the tab has — in a tab whose whole point is that nothing leaves
// it. PagedView lays out every page from its declared size, so the scrollbar
// and the page number stay honest about a document whose pictures are mostly
// not held.
const pageWindow = 2

// viewerFrame is how long all the pages together may take in one repaint.
// What is not drawn by then is left for the next, which the scene asks for by
// going dirty — the same arrangement the rail uses, so a document whose pages
// are slow fills in rather than freezing the window.
const viewerFrame = 400 * time.Millisecond

// fallbackW and fallbackH are the paper a page whose box cannot be read is
// given a place at. The same US Letter pageSize itself falls back on, so a
// page measured and a page guessed at agree.
const fallbackW, fallbackH = 612.0, 792.0

// newViewer builds the page band's viewer. It starts paginated and fitted to
// the page, which is what the workbench did before there was any choice: one
// page, as large as it will go without spilling. The difference is that it is
// now a choice, and the toolbar says so.
func newViewer() *toolkit.PagedView {
	pv := toolkit.NewPagedView(nil)
	pv.Mode().Set(toolkit.PagedPaginated)
	pv.SetFit(toolkit.PagedFitPage)
	return pv
}

// fillPages draws more of what the reader can see, a frame at a time, for a
// document whose pages take longer than one frame allows.
//
// ⛔ It draws from the document ALREADY PARSED rather than going through
// refresh: a refresh writes the whole document out and reads it back, and
// doing that once a frame to finish one page would cost more than the page.
// The rail's fillRail is the same shape for the same reason.
func (s *state) fillPages() {
	if !s.pageMore || s.src == nil || s.pv.PageCount() == 0 {
		return
	}
	s.pageMore = s.showPages(s.src, s.pvDoc)
	s.dirty = true
}

// pageShown says a drawn page is what the view band is holding: the viewer is
// in the band (rather than a reading, a form or a sentence) AND the page being
// read has a picture in it.
//
// ⛔ Both halves. The viewer is never nil now — it is built once and kept — so
// "is there a viewer" answers nothing, and a viewer full of blank cards is
// what a document whose pages will not draw looks like.
func (s *state) pageShown() bool {
	pv, ok := s.shown.(*toolkit.PagedView)
	return ok && pv == s.pv && s.drawn[s.at] > 0
}

// followViewer takes the page the reader has scrolled, keyed or paged to.
//
// ⛔ It does NOT refresh. refresh rebuilds the status line, re-reads the
// document and redraws the rail, and it is called by every verb — calling it
// from here would make a wheel notch as expensive as a crop, and would re-enter
// the viewer while the viewer is announcing a change. Marking the scene dirty
// is enough: the next frame repaints with the new page lit in the rail.
func (s *state) followViewer(page int) {
	if page == s.at || page < 1 {
		return
	}
	s.at = page
	s.railView, s.railMore = nil, false // the lit tile moved
	s.reconsider()                      // "previous" greys on page 1, "next" on the last
	s.dirty = true
}

// showPages puts the document in the viewer and draws what can be seen of it.
// It returns whether anything is still waiting to be drawn, so the scene can
// ask for another repaint — the same contract rail has.
func (s *state) showPages(src *reader.Document, stamp [2]uint64) bool {
	s.declarePages(src, stamp)
	// ⛔ The viewer's idea of the current page and the workbench's are the same
	// datum, and this is the direction that is easy to forget: a verb that
	// deletes page 7 moves the workbench to 6, and the viewer has to follow or
	// the next scroll jumps back. The other direction is a Subscribe, set up
	// once in newState.
	if s.pv.CurrentPage().Get() != s.at {
		s.pv.ScrollToPage(s.at, 0)
	}
	return s.drawVisible(src)
}

// declarePages tells the viewer the shape of every page in the document, which
// is what lets it lay out five hundred of them while holding a handful.
//
// ⛔ Only when the document has CHANGED. Re-declaring on every repaint would
// clear the bitmaps and re-render the visible pages from scratch each frame,
// which is the same defect as rebuilding the viewer, one layer down.
func (s *state) declarePages(src *reader.Document, stamp [2]uint64) {
	if s.pvDoc == stamp && s.pv.PageCount() == src.PageCount() {
		return
	}
	s.pvDoc = stamp
	n := src.PageCount()
	sizes := make([]image.Point, n)
	for i := 1; i <= n; i++ {
		// A page that cannot be measured still gets a place, at the size
		// pageSize falls back on, because a page missing from the stack would
		// shift every number after it.
		w, h := 0.0, 0.0
		if page, err := src.Page(i); err == nil {
			w, h = pageSize(src, page)
		}
		if w <= 0 || h <= 0 {
			w, h = fallbackW, fallbackH
		}
		// At least one pixel each way: a card with no extent is a page with no
		// place, and every page after it would shift up by one.
		sizes[i-1] = image.Point{X: max(1, int(w+0.5)), Y: max(1, int(h+0.5))}
	}
	s.pv.SetPageSizes(sizes)
	s.drawn = map[int]int{}
	// No relayout is forced here, although the viewer was placed in the band
	// before it was told these sizes. PagedView.PageRect relays out at the top
	// of itself (pagedview.go:931), so the first question drawVisible asks it
	// is already answered against the new set.
	//
	// ⛔ A SetBounds nudge stood here for a while, with a confident comment
	// about stale geometry. Removing it broke no test, which is how the comment
	// was found to be a story: the defect it claimed to fix was the drawings
	// cache, and invalidatePages is what fixes that.
}

// invalidatePages drops every drawing, so the next repaint makes them again.
//
// ⛔ Called by refresh, which is what every verb ends in. The window of
// drawings exists for SCROLLING and ZOOMING, where the document is not
// changing and re-rendering a page the reader is still looking at would be
// pure waste. A verb is the opposite case: the document under those drawings
// is gone, and keeping them would show the page as it was before the crop.
func (s *state) invalidatePages() {
	for i := range s.drawn {
		s.pv.SetPageBitmap(i, nil)
	}
	s.drawn = map[int]int{}
}

// drawVisible renders the pages the reader can actually see, at the size they
// are being shown at.
//
// ⛔ AT THE SIZE THEY ARE SHOWN AT, which is the difference between a viewer
// and a picture. PagedView scales a bitmap to the card it declared, so a page
// rendered once at one scale and then zoomed to 400% is that first rendering
// enlarged -- soft, and softest exactly when somebody has zoomed in to look
// closely. Rendering to the card's own width makes the blit 1:1.
func (s *state) drawVisible(src *reader.Document) bool {
	n := src.PageCount()
	deadline := timeNow().Add(viewerFrame)
	more := false
	for i := 1; i <= n; i++ {
		rect, clip, ok := s.pv.PageRect(i)
		// An empty clip is a page that exists and is not on screen; a card no
		// pixels wide is one the zoom has scaled away, which there is nothing
		// to render into. Outside the window either also loses whatever it was
		// holding: the window has to let go or it is not a window.
		if !ok || clip.W <= 0 || clip.H <= 0 || rect.W <= 0 {
			if i < s.at-pageWindow || i > s.at+pageWindow {
				s.forgetPage(i)
			}
			continue
		}
		if s.drawn[i] == rect.W {
			continue // already drawn at this size
		}
		if !timeNow().Before(deadline) {
			more = true // out of time; the next repaint picks this up
			continue
		}
		// ⛔ Not consulted for "is there more". A page the renderer refused is
		// FINISHED, not pending: asking for another repaint to try it again is
		// a loop, not a retry. Only the frame budget leaves work behind.
		s.drawPageInto(src, i, rect.W)
	}
	return more
}

// forgetPage releases a page's picture and the note of what size it was drawn
// at, together — a size remembered for a bitmap that is gone would stop the
// page ever being drawn again.
func (s *state) forgetPage(i int) {
	if _, held := s.drawn[i]; !held {
		return
	}
	delete(s.drawn, i)
	s.pv.SetPageBitmap(i, nil)
}

// drawPageInto renders one page so that it is w device pixels wide and gives
// it to the viewer. A page it cannot draw keeps its blank card: the stack is
// laid out from the declared sizes, so a page with no picture is still a page
// in the right place with the right number.
func (s *state) drawPageInto(src *reader.Document, i, w int) {
	page, err := src.Page(i)
	if err != nil {
		return
	}
	pw, _ := pageSize(src, page)
	if pw <= 0 {
		return
	}
	img, err := drawPage(src, i, render.Options{
		Scale:       float64(w) / pw,
		MaxDuration: pageBudget,
	})
	// A page that ran out of time comes back as far as it got, which is worth
	// showing: somebody scrolling would rather see most of a figure than a
	// sentence saying there was one.
	partial := errors.Is(err, render.ErrTimedOut) && img != nil
	if err != nil && !partial {
		return // its card stays blank
	}
	s.pv.SetPageBitmap(i, &image.RGBA{
		Pix:    img.Pix,
		Stride: img.W * 4,
		Rect:   image.Rect(0, 0, img.W, img.H),
	})
	s.drawn[i] = w
	if partial {
		s.note = "a page was still being drawn when its time ran out; this is as far as it got"
	}
}
