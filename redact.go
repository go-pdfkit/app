// Taking the selected words out of the document.
//
// ⛔⛔ THIS IS THE ONE VERB THAT DESTROYS SOMETHING, and the only one on the
// Read tab that does. Everything else there shows the document or writes
// something over it; this removes text from the page's own content stream, and
// a saved file cannot be talked out of it.
//
// It is here rather than on Protect because it acts on a SELECTION, and a
// selection is made two controls away: a reader drags across a passage and the
// verb has to be where their eye already is. A redaction they had to go and
// find on another tab is one they would do with a black rectangle instead,
// which is the thing this exists to stop them doing.
//
// ⛔ The refusal comes from the library and reaches the status line unchanged.
// ops.Doc.Redact will not remove text it cannot remove completely — text inside
// a form, across an image, under an annotation, or drawn by an operator that
// also draws words outside the selection — and says which. Those messages are
// the useful part: a reader told "this line is drawn in one piece, and taking
// it would take more than you asked for" can select the whole line instead. A
// workbench that swallowed them and drew a box would be the leak.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
)

// redactSelection removes the selected words from the document and paints the
// space they occupied.
func (s *state) redactSelection() {
	groups := s.selectionQuads()
	if len(groups) == 0 {
		s.fail("nothing is selected: drag across the words on a page first")
		return
	}
	// ⛔ COUNTED BEFORE THE CHANGE. changeSaying rebuilds the document and the
	// selection is cleared after it, so the words are gone by the time there is
	// anything to say about them.
	chars := len([]rune(s.sel.SelectedText()))
	rs := make([]ops.Redaction, 0, len(groups))
	for _, g := range groups {
		rs = append(rs, ops.Redaction{Page: g.page, Quads: g.quads})
	}
	pages := "page"
	if len(rs) > 1 {
		pages = "pages"
	}
	note := fmt.Sprintf("removed %d character", chars)
	if chars != 1 {
		note += "s"
	}
	note += fmt.Sprintf(" from %d %s — the text is gone from the file, not covered",
		len(rs), pages)
	if s.changeSaying(note, func(d *ops.Doc) error { return d.Redact(rs...) }) {
		s.sel.Clear()
	}
}
