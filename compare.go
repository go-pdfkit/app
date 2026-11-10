// Comparing the document on the screen with another version of it.
//
// ⛔ THE DIRECTION IS SAID OUT LOUD, every time. A comparison has two documents
// in it and a reader who has the order backwards reads every addition as a
// deletion — so the file they chose is the EARLIER one, the document they have
// open is the later one, and the heading of the report says so in those words
// rather than leaving it to be worked out from the page numbers.
//
// Nothing here changes the document. It is a reading, like the text and the
// tables: it replaces the picture of the page and is put away by pressing the
// same verb again.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// readingCompare is the report where the picture of the page was.
const readingCompare = "compare"

// A comparison is what the last one found, and what it was against.
//
// ⛔ IT IS DROPPED THE MOMENT THE DOCUMENT CHANGES, in changeSaying, which is
// the one place every verb goes through. A report that stayed on the screen
// after a page was deleted would name pages that are no longer there and
// changes that are no longer true, and it would look exactly as authoritative
// as it did a second earlier.
type comparison struct {
	against string
	pairs   []ops.PagePair
}

// compareWithAFile asks for another version of this document and reports what
// changed between it and the one on the screen.
func (s *state) compareWithAFile() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	s.host.Open(func(name string, data []byte) {
		if len(data) == 0 {
			return // the person changed their mind, which is not an error
		}
		// ⛔ saveBytes rather than doc.Bytes: the comparison has to be against
		// what SAVE would write, and for a form that has been filled in those
		// are not the same bytes — a filled form is saved as the file it came
		// from with the answers appended, because that is the only way of
		// saving one that keeps it a form. This is the one function that knows
		// the difference.
		mine, msg := s.saveBytes()
		if msg != "" {
			s.fail(msg)
			return
		}
		// ⛔ The chosen file FIRST: it is the earlier version, and what is
		// reported is what this document did to it.
		pairs, err := ops.Compare(data, mine)
		if err != nil {
			s.fail(err.Error())
			return
		}
		s.comparison = &comparison{against: name, pairs: pairs}
		s.tools.reading.Set(readingCompare)
		s.held = 0
		s.note = fmt.Sprintf("compared with %s — %s", name, pagesChanged(pairs))
		s.refresh()
	})
}

// pagesChanged is how many of the pairs differ, in words.
func pagesChanged(pairs []ops.PagePair) string {
	changed := 0
	for _, p := range pairs {
		if p.Changed() {
			changed++
		}
	}
	switch changed {
	case 0:
		return fmt.Sprintf("the two say the same thing on all %d page(s)", len(pairs))
	case 1:
		return fmt.Sprintf("1 of %d page(s) changed", len(pairs))
	default:
		return fmt.Sprintf("%d of %d page(s) changed", changed, len(pairs))
	}
}

// compareView is the report: the pairs that differ, and what differs on each.
func (s *state) compareView() toolkit.Widget {
	if s.comparison == nil {
		return toolkit.NewLabel("press Compare with a file to choose the other version")
	}
	c := s.comparison
	col := newColumn()
	col.add(toolkit.NewLabel("against "+c.against+" — "+pagesChanged(c.pairs)), lineH)
	// ⛔ The sentence that keeps a reader from reading the report backwards.
	col.add(toolkit.NewLabel("the left page is "+c.against+"; the right page is this document"), lineH)
	col.add(toolkit.NewLabel(""), lineH)
	for _, p := range c.pairs {
		if !p.Changed() {
			continue
		}
		col.add(toolkit.NewLabel(pairHeading(p)), lineH)
		for _, ch := range p.Changes {
			col.add(toolkit.NewLabel(fmt.Sprintf("    %-18s %s", ch.Kind, ch.What)), lineH)
		}
	}
	return col.scrollerOf(s.pageW() - 2*margin)
}

// pairHeading is the heading of one pair: "page 2 → page 3", with a dash for
// the side that has no page.
func pairHeading(p ops.PagePair) string {
	side := func(n int) string {
		if n == 0 {
			return "—"
		}
		return fmt.Sprintf("page %d", n)
	}
	return side(p.Before) + " → " + side(p.After)
}
