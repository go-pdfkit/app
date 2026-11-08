// Replacing the words a page already says, rather than marking them, hiding
// them or writing beside them.
//
// ⛔⛔ THIS IS THE ONE VERB THAT CHANGES WHAT THE DOCUMENT SAYS. A mark is a
// layer that comes off, a comment is an annotation that comes off, a redaction
// takes words out and says so. This puts other words in their place, and the
// result is a document that reads as though its author had written something
// they did not. So it says what it replaced, in both directions, every time.
//
// ⛔ AND IT WORKS ON ONE PIECE OF TEXT AS THE PAGE DREW IT, which is not the
// same as "the words you selected". A PDF draws text in operations, and a page
// is free to make each one a line, a word or a single glyph — TeX sets type
// glyph by glyph. ops.Doc.ReplaceText rewrites ONE operation, because the
// others in a shared operator would have to be re-drawn from their characters
// and a font's codes cannot be recovered from characters. So a selection
// covering several operations is refused, with the number, rather than half
// done.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
)

// replaceWords puts the typed text in place of the selected text.
func (s *state) replaceWords() {
	now := strings.TrimSpace(s.tools.replacement.Get())
	if now == "" {
		s.fail("type the words to put there instead, beside it")
		return
	}
	groups := s.selectionQuads()
	if len(groups) == 0 {
		s.fail("nothing is selected: drag across the words you want to replace")
		return
	}
	g := groups[0]
	x0, y0, x1, y1 := quadsBox(g.quads)
	found := runsUnder(s.runsFor(s.src, g.page, s.docStamp), [4]float64{x0, y0, x1, y1})
	pieces := operationsOf(found)
	switch {
	case len(pieces) == 0:
		s.fail("the words selected are not text this page drew: nothing there " +
			"can be replaced")
		return
	case len(pieces) > 1:
		// ⛔ THE REFUSAL CARRIES THE NUMBER AND WHAT TO DO, because on a page
		// that sets type glyph by glyph every word is several operations and a
		// bare "cannot" would read as a broken verb rather than as a page shaped
		// the wrong way for this.
		s.fail(fmt.Sprintf("this page draws those words in %d separate "+
			"operations, and they can only be replaced one at a time: select "+
			"one piece of the text, or redact the passage and write the "+
			"replacement in the margin", len(pieces)))
		return
	}
	was := pieces[0]
	if !s.changeSaying(fmt.Sprintf("replaced %q with %q on page %d",
		was.Text, now, g.page), func(d *ops.Doc) error {
		return d.ReplaceText(ops.TextEdit{Page: g.page, At: was.Op, Now: now})
	}) {
		return
	}
	s.sel.Clear()
	s.refresh()
}

// quadsBox is the smallest rectangle holding every quad.
func quadsBox(quads []ops.Quad) (x0, y0, x1, y1 float64) {
	x0, y0 = quads[0].TopLeftX, quads[0].BottomLeftY
	x1, y1 = quads[0].TopRightX, quads[0].TopLeftY
	for _, q := range quads[1:] {
		x0 = min(x0, q.TopLeftX)
		x1 = max(x1, q.TopRightX)
		y0 = min(y0, q.BottomLeftY)
		y1 = max(y1, q.TopLeftY)
	}
	return x0, y0, x1, y1
}

// operationsOf is one run per text-showing operation among these runs, in
// reading order.
//
// ⛔ SEVERAL RUNS CAN SHARE ONE OPERATION — extract says so itself: a TJ array
// shows several strings with a single operator. So counting RUNS would refuse a
// selection that is one operation drawn in pieces, and counting operations is
// what the question "can this be rewritten" actually asks. A run with no span
// at all is skipped: it was drawn from somewhere this cannot edit, and ops
// refuses it by name if it ever arrives.
func operationsOf(runs []extract.Run) []extract.Run {
	var out []extract.Run
	seen := map[extract.Span]bool{}
	for _, r := range runs {
		if r.Op.Empty() || seen[r.Op] {
			continue
		}
		seen[r.Op] = true
		out = append(out, r)
	}
	return out
}
