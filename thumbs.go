// The rail of minipages down the side of the page.

package main

import (
	"hash/crc32"
	"strconv"
	"time"

	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

const (
	// railW is the whole rail and thumbPicW the picture inside it: the
	// scrollbar down its edge and the frame around each tile take the rest.
	railW     = 132
	thumbPicW = 94
	// thumbTallest caps a tile, because a tile's height follows its page's
	// shape and a page may be any shape at all: a cutting plan 50 times taller
	// than it is wide would otherwise make one tile the length of the rail.
	thumbTallest = thumbPicW * 5 / 2

	// thumbWindow is how many pages either side of the one being shown are
	// drawn and kept.
	//
	// ⛔ Not unbounded. A tile is roughly 94x125x4 bytes, so a 500-page
	// document would be 24MB of bitmaps in a tab whose whole point is that
	// nothing leaves it. A tile with no picture still draws its frame and its
	// number, which is what the rail is for -- so far from the page you are
	// on, numbered frames say where you are just as well as pictures would.
	thumbWindow = 20

	// thumbFrame is how long all the minipages together may take in one
	// repaint, and thumbPage bounds any one of them so a single pathological
	// page cannot eat the frame on its own.
	//
	// What is not drawn by then is left for the next repaint, which the scene
	// asks for by going dirty. A document whose pages are slow therefore fills
	// its rail in rather than holding up the page it belongs to.
	thumbFrame = 200 * time.Millisecond
	thumbPage  = 150 * time.Millisecond
)

// thumbs is what has been drawn, and what it was drawn from.
type thumbs struct {
	// stamp identifies the document the pictures came from: the length of its
	// bytes and a CRC over them.
	//
	// The workbench writes the document out and reads it back on every
	// repaint, so what is drawn is always what Save would give -- which means
	// the bytes are already in hand here and a digest of them is the one key
	// that cannot go stale. A counter bumped by each verb would be cheaper and
	// would be wrong the first time somebody added a verb and forgot it.
	//
	// CRC-32 rather than a cryptographic hash because the question is "are
	// these the same bytes as a moment ago", not "can someone forge them": the
	// cost of the roughly one-in-four-billion miss is one stale minipage.
	stamp [2]uint64
	pix   map[int][]byte
	size  map[int][2]int
}

// stampOf identifies a document by its bytes.
func stampOf(raw []byte) [2]uint64 {
	return [2]uint64{uint64(len(raw)), uint64(crc32.Checksum(raw, crc32.MakeTable(crc32.Castagnoli)))}
}

// forget starts again when the document is not the one the pictures came from.
func (t *thumbs) forget(stamp [2]uint64) {
	if t.stamp == stamp && t.pix != nil {
		return
	}
	t.stamp = stamp
	t.pix = map[int][]byte{}
	t.size = map[int][2]int{}
}

// drop releases the pictures outside the window around the page being shown,
// so the rail's cost follows the window rather than the document.
func (t *thumbs) drop(at int) {
	for n := range t.pix {
		if n < at-thumbWindow || n > at+thumbWindow {
			delete(t.pix, n)
			delete(t.size, n)
		}
	}
}

// rail is the column of minipages beside the page, or nil when there is no
// reason for one: a document of a single sheet says where you are by being a
// single sheet.
//
// It returns whether anything is still waiting to be drawn, so the scene can
// ask for another repaint and let the rail fill in.
func (s *state) rail(src *reader.Document, stamp [2]uint64) (toolkit.Widget, bool) {
	n := src.PageCount()
	if n < 2 {
		return nil, false
	}
	if s.thumbs == nil {
		s.thumbs = &thumbs{}
	}
	t := s.thumbs
	t.forget(stamp)
	t.drop(s.at)

	more := s.drawThumbs(src, n)

	col := toolkit.NewVBox()
	col.Spacing = thumbGap
	h := (n - 1) * thumbGap
	for i := 1; i <= n; i++ {
		tileH := s.tileH(src, i)
		col.AddFixed(s.tile(i, n), tileH)
		h += tileH
	}
	col.SetBounds(toolkit.Rect{W: railW - scrollbarW, H: h})
	sv := toolkit.NewScrollView(col)
	sv.SetContentSize(railW-scrollbarW, h)
	return sv, more
}

// tileH is how tall the ith tile is: its page's own shape at the rail's width,
// plus the strip the number is written in.
//
// Each page measured rather than one shape assumed for the document, because a
// report with a landscape table in the middle of it is exactly the document
// somebody needs a rail to find their way around, and a tile the shape of its
// page says "that one" before any picture in it is legible.
func (s *state) tileH(src *reader.Document, i int) int {
	pic := thumbPicW // a page that cannot be measured gets a square
	if page, err := src.Page(i); err == nil {
		if w, h := pageSize(src, page); w > 0 && h > 0 {
			pic = int(float64(thumbPicW) * h / w)
		}
	}
	if pic > thumbTallest {
		pic = thumbTallest
	}
	return pic + textH() + 2*toolkit.ThumbnailLabelPad
}

// thumbGap is the space between two tiles, and scrollbarW what the bar down
// the rail's edge takes from the tiles' width.
const (
	thumbGap   = 6
	scrollbarW = 14
)

// tile is one minipage: its picture if there is one, its number either way,
// and a press that goes there.
func (s *state) tile(i, n int) toolkit.Widget {
	th := toolkit.NewThumbnail(s.thumbs.pix[i], s.thumbs.size[i][0], s.thumbs.size[i][1])
	th.Label = strconv.Itoa(i)
	th.Alt = "page " + strconv.Itoa(i) + " of " + strconv.Itoa(n)
	// Area rather than nearest: a page shrunk to a hundred pixels by taking
	// one sample in eight loses the lines of a table and keeps a moiré of
	// them, which is worse than no picture. These are static previews, which
	// is the case the toolkit caches the averaged copy for.
	th.Area = true
	th.Selected().Set(i == s.at)
	th.OnClick = func() { s.goTo(i) }
	return th
}

// goTo shows another page. Unlike step it is given the page rather than a
// direction, because a minipage is a place rather than a move.
func (s *state) goTo(to int) {
	if s.doc == nil || to < 1 || to > s.doc.PageCount() || to == s.at {
		return
	}
	s.at = to
	s.refresh()
}

// drawThumbs draws what the window asks for and the budget allows, nearest to
// the page being shown first, and says whether any are still missing.
//
// Nearest first because that is the order somebody needs them in: the tiles
// around the one you are on are the ones you are looking at, and a rail that
// filled from page one would draw twenty pages you cannot see before the one
// under your eyes.
func (s *state) drawThumbs(src *reader.Document, n int) bool {
	t := s.thumbs
	until := timeNow().Add(thumbFrame)
	more := false
	for _, i := range aroundWithin(s.at, n, thumbWindow) {
		if _, have := t.pix[i]; have {
			continue
		}
		if !timeNow().Before(until) {
			more = true
			continue
		}
		page, err := src.Page(i)
		if err != nil {
			continue
		}
		w, _ := pageSize(src, page)
		if w <= 0 {
			continue
		}
		img, err := drawPage(src, i, render.Options{
			Scale:       float64(thumbPicW) / w,
			MaxDuration: thumbPage,
		})
		// A minipage that ran out of time is kept as far as it got, and one
		// that cannot be drawn at all leaves a numbered frame. Neither is
		// worth a word in the status line: the rail is an aid to finding your
		// place, and a message about it would push aside what the page itself
		// has to say.
		if img == nil {
			continue
		}
		t.pix[i], t.size[i] = img.Pix, [2]int{img.W, img.H}
	}
	return more
}

// timeNow is a variable so a test can hold the clock still and watch the
// budget do its work without waiting for it.
var timeNow = time.Now

// aroundWithin is the pages within reach of at, nearest first: at, at-1, at+1,
// at-2 and so on, dropping whatever falls outside 1..n.
func aroundWithin(at, n, reach int) []int {
	out := make([]int, 0, 2*reach+1)
	if at >= 1 && at <= n {
		out = append(out, at)
	}
	for d := 1; d <= reach; d++ {
		for _, i := range [2]int{at - d, at + d} {
			if i >= 1 && i <= n {
				out = append(out, i)
			}
		}
	}
	return out
}

// fillRail draws another frame's worth of minipages and puts the rail back
// together, which is how a long document fills in instead of holding up the
// page it belongs to.
//
// It draws from the document ALREADY PARSED rather than calling renderPage
// again: a repaint writes the whole document out and reads it back, and doing
// that once per frame to add two minipages would cost more than the minipages.
func (s *state) fillRail() {
	if !s.railMore || s.shown == nil || s.src == nil || s.thumbs == nil {
		return
	}
	rail, more := s.rail(s.src, s.thumbs.stamp)
	s.railView, s.railMore = rail, more
	s.show(s.shown)
	s.dirty = true
}

// tick is what the harness calls once an animation frame: it lets the rail
// catch up and then says whether anything needs painting.
func (s *state) tick() bool {
	s.fillRail()
	return s.takeDirty()
}
