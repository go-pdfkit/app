// Putting the pages in another order by looking at them.
//
// The rail down the left has shown every page as a minipage since the rail
// existed, and `Move it there` has moved one page since the Pages tab existed —
// by its NUMBER, typed into a box, which is the one thing nobody has in mind
// when they want a page somewhere else. What they have in mind is the picture
// of it.
//
// ⛔ TWO PRESSES, NOT A DRAG. A drag over a grid inside a scroll view has to
// hit-test tiles against a moving viewport and decide what a press that moved
// four pixels meant; two presses say the same thing — this page, then where it
// goes — and a reader can see what is held between them, because the tile says
// so and the status line says so. It is also the gesture a touch screen gives
// for nothing.

package main

import (
	"fmt"
	"strconv"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// readingArrange is the reading that shows the pages as a grid.
const readingArrange = "arrange"

// arrangePages puts the grid where the page was, or the page back when it is
// already showing — and lets go of whatever was held, because a page picked up
// in a view that is no longer on the screen is a move nobody can finish.
//
// ⛔ `arrangePages` and not `arrange`: the band's own layout function is called
// arrange, and this is the verb. Two methods of one type cannot share a name,
// which is the compiler saying what a reader would have had to work out.
//
// ⛔ The letting go happens in `read`, not here, because the grid can be left by
// any of the other readings' verbs as well as by this one — and a page held in a
// view nobody is looking at is a move the next press would finish by surprise.
func (s *state) arrangePages() { s.read(readingArrange) }

// arrangeView is every page of the document as a tile.
//
// ⛔ THE PICTURES ARE THE RAIL'S OWN, at the rail's width and already drawn.
// Rendering a second set at another size for this view would double what a
// forty-page document costs to look at, and the tiles further out are a frame
// and a number either way — which, for putting pages in order, is most of what
// a tile is for.
func (s *state) arrangeView(src *reader.Document) toolkit.Widget {
	n := src.PageCount()
	wide := s.pageW() - 2*margin
	// ⛔ AT LEAST ONE, and max rather than an if: a band narrower than a tile
	// would divide to nought, and the loop below steps by this — so the guard is
	// not tidiness, it is the difference between a narrow window and a hang. As
	// an `if` it was also a branch no test could reach, since nothing can make
	// the band that narrow; as arithmetic it is simply true.
	across := max(1, (wide+thumbGap)/(s.picW()+thumbGap))
	col := newColumn()
	col.add(toolkit.NewLabel(s.arrangeSays(n)), bareH())
	// ⛔ THE TILES ARE REMEMBERED WITH THEIR PAGE NUMBERS, because a press is
	// answered long after this builds them: the page menu asks "which page is
	// under this point", and reading the number back off a tile's LABEL would be
	// a second spelling of the same fact, two files apart. A widget knows its
	// own rectangle once it has been laid out; what it does not know is which
	// page it stands for, so that is what is kept beside it.
	s.grid = s.grid[:0]
	for i := 1; i <= n; i += across {
		row := toolkit.NewHBox()
		row.Spacing = thumbGap
		tall := 0
		for j := i; j < i+across && j <= n; j++ {
			th := s.arrangeTile(j, n)
			s.grid = append(s.grid, gridTile{page: j, at: th})
			row.AddFixed(th, s.picW())
			if h := s.tileH(src, j); h > tall {
				tall = h
			}
		}
		col.add(row, tall)
	}
	return col.scrollerOf(wide)
}

// A gridTile is one tile of the arranging grid and the page it stands for.
type gridTile struct {
	page int
	at   toolkit.Widget
}

// gridUnder is the page whose tile is under the point, if the grid is the view
// and the point is on one of its tiles.
//
// ⛔ It is what lets the PAGE MENU work in the grid, and the menu is the whole
// of BentoPDF's multi-tool that the grid did not already answer: turn this one,
// drop that one, without leaving the view that shows which is which. The menu
// goes to the page it names first, so every verb on it — and every verb on the
// ribbon after it — acts on the tile that was pressed.
func (s *state) gridUnder(x, y int) (int, bool) {
	if s.tools.reading.Get() != readingArrange {
		return 0, false
	}
	for _, t := range s.grid {
		if t.at.Bounds().Contains(x, y) {
			return t.page, true
		}
	}
	return 0, false
}

// arrangeSays is the line above the grid: what to do, or what is held and what
// the next press will do with it.
func (s *state) arrangeSays(n int) string {
	if s.held == 0 {
		return fmt.Sprintf("%d pages. Press one to pick it up, then press where it goes.", n)
	}
	return fmt.Sprintf("Page %d is held. Press the page it should go in front of, "+
		"or press it again to put it back.", s.held)
}

// arrangeTile is one page in the grid: its picture, its number, and a press
// that picks it up or drops what is held in front of it.
func (s *state) arrangeTile(i, n int) toolkit.Widget {
	th := toolkit.NewThumbnail(s.thumbs.pix[i], s.thumbs.size[i][0], s.thumbs.size[i][1])
	th.Label = strconv.Itoa(i)
	th.Alt = "page " + strconv.Itoa(i) + " of " + strconv.Itoa(n)
	th.Area = true
	// ⛔ What is LIT is the page being held, not the page being read. In the
	// rail the mark means "you are here"; here it means "this is the one you
	// picked up", because that is the question this view answers and a mark
	// that meant both would answer neither.
	th.Selected().Set(i == s.held)
	th.OnClick = func() { s.pressArranged(i) }
	return th
}

// pressArranged is what a press on a tile of the grid does.
func (s *state) pressArranged(i int) {
	switch {
	case s.held == 0:
		s.held = i
		// ⛔ AND IT IS THE PAGE BEING READ NOW, which is what makes the grid a
		// place to work rather than a place to look. Rotate, Delete and every
		// other verb that acts on "the page" acts on s.at, so without this a
		// reader who could SEE the upside-down page in the grid still had to
		// find it again in the rail to turn it.
		s.at = i
		s.note = fmt.Sprintf("page %d picked up — press the page it should go in front of, "+
			"or use a verb: it is the page being read now", i)
		s.refresh()
	case s.held == i:
		s.held = 0
		s.note = fmt.Sprintf("page %d put back", i)
		s.refresh()
	default:
		s.movePageTo(s.held, i)
	}
}

// movePageTo takes one page out and puts it back in front of another.
//
// ⛔ IN FRONT OF THE PAGE PRESSED, and that is not the same as "to its number".
// ops.Move takes the page out first, so moving page 2 to position 5 in a
// six-page document leaves it AFTER the page that was fifth — a reader who
// pressed the fifth tile meant "before this one". The number to ask for is one
// less when the page is moving forwards, and the status line says where it
// landed rather than where it was asked to go.
func (s *state) movePageTo(from, before int) {
	to := before
	if from < before {
		to = before - 1
	}
	if to == from {
		s.held = 0
		s.note = fmt.Sprintf("page %d is already in front of page %d", from, before)
		s.refresh()
		return
	}
	// ⛔ The refusal is not branched on, and that is the whole of what this has
	// to say about it: ops.Move refuses a page outside 1..n, and both numbers
	// here came from tiles of a grid built from the pages the document has.
	// A branch would be an arm no test could reach.
	s.changeSaying(fmt.Sprintf("moved page %d in front of page %d", from, before),
		func(d *ops.Doc) error { return d.Move(from, to) })
	// ⛔ THE VIEW FOLLOWS THE PAGE IT MOVED, as `Move it there` does: a reader
	// who moved the page they were reading is still reading it.
	s.held = 0
	s.at = to
	s.note = fmt.Sprintf("moved page %d in front of page %d, where it is now page %d",
		from, before, to)
	s.refresh()
}
