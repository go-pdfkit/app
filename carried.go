// The files a document carries inside it, and mixing another file's pages into
// this one's.
//
// ⛔ A DOCUMENT CAN HOLD OTHER FILES, and nothing in the workbench ever said
// so. `ops` has had Attach, Attachments and Detach throughout; the ribbon had
// no way to see what a document was carrying, let alone take it out — so a PDF
// arriving with a spreadsheet inside it looked like a PDF with nothing inside
// it. Three of BentoPDF's tools (Add, Extract and Edit Attachments) were that
// one gap, and a fourth (Alternate & Mix Pages) was three lines over a seam the
// Sheet tab already had.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// readingCarried is the reading that shows the files inside the document.
const readingCarried = "carried"

// readCarried shows what the document carries, or puts the page back.
func (s *state) readCarried() { s.read(readingCarried) }

// carriedView is every file travelling inside the document, with a way of
// taking each one out and a way of dropping it.
//
// ⛔ PLAIN BUTTONS, NOT VERBS, for the reason imagesView gives: this view is
// rebuilt on every refresh, and a verb appends a command to the state that is
// never taken off again — a list that grows for as long as somebody keeps
// looking.
//
// ⛔ AND IT IS THE DOCUMENT'S OWN LIST, not the saved file's. s.doc is what
// would come out of Save, so a file attached a moment ago is in it and a file
// just dropped is gone from it. Reading the attachments out of the written
// bytes instead would show the list one refresh late, which for somebody
// attaching two files in a row means showing them the wrong one.
func (s *state) carriedView() toolkit.Widget {
	if s.doc == nil {
		return toolkit.NewLabel("nothing is open")
	}
	carried := s.doc.Attachments()
	if len(carried) == 0 {
		return toolkit.NewLabel("this document carries no files inside it")
	}
	col := newColumn()
	col.add(toolkit.NewLabel("Files travelling inside this document:"), lineH)
	for _, f := range carried {
		name, data := f.Name, f.Data
		row := toolkit.NewSettingRow(name, buttons(
			button("Take it out", toolkit.ButtonDefault, func() { s.handOver(name, data) }),
			button("Drop it", toolkit.ButtonDefault, func() { s.detach(name) }),
		))
		row.Subtitle = carriedSays(f)
		col.add(row, 2*lineH+8)
	}
	return col.scrollerOf(s.pageW() - 2*margin)
}

// carriedSays describes one carried file.
func carriedSays(f ops.Attachment) string {
	size := fmt.Sprintf("%d bytes", len(f.Data))
	if f.Description == "" {
		return size + ", with nothing said about what it is"
	}
	return size + " — " + f.Description
}

// attach puts another file inside the document.
//
// ⛔ The picker is asked for the way Open and `merge` ask: the press that
// reaches it is the one somebody made, which is what a browser requires before
// it will show a file chooser at all.
func (s *state) attach() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	s.host.Open(func(name string, data []byte) {
		// ⛔ ops refuses a name already used rather than replacing what is
		// there — "two files under one name is a document that has lost one of
		// them" — and that refusal is the useful answer, so it goes through
		// changeSaying and reaches the status line unchanged.
		if s.changeSaying(fmt.Sprintf("put %s inside the document", name),
			func(d *ops.Doc) error { return d.Attach(name, data, "") }) {
			s.tools.reading.Set(readingCarried)
			s.refresh()
		}
	})
}

// detach drops one carried file.
//
// ⛔ NO `s.doc == nil` CHECK OF ITS OWN. changeSaying refuses a document that
// is not there and says so in the same words, so a second guard here would
// make its own else-branch unreachable — one guard, at the place that already
// has it.
func (s *state) detach(name string) {
	// ⛔ Detach says whether it found anything, and a verb that reported
	// success for a name the document never carried would be telling somebody
	// their file is gone when it was never there.
	gone := false
	if !s.changeSaying("dropped "+name, func(d *ops.Doc) error {
		gone = d.Detach(name)
		return nil
	}) {
		return
	}
	if !gone {
		s.fail("this document carries no file called " + name)
	}
}

// interleave takes one page from this document and one from another, in turn.
func (s *state) interleave() {
	s.withAnother("mixed in", func(d, other *ops.Doc) error { return d.Interleave(other) })
}
