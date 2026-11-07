// Marking the passage a reader has selected.
//
// ⛔⛔ THE MARK GOES WHERE THE HIGHLIGHT IS, AND THAT IS NOT A DETAIL — it is
// the reason toolkit.TextSelection.SelectedRects exists. The reader drags
// across a paragraph, the toolkit lights the words it covers, and this writes a
// PDF annotation over them. If the two disagreed by half a word the mark would
// be wrong in the saved file while looking right on the screen, and nothing
// here would say so.
//
// So this does NOT re-derive the covered span from the runs and the caret
// positions. It asks the selection for the rectangles it PAINTS and inverts
// placeRun on each one. One rule, one implementation, measured both ways by
// TestQuadOfARunIsTheRunItWasPlacedFrom.
//
// ⛔ And a highlight is an ANNOTATION, not ink on the page. ISO 32000-1
// §12.5.6.10 Table 179: a text-markup annotation carries QuadPoints, "an array
// of 8 × n numbers specifying the coordinates of n quadrilaterals in default
// user space" — the page's own content stream is untouched, so the words
// underneath stay selectable, searchable and extractable, and a reader can
// delete the mark again. ops.Doc.Mark writes the /AP the spec says "shall take
// precedence", because a viewer that honours it must be shown one.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// Highlighter yellow, and the red a reader reaches for to strike something out.
// Both are the annotation's own colour, drawn at 40% alpha by the library, so
// the text stays legible under a highlight — see ops/markup.go, which found
// that out the hard way: render has no blend modes, so a /Multiply highlight
// came out as an opaque block over the words.
const (
	markYellowR, markYellowG, markYellowB = 1.0, 0.9, 0.15
	markRedR, markRedG, markRedB          = 0.85, 0.1, 0.1
)

// highlightSelection, underlineSelection and strikeSelection are the three
// things a reader does to a passage they have just selected.
func (s *state) highlightSelection() {
	s.markSelection(ops.MarkHighlight, "highlighted", markYellowR, markYellowG, markYellowB)
}

func (s *state) underlineSelection() {
	s.markSelection(ops.MarkUnderline, "underlined", markRedR, markRedG, markRedB)
}

func (s *state) strikeSelection() {
	s.markSelection(ops.MarkStrikeOut, "struck out", markRedR, markRedG, markRedB)
}

// markSelection writes one markup annotation per page the selection reaches.
//
// ⛔ It says how much it marked, and it clears the selection afterwards. Both
// for the reason copySelection says how much it copied: the mark lands in the
// file, not on the screen the reader is looking at, and a verb that worked in
// silence is one they press twice. A selection left lit after it has been
// marked is the same trap — the second press would write the annotation again.
//
// ⛔⛔ AND IT COUNTS CHARACTERS, NOT QUADRILATERALS, because a quadrilateral is
// not a thing a reader can see. There is one per RUN, and a run's size is the
// producer's choice: the page this was measured on sets one glyph at a time, so
// "the first chapter" is three runs and the first draft of this said
// "highlighted 3 lines" for one line of text. A page that draws the same line
// with a single Tj would have said "1 line" for the same gesture. The character
// count is the same number whoever made the file, and it is the number
// copySelection already reports for the same selection.
func (s *state) markSelection(kind ops.MarkupKind, said string, r, g, b float64) {
	marks, quads := s.selectionMarks(kind, r, g, b)
	if quads == 0 {
		s.fail("nothing is selected: drag across the words on a page first")
		return
	}
	chars := len([]rune(s.sel.SelectedText()))
	pages := "page"
	if len(marks) > 1 {
		pages = "pages"
	}
	note := fmt.Sprintf("%s %d character", said, chars)
	if chars != 1 {
		note += "s"
	}
	note += fmt.Sprintf(" on %d %s", len(marks), pages)
	if s.changeSaying(note, func(d *ops.Doc) error { return d.Mark(marks...) }) {
		s.sel.Clear()
	}
}

