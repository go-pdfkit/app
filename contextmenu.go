// The secondary press: a menu of the verbs that act on ONE page, opened over
// the page they will act on.
//
// ⛔ IT NAMES THE PAGE IT WILL ACT ON. Every verb here already exists on the
// ribbon, where it acts on "the page being read". A menu opened over minipage
// 7 that then rotated whatever page the viewer happened to be showing would be
// worse than no menu at all — so opening it MOVES the workbench to the page
// under the pointer first, and the menu's title says which page that is.
// Nothing in it is a verb the ribbon lacks; what it adds is the page.

package main

import (
	"strconv"

	"github.com/go-widgets/toolkit"
)

// pageVerbs are the ribbon verbs that act on a single page, in the order a
// menu offers them: what is done to the page, then what is taken from it.
//
// ⛔ Read from the verbs table by id rather than restated here. A menu with
// its own copy of a label is a menu that keeps the old one after a rename —
// the strip's icon map did exactly that, two hundred lines from the button it
// belonged to, and renaming a control silently dropped its mark.
var pageVerbs = []string{"rotate", "delete", "insert", "pagePicture", "readPage"}

// handleContext answers a secondary press: the page menu over whatever page
// it landed on, and nothing at all anywhere else.
//
// ⛔ A press that opens no menu must still CLOSE one that is up, and must
// report true either way. Reporting false lets the host fall back on the
// browser's own menu over a canvas we have already drawn our own on.
func (s *state) handleContext(x, y int) bool {
	s.closeContextMenu()
	s.openContextMenu(x, y)
	return true
}

// openContextMenu puts the page menu up at (x, y), over the page or the
// minipage the press landed on. It reports whether it opened one.
func (s *state) openContextMenu(x, y int) bool {
	page, ok := s.pageUnder(x, y)
	if !ok {
		return false
	}
	// ⛔ Go there FIRST. The verbs act on s.at, so a menu that opened without
	// moving would act on the page the reader was already on while sitting over
	// the one they pointed at.
	if page != s.at {
		s.goTo(page)
	}
	// ⛔ The first row is the page's number, and it is deliberately not
	// actionable: a menu that acts on one page has to SAY which, or a reader
	// who opened it over the wrong tile has no way to find out before pressing.
	// A nil Action is what the toolkit renders as a greyed row.
	items := []toolkit.MenuItem{{Label: "Page " + strconv.Itoa(page)}, {Separator: true}}
	for _, id := range pageVerbs {
		// ⛔ No "if the id is unknown, skip it". A verb that was renamed would
		// then drop out of this menu in silence, which is the same shape as the
		// strip's old icon map: a lookup by a string nobody checks, two files
		// from the thing it names. TestEveryPageVerbIsOneTheRibbonHas fails
		// loudly instead, and a missing id would panic here rather than leave a
		// reader a menu that is quietly one row short.
		v := verbs[id]
		run := v.run
		item := toolkit.MenuItem{Label: v.label, Icon: iconoirGlyph(v.icon)}
		// A verb that may not run keeps its row and loses its action, which is
		// how this toolkit greys one — and a row that vanished would move every
		// row under it between one opening and the next.
		if s.mayRun(v.when) {
			item.Action = func() { run(s); s.closeContextMenu() }
		}
		items = append(items, item)
	}
	s.menu = toolkit.NewContextMenu(toolkit.NewMenu(items))
	// ⛔ THE SURFACE FIRST, THEN THE ANCHOR. ContextMenu.MenuBounds clamps the
	// menu inside its OWN bounds so it cannot open off the edge of the window —
	// which means a menu that was never given bounds is clamped into a surface
	// of nothing: placed at the origin, zero pixels tall, drawn nowhere and
	// hit-testing nothing. It opened, it was "up", and it was not on the screen.
	s.menu.SetBounds(toolkit.Rect{W: s.w, H: s.h})
	s.menu.Popup(x, y)
	s.dirty = true
	return true
}

// closeContextMenu takes the menu down. Safe when none is up, because a press
// anywhere calls it.
func (s *state) closeContextMenu() {
	if s.menu == nil {
		return
	}
	s.menu.Close()
	s.menu = nil
	s.dirty = true
}

// pageUnder is the page the point is over: the minipage under it if the press
// landed in the rail, otherwise the page the viewer is showing there.
//
// ⛔ The rail is asked FIRST. The two overlap in no pixel, but the viewer
// answers for its own bounds only and the rail is outside them — asking the
// viewer first would simply miss every press on a minipage, which is the half
// of this feature a reader reaches for most.
func (s *state) pageUnder(x, y int) (int, bool) {
	if s.doc == nil {
		return 0, false
	}
	if page, ok := s.tileUnder(x, y); ok {
		return page, true
	}
	// ⛔ The arranging grid answers too, and it has to: that view REPLACES the
	// pages with tiles, so without this the one place a reader can see every
	// page at once is the one place the page menu did not open — and turning or
	// dropping the page they are looking at meant leaving the view that showed
	// them which it was.
	if page, ok := s.gridUnder(x, y); ok {
		return page, true
	}
	if b := s.pv.Bounds(); b.Contains(x, y) {
		// PageAt wants a point local to the widget and answers 0 for the
		// toolbar strip, a gap between cards or the scrollbar gutter — all of
		// which are places a menu about a page would be a lie.
		if page, _, _, ok := s.pv.PageAt(x-b.X, y-b.Y); ok {
			return page, true
		}
	}
	return 0, false
}

// tileUnder is the minipage under the point, if the rail is up and the point
// is on one of its tiles.
func (s *state) tileUnder(x, y int) (int, bool) {
	if s.railView == nil {
		return 0, false
	}
	for i, t := range railTiles(s.railView) {
		if t.Bounds().Contains(x, y) {
			return i + 1, true
		}
	}
	return 0, false
}

// railTiles walks the rail for its minipages, in order. The rail is a scroll
// view over a column of them, and the column is rebuilt on every repaint, so
// there is nothing to keep a reference to.
func railTiles(w toolkit.Widget) []*toolkit.Thumbnail {
	if t, ok := w.(*toolkit.Thumbnail); ok {
		return []*toolkit.Thumbnail{t}
	}
	p, ok := w.(interface{ Children() []toolkit.Widget })
	if !ok {
		return nil
	}
	var out []*toolkit.Thumbnail
	for _, c := range p.Children() {
		out = append(out, railTiles(c)...)
	}
	return out
}
