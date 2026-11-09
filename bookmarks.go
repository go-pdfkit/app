// Writing the table of contents: the other half of the rail, which has shown a
// document's bookmarks since the rail existed and could only ever drop them.
//
// ⛔ THE GRAMMAR IS THE LIBRARY'S, NOT THIS PROGRAM'S. `pdfops outline -from
// toc.txt` has read a table of contents as text for as long as the verb has
// existed — a title, a tab, a page number, two spaces of indent for each level
// — and that parser now lives in ops as ParseOutline, so the file somebody
// writes for the command is the file they hand to the workbench. A second
// parser here would have been a second grammar within a week.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// maxContentsBytes bounds the table of contents read from a file. A table of
// contents is a few thousand bytes; this is a thousand times that, and it is
// here because the file comes from whoever is sitting at the browser and the
// parser allocates a row per line.
const maxContentsBytes = 1 << 20

// bookmarkTitleMax bounds the title taken from a passage, in runes. A reader
// who selects three paragraphs and presses the button means the first words of
// them, not all of it: a bookmark is a line in a rail.
const bookmarkTitleMax = 80

// bookmarkPage adds one bookmark to whatever the document already carries,
// pointing at the page on the screen — or at the page a passage is on, titled
// with that passage, when one is selected.
//
// ⛔ THE ONES ALREADY THERE ARE READ AND WRITTEN BACK. ops.SetOutline writes
// the bookmarks it is given INSTEAD of the sources' own, so adding one means
// handing it the whole tree. A verb that passed only the new entry would have
// read as "bookmark this page" and acted as "replace the table of contents
// with this page".
func (s *state) bookmarkPage() {
	page, title := s.bookmarkHere()
	rows := append(s.levelsNow(), ops.Level{Title: title, Page: page})
	// ⛔ The refusal is not branched on, and that is the whole of what this has
	// to say about it: ops.Nest refuses a row more than one level under the row
	// before it, and these rows cannot hold one. walkOutline appends a row and
	// then recurses one level deeper, so a document's own bookmarks step by one
	// or not at all, and the entry appended here sits at the top level.
	// TestADocumentsOwnBookmarksNeverSkipALevel is what holds that, and a
	// branch here would be an arm no test could reach.
	marks, _ := ops.Nest(rows)
	s.changeSaying(fmt.Sprintf("bookmarked page %d as %q", page, title),
		func(d *ops.Doc) error {
			d.SetOutline(marks)
			return nil
		})
}

// bookmarkHere is what a new bookmark says and where it points: the selected
// passage and the page it is on, or the page being read when nothing is
// selected.
//
// ⛔ THE PAGE COMES FROM THE SELECTION AND NOT FROM THE VIEW. A reader can
// select a passage on page three and scroll to page two before pressing the
// button; taking the title from the selection and the page from the view would
// write a bookmark that says one thing and goes somewhere else.
func (s *state) bookmarkHere() (page int, title string) {
	quads := s.selectionQuads()
	said := strings.TrimSpace(s.sel.SelectedText())
	if len(quads) == 0 || said == "" {
		return s.at, fmt.Sprintf("Page %d", s.at)
	}
	return quads[0].page, oneLine(said, bookmarkTitleMax)
}

// oneLine is a passage as a title: its first line, cut to a length a rail can
// draw, with an ellipsis when it was cut.
func oneLine(said string, max int) string {
	if line, _, cut := strings.Cut(said, "\n"); cut {
		said = strings.TrimSpace(line)
	}
	r := []rune(said)
	if len(r) <= max {
		return said
	}
	return strings.TrimRight(string(r[:max]), " ") + "…"
}

