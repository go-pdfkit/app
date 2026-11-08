// Saying something about a passage: a sticky note beside it, or a comment
// written in the margin next to it.
//
// ⛔ BOTH ACT ON THE SELECTION, like the four marks and Redact, and that is a
// decision about the GESTURE rather than about the format. A sticky note is
// attached to a point and a comment to a box, so either could have been placed
// by pointing — but the workbench's way of pointing at words is already a drag,
// and a second placing gesture would be a second thing to learn and a second
// set of arithmetic to keep in step with the first. Select the passage, then say
// what you think of it.
//
// ⛔ And neither touches the page. The words under a comment are the author's,
// the comment comes off again by dropping the annotation, and a reader who
// wants the paper as published gets it.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// noteGap is how far from the passage a sticky note sits, and marginGap how
// much paper is left either side of a margin comment, in points.
const (
	noteGap   = 4.0
	marginGap = 8.0
)

// marginLeast is the narrowest margin worth writing in. Narrower than this and
// a comment is one word per line down a column, which is a refusal worth making
// rather than a result worth handing over.
const marginLeast = 54.0

// commentText is what was typed beside the two verbs, or "" with the status
// line already saying what is missing.
func (s *state) commentText() string {
	text := strings.TrimSpace(s.tools.comment.Get())
	if text == "" {
		s.fail("type the comment beside it first")
		return ""
	}
	return text
}