// selectionMarks is the selection as markup annotations, one per page it
// reaches, and the number of quadrilaterals in all of them — one per run the
// selection covers, which is what keeps the mark the same shape as the lit
// rectangles rather than one box drawn around them all.
//
// ⛔ The runs are placed again first, exactly as drawSelection does before it
// paints. The selection holds character positions, not pixels; the pixels come
// from PagedView.PageRect, which moves under a scroll, a zoom or a resize. So
// the rectangles have to be asked for against the layout as it stands NOW,
// otherwise a reader who scrolls between selecting and pressing Highlight gets
// a mark where the words used to be.
func (s *state) selectionMarks(kind ops.MarkupKind, r, g, b float64) (marks []ops.Markup, lines int) {
	for _, p := range s.selectionQuads() {
		lines += len(p.quads)
		marks = append(marks, ops.Markup{
			Page: p.page, Kind: kind, Quads: p.quads, R: r, G: g, B: b,
		})
	}
	return marks, lines
}

// pageQuads is one page and the part of the selection that lands on it.
type pageQuads struct {
	page  int
	quads []ops.Quad
}

// selectionQuads is the selection as quadrilaterals of page space, a group per
// page it reaches, in page order.
//
// ⛔ ONE PLACE WHERE THE SELECTION BECOMES PAGE SPACE. Marking a passage and
// redacting one are different acts with opposite consequences — one is written
// over the page and comes off again, the other takes the text out of the file —
// but they are the same question about WHERE, and they must answer it
// identically. A reader who highlights a line and then redacts it has to get
// the same words both times.
func (s *state) selectionQuads() []pageQuads {
	if s.sel.IsEmpty() {
		return nil
	}
	s.sel.SetRuns(s.textRuns(s.src, s.docStamp))
	rects := s.sel.SelectedRects()
	var out []pageQuads
	s.eachVisiblePage(s.src, func(i int, pw, ph float64, rect toolkit.Rect) {
		var quads []ops.Quad
		for _, sr := range rects {
			if inPage(sr, rect) {
				quads = append(quads, quadOfRect(sr, pw, ph, rect))
			}
		}
		if len(quads) == 0 {
			return // a page on screen that the selection does not reach
		}
		out = append(out, pageQuads{page: i, quads: quads})
	})
	return out
}

// inPage says whether a selection rectangle belongs to the page drawn at rect,
// by its CENTRE.
//
// ⛔ The centre, not a corner or containment. A run is placed inside its own
// page's rectangle, but placeRun rounds and clamps a run's width and height up
// to one pixel, so a rectangle on the last line of a page can end a pixel
// below it. A corner test would then hand that line to the next page — a mark
// on the wrong sheet — and a containment test would drop it silently. The page
// cards do not overlap, so exactly one page claims each rectangle.
func inPage(r, page toolkit.Rect) bool {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	return cx >= page.X && cx < page.X+page.W && cy >= page.Y && cy < page.Y+page.H
}

// quadOfRect is placeRun run backwards: one rectangle of lit pixels, as the
// quadrilateral of page space it covers.
//
// PDF counts upward from the foot of the page and a screen counts downward from
// the top, so the top edge of the rectangle is the HIGHER y in page space.
// QuadOf sorts the corners it is given, which is what keeps that from being a
// bow-tie if this ever has them the other way round.
//
// ⛔ It is exact to within placeRun's rounding, not beyond it: placeRun
// truncates each edge to a whole pixel, so a round trip is accurate to one
// screen pixel divided by the scale — a third of a point at fit-width on A4,
// well inside the half-line slack a highlight needs. There is no sub-pixel
// position to recover, because the pixels are what the reader saw.
func quadOfRect(r toolkit.Rect, pw, ph float64, page toolkit.Rect) ops.Quad {
	sx := float64(page.W) / pw
	sy := float64(page.H) / ph
	x0 := float64(r.X-page.X) / sx
	x1 := float64(r.X+r.W-page.X) / sx
	top := ph - float64(r.Y-page.Y)/sy
	bottom := ph - float64(r.Y+r.H-page.Y)/sy
	return ops.QuadOf(x0, bottom, x1, top)
}