// levelsNow is the document's own bookmarks, in reading order, ready to be
// nested again with another one among them.
//
// ⛔ A BOOKMARK THAT POINTS NOWHERE GOES, AND EVERYTHING UNDER IT GOES WITH IT.
// The rail keeps such a row, so that the shape of a document's contents is not
// a lie about what the file holds; a tree being WRITTEN cannot keep it, because
// there is no page to write.
//
// ⛔⛔ AND KEEPING ITS CHILDREN IS WORSE THAN A REFUSAL, WHICH IS WHY THIS IS
// NOT A `continue`. Measured, by taking the two lines below out: a sub-section
// under a heading that pointed nowhere was written out under the PREVIOUS
// heading instead — depth 1 after a kept row at depth 0 is a rung ops.Nest has
// no reason to question — so the entry moved chapter silently. Deeper down the
// same fault is refused outright and the whole outline is written as nothing.
// Either way the status line says a bookmark was added.
// TestABookmarkThatGoesNowhereTakesItsChildrenWithIt is that measurement.
func (s *state) levelsNow() []ops.Level {
	marks := s.marksFor(s.src, s.docStamp)
	rows := make([]ops.Level, 0, len(marks))
	// dropped is the depth of the entry being skipped, or -1 for none: the rows
	// that follow it and sit deeper are its children.
	dropped := -1
	for _, m := range marks {
		if dropped >= 0 && m.depth > dropped {
			continue
		}
		dropped = -1
		if m.page < 1 {
			dropped = m.depth
			continue
		}
		rows = append(rows, ops.Level{Title: m.title, Page: m.page, Depth: m.depth})
	}
	return rows
}

// bookmarksFromFile replaces the table of contents with one written as text.
//
// ⛔ The picker is asked for the way Open, `merge` and `attach` ask: the press
// that reaches it is the one somebody made, which is what a browser requires
// before it will show a file chooser at all.
func (s *state) bookmarksFromFile() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	s.host.Open(func(name string, data []byte) {
		if len(data) > maxContentsBytes {
			s.fail(fmt.Sprintf("%s is %d bytes, and a table of contents is read up to %d",
				name, len(data), maxContentsBytes))
			return
		}
		marks, err := ops.ParseOutline(string(data))
		if err != nil {
			// The library names the line, which is the useful answer and reaches
			// the status line unchanged.
			s.fail(name + ": " + err.Error())
			return
		}
		if len(marks) == 0 {
			s.fail(name + " holds no bookmarks: a title, a tab and a page number " +
				"a line, two spaces of indent for each level under another")
			return
		}
		n := countMarks(marks)
		had := "bookmark"
		if n != 1 {
			had = "bookmarks"
		}
		if s.changeSaying(fmt.Sprintf("wrote %d %s from %s", n, had, name),
			func(d *ops.Doc) error {
				d.SetOutline(marks)
				return nil
			}) {
			s.refresh()
		}
	})
}

// contentsGroup is the panel beside the three bookmark verbs.
//
// ⛔ IT SAYS WHAT THE FILE LOOKS LIKE AND NOTHING ABOUT THIS DOCUMENT. A panel
// is built once and kept — what was typed into it has to still be there when a
// reader comes back to the tab — so a count of the bookmarks the document
// carries would be a number read at the moment the tab was first opened and
// drawn for ever afterwards. The rail beside the page is where that answer
// lives, and it is rebuilt on every change.
func (s *state) contentsGroup() *column {
	box := newColumn()
	box.add(toolkit.NewLabel("The rail shows the bookmarks this document carries."), bareH())
	box.add(toolkit.NewLabel("A bookmark file is one entry a line: a title, a TAB, "+
		"then the page number. Two spaces of indent for each level under another."), bareH())
	box.add(toolkit.NewLabel("It is the file `pdfops outline -from` takes."), bareH())
	return box
}

// countMarks is how many bookmarks a tree holds, its children included: what
// the status line reports, which is the number of rows that will appear in the
// rail rather than the number of top-level headings.
func countMarks(marks []ops.Bookmark) int {
	n := 0
	for _, m := range marks {
		n += 1 + countMarks(m.Children)
	}
	return n
}
