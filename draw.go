// Drawing on a passage: a box round it, a ring round it, an arrow pointing at
// it from the margin.
//
// ⛔ THEY ACT ON THE SELECTION, like the four marks and the two comments, and
// for the reason written in comment.go: the workbench's way of pointing at
// words is a drag, and a second placing gesture would be a second thing to
// learn and a second set of arithmetic to keep in step with the first. A shape
// wants a REGION rather than a run of text, and the selection's own box is one.
//
// ⛔ AND THEY ARE ANNOTATIONS. A box drawn into the content stream would be a
// different document; these come off again by dropping them, like every other
// mark on this tab.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
)

// shapeInk is how thick these are drawn and in what colour: a reviewer's red,
// the same the underline and the strike-out use, at a width that reads on a
// printed page without hiding the words.
//
// ⛔ NOT A CONTROL. Every one of these is a mark somebody makes while reading,
// and a reader who wants to choose a colour wants a palette, which is a panel
// of its own rather than two more boxes squeezed onto this one. The marks beside
// them took the same decision for the same reason.
const shapeInk = 1.5

// shapeRoom is how much paper the box is given beyond the words, so a box does
// not sit on the glyphs it encloses.
const shapeRoom = 3.0

// arrowReach is how long an arrow is, from the margin to the words.
const arrowReach = 40.0

// boxSelection draws a rectangle round the selected words.
func (s *state) boxSelection() {
	s.shapeOnSelection(ops.ShapeRectangle, "drew a box round")
}

// ringSelection draws an ellipse round them, which is what somebody circling a
// word on paper does.
func (s *state) ringSelection() {
	s.shapeOnSelection(ops.ShapeEllipse, "drew a ring round")
}

// shapeOnSelection draws one closed shape round the passage.
func (s *state) shapeOnSelection(kind ops.ShapeKind, said string) {
	page, x0, y0, x1, y1, pw, ph, ok := s.commentPlace()
	if !ok {
		return
	}
	// ⛔ Kept on the paper. A passage at the very edge of a page would otherwise
	// be given a box whose own border runs off it, and ops grows the Rect by
	// half the line width on top of that.
	x0, y0 = max(x0-shapeRoom, shapeInk), max(y0-shapeRoom, shapeInk)
	x1, y1 = min(x1+shapeRoom, pw-shapeInk), min(y1+shapeRoom, ph-shapeInk)
	// ⛔ NO "is there any room left?" CHECK, and that is measured rather than
	// assumed. After the clamps the box could only collapse on paper about
	// three points across — and a page that small cannot have a selection on
	// it at all, because the words would fall outside the rectangle the viewer
	// draws and `inPage` drops them. The guard that was here could not fire,
	// and an unreachable refusal reads like a safeguard the next person trusts.
	//
	// The net is still there and is ops': AddShapes refuses a rectangle of no
	// area by name, and changeSaying puts its words on the status line. One
	// check, in the place that can see the numbers.
	if s.changeSaying(fmt.Sprintf("%s the words on page %d", said, page),
		func(d *ops.Doc) error {
			return d.AddShapes(ops.Shape{
				Page: page, Kind: kind,
				X0: x0, Y0: y0, X1: x1, Y1: y1,
				R: markRedR, G: markRedG, B: markRedB,
				Width:  shapeInk,
				Author: s.tools.author.Get(),
			})
		}) {
		s.sel.Clear()
		s.refresh()
	}
}

// arrowToSelection draws an arrow from the margin to the passage.
//
// ⛔ IT POINTS AT THE WORDS AND STARTS AWAY FROM THEM, which is the one thing a
// line over a passage must not get wrong: a line drawn THROUGH the selection
// would be a strike-out, and there is already a verb for that two controls
// along. So it comes in from whichever margin has room, and refuses when
// neither has.
func (s *state) arrowToSelection() {
	page, x0, y0, x1, y1, pw, _, ok := s.commentPlace()
	if !ok {
		return
	}
	midY := (y0 + y1) / 2
	// The left margin if the words leave room for an arrow in it, otherwise the
	// right. The tip sits just clear of the glyphs either way.
	switch {
	case x0-shapeRoom-arrowReach >= shapeInk:
		s.drawArrow(page, x0-shapeRoom-arrowReach, midY, x0-shapeRoom, midY)
	case x1+shapeRoom+arrowReach <= pw-shapeInk:
		s.drawArrow(page, x1+shapeRoom+arrowReach, midY, x1+shapeRoom, midY)
	default:
		s.fail(fmt.Sprintf("neither margin beside those words has the %.0f "+
			"points an arrow needs: select words further from the edge, or draw "+
			"a box round them instead", arrowReach))
	}
}

// drawArrow puts one arrow on the page, from (x0, y0) to the head at (x1, y1).
func (s *state) drawArrow(page int, x0, y0, x1, y1 float64) {
	if s.changeSaying(fmt.Sprintf("drew an arrow at the words on page %d", page),
		func(d *ops.Doc) error {
			return d.AddShapes(ops.Shape{
				Page: page, Kind: ops.ShapeLine,
				X0: x0, Y0: y0, X1: x1, Y1: y1,
				R: markRedR, G: markRedG, B: markRedB,
				Width: shapeInk,
				Arrow: true,
				// ⛔ A filled head, which Table 176 makes conditional: a
				// ClosedArrow is "filled with the annotation's interior color,
				// IF ANY", so without one the head is an outline. An outlined
				// arrowhead at this size is a smudge; the fill is the line's own
				// colour, so the head reads as part of the line.
				Fill:   &[3]float64{markRedR, markRedG, markRedB},
				Author: s.tools.author.Get(),
			})
		}) {
		s.sel.Clear()
		s.refresh()
	}
}
