// The verbs that change how a page and a sheet of paper line up: cutting one
// page into several, putting every page onto one, spreading one page over
// several sheets, and straightening a scan that went in crooked.
//
// ⛔⛔ EVERY ONE OF THESE WAS ALREADY BUILT AND UNREACHABLE. `ops` carries
// Divide, OnePage, Poster and Deskew, the workbench links `ops`, and none of
// the four was on the ribbon — so the fleet could do them and nobody using the
// workbench could. Measured against BentoPDF's own tool tables (the bentogap
// instrument, 2026-11-08): 44 of its 111 tools were in that state, which is
// more than the 18 it genuinely cannot do. A library with no caller is not
// delivered, and a verb with no control is not either.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
)

// divide cuts every page into a grid of smaller pages, each holding its own
// part of the original.
func (s *state) divide() {
	across, down := s.tools.cutAcross.Get(), s.tools.cutDown.Get()
	if across < 1 || down < 1 || across*down < 2 {
		s.fail("cutting asks for at least two pieces: set across and down beside it")
		return
	}
	if !s.changeSaying(fmt.Sprintf("cut each page into %d × %d", across, down),
		func(d *ops.Doc) error { return d.Divide(across, down) }) {
		return
	}
	s.at = 1
	s.refresh()
}

// onePage puts every page of the document onto a single tall page.
func (s *state) onePage() {
	if !s.changeSaying("put every page onto one",
		func(d *ops.Doc) error { return d.OnePage() }) {
		return
	}
	s.at = 1
	s.refresh()
}

// poster spreads each page over a grid of sheets, to be printed larger than
// the paper and joined up afterwards.
//
// ⛔ Not the same act as divide, though it takes the same two numbers. Divide
// cuts a page into smaller pages of its own content; this enlarges one page
// across several sheets. The panel therefore keeps two pairs of numbers, so
// "2 × 2" does not mean halves in one place and quarters-enlarged in the other
// under the same control.
func (s *state) poster() {
	across, down := s.tools.tilesAcross.Get(), s.tools.tilesDown.Get()
	if across < 1 || down < 1 || across*down < 2 {
		s.fail("a poster asks for at least two tiles: set them beside it")
		return
	}
	if !s.changeSaying(fmt.Sprintf("spread each page over %d × %d sheets", across, down),
		func(d *ops.Doc) error { return d.Poster(across, down) }) {
		return
	}
	s.at = 1
	s.refresh()
}

// deskew straightens the pages of a scan that went in crooked.
//
// ⛔ IT SAYS WHAT IT MEASURED, not just that it ran. ops.Doc.Deskew hands back
// a PageSkew per page — the angle it found, whether it turned the page, and why
// a page could not be measured — and that is the whole value of the verb: a
// reader who sees "measured 12 pages, straightened 3" knows the other nine were
// already level, where "done" would leave them wondering whether it worked.
// The zero DeskewOptions is half a degree at 150 DPI, which is the threshold
// below which turning a page costs it a transformation it did not need.
func (s *state) deskew() {
	var found []ops.PageSkew
	if !s.changeSaying("", func(d *ops.Doc) error {
		skews, err := d.Deskew(ops.DeskewOptions{})
		found = skews
		return err
	}) {
		return
	}
	turned, unreadable := 0, 0
	for _, sk := range found {
		switch {
		case sk.Err != nil:
			unreadable++
		case sk.Corrected:
			turned++
		}
	}
	note := fmt.Sprintf("measured %d page", len(found))
	if len(found) != 1 {
		note += "s"
	}
	switch {
	case turned == 0:
		note += ", all of them already level"
	default:
		note += fmt.Sprintf(", straightened %d", turned)
	}
	if unreadable > 0 {
		note += fmt.Sprintf("; %d could not be measured", unreadable)
	}
	s.note = note
	s.refresh()
}