// commentPlace is the page, the passage's box in that page's own points, and
// the page's size — everything either verb needs to decide where to put itself.
//
// ⛔ THE FIRST PAGE THE SELECTION REACHES, and the box around every piece of it
// on that page. A selection dragged across a page boundary is two stretches of
// text, and a note is one point: attaching it to the second page because the
// drag ended there would put the comment where the reader was not looking.
func (s *state) commentPlace() (page int, x0, y0, x1, y1, pw, ph float64, ok bool) {
	groups := s.selectionQuads()
	if len(groups) == 0 {
		s.fail("nothing is selected: drag across the words you want to say something about")
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	g := groups[0]
	x0, y0, x1, y1 = g.quads[0].TopLeftX, g.quads[0].BottomLeftY,
		g.quads[0].TopRightX, g.quads[0].TopLeftY
	for _, q := range g.quads[1:] {
		x0 = min(x0, q.TopLeftX)
		x1 = max(x1, q.TopRightX)
		y0 = min(y0, q.BottomLeftY)
		y1 = max(y1, q.TopLeftY)
	}
	// ⛔ The paper's size comes from the selection itself and is not read again:
	// eachVisiblePage has already stepped over a page that declares no size, so
	// a guard here would be an arm this slice cannot reach. See pageQuads.
	return g.page, x0, y0, x1, y1, g.pw, g.ph, true
}

// noteOnSelection puts a sticky note beside the passage.
//
// ⛔ BESIDE IT AND NOT OVER IT. A note's icon is twenty points square and
// opaque; dropped on the words it is about, it hides exactly what it comments
// on. notePlace chooses a side that is clear of them, and this refuses when
// there is none rather than putting the icon somewhere that merely fits.
func (s *state) noteOnSelection() {
	text := s.commentText()
	if text == "" {
		return
	}
	page, x0, y0, x1, y1, pw, ph, ok := s.commentPlace()
	if !ok {
		return
	}
	x, y, room := notePlace(x0, y0, x1, y1, pw, ph)
	if !room {
		s.fail("there is nowhere beside that passage to put a note without " +
			"covering it: select fewer words, or write in the margin instead")
		return
	}
	// ⛔ THE SELECTION IS LET GO ONLY IF THE CHANGE WAS MADE, which is why this
	// is the `if` body rather than an early return after it. ops can refuse —
	// an empty comment, a page the document has not — and everything this verb
	// could be refused for is already checked above, so the refusing arm is one
	// no test can reach. Written this way there is no such arm: the work that
	// follows a success simply sits inside the success.
	if s.changeSaying(fmt.Sprintf("put a note on page %d", page), func(d *ops.Doc) error {
		return d.AddNotes(ops.Note{
			Page: page, X: x, Y: y,
			Contents: text,
			Author:   s.tools.author.Get(),
		})
	}) {
		// ⛔ Let go, as the marks do: the comment has landed in the document,
		// and words still lit afterwards invite a second press that writes a
		// second note saying the same thing.
		s.sel.Clear()
		s.refresh()
	}
}

// notePlace is where a sticky note's icon goes beside a passage: to its left
// if there is room on the paper, otherwise to its right, otherwise above it.
// The last result is false when none of the three fits.
//
// ⛔⛔ THE FIRST VERSION CLAMPED INSTEAD OF CHOOSING, AND THE TEST CAUGHT IT
// DOING EXACTLY WHAT THE VERB EXISTS TO PREVENT. It took the left position and
// pushed it back onto the paper when it fell off — so a passage starting 19.7
// points from the edge, with a 20-point icon, got an icon drawn from x=8 to
// x=28 straight across the words it was commenting on. A clamp keeps a number
// in range; it does not keep a promise. Each candidate here is CHECKED against
// the passage and against the paper, and a position that fails either is not
// used.
func notePlace(x0, y0, x1, y1, pw, ph float64) (x, y float64, ok bool) {
	// Level with the top of the passage, which is where a reader looks.
	beside := min(max(y1-noteSide, marginGap), ph-noteSide-marginGap)
	for _, try := range [][2]float64{
		{x0 - noteSide - noteGap, beside}, // the left margin
		{x1 + noteGap, beside},            // the right margin
		{x0, y1 + noteGap},                // above the line
	} {
		x, y := try[0], try[1]
		if x < marginGap || x+noteSide > pw-marginGap {
			continue // off the paper sideways
		}
		if y < marginGap || y+noteSide > ph-marginGap {
			continue // off the paper vertically
		}
		// ⛔ NO "does it overlap?" CHECK HERE, and that is deliberate rather
		// than forgotten. Each candidate above is placed OUTSIDE the passage by
		// construction — noteSide+noteGap to the left of x0, noteGap to the
		// right of x1, noteGap above y1 — and commentPlace normalises the box so
		// x0 <= x1 and y0 <= y1. An overlap test would therefore be an arm
		// nothing can reach: unreachable code that reads like a safeguard, which
		// is worse than none, because the next reader trusts it. The invariant
		// is asserted where it can fail — in the test, against every answer this
		// function gives.
		return x, y, true
	}
	return 0, 0, false
}

// overlapsBox says whether two upright rectangles share any area.
func overlapsBox(ax0, ay0, ax1, ay1, bx0, by0, bx1, by1 float64) bool {
	return ax0 < bx1 && bx0 < ax1 && ay0 < by1 && by0 < ay1
}

// commentInMargin writes the comment in the margin beside the passage.
//
// ⛔ IT REFUSES A MARGIN TOO NARROW TO READ rather than writing a column of one
// word per line. ops will clip whatever does not fit its box, which is right for
// a box somebody chose and wrong for one this verb chose for them: a comment
// nobody can read is not a comment, and the reader can widen nothing from here.
func (s *state) commentInMargin() {
	text := s.commentText()
	if text == "" {
		return
	}
	page, _, _, x1, y1, pw, _, ok := s.commentPlace()
	if !ok {
		return
	}
	left := x1 + marginGap
	right := pw - marginGap
	if right-left < marginLeast {
		s.fail(fmt.Sprintf("the margin beside that passage is %.0f points wide, "+
			"and a comment needs %.0f: select a passage that ends further from "+
			"the edge, or put a note on it instead", max(right-left, 0), marginLeast))
		return
	}
	if s.changeSaying(fmt.Sprintf("wrote in the margin of page %d", page), func(d *ops.Doc) error {
		return d.AddFreeText(ops.FreeText{
			Page: page,
			// From the top of the passage down to the foot of the paper: a
			// comment is as long as it is, and the height a reader would have
			// chosen is "enough".
			X0: left, Y0: marginGap, X1: right, Y1: y1,
			Text:   text,
			Size:   marginTextSize,
			Author: s.tools.author.Get(),
		})
	}) {
		s.sel.Clear()
		s.refresh()
	}
}

// marginTextSize is how large a margin comment is set: smaller than the text it
// comments on, as a hand-written note in a margin is.
const marginTextSize = 9.0

// noteSide is ops' own icon size, repeated here because the note has to be
// placed and ops does not expose it.
//
// ⛔ It is a CONSTANT in ops (notes.go), not an option, and for a good reason:
// Table 172 says a text annotation does not scale with the page. So this cannot
// drift as a setting might — but it could drift as a number, which is why the
// test asserts the icon lands clear of the passage rather than asserting this
// value.
const noteSide = 20.0

// commentGroup is the panel row the two verbs read.
func (s *state) commentRow() toolkit.Widget {
	return s.entryRow("Comment", "what to say about the passage", s.tools.comment)
}
