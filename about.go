// What the file says about itself, how big its pages are, and whether it had
// to be repaired to be read at all.
//
// ⛔⛔ THE PANEL COULD WRITE AND COULD NOT READ. Giving a document a title and
// an author has been on the File tab since the beginning; showing the title it
// already has was never there. So a reader who wanted to know what a file
// claims about itself — who produced it, when, what it is called — had to open
// it in something else, and anybody typing into those two boxes was overwriting
// a value they could not see. That was measured against BentoPDF's own tables
// and recorded as its own row: "the workbench's Metadata panel WRITES a title
// and an author and never shows what is there".
//
// It answers three of BentoPDF's tools at once, because they are one question:
// View Metadata, Page Dimensions, and Repair PDF — the last because repair is
// something a PDF reader does on the way IN, so the honest verb is to say
// whether this file needed it and that saving writes a sound one.

package main

import (
	"fmt"
	"sort"

	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// readingAbout is the reading that shows the file rather than the page.
const readingAbout = "about"

// infoKeys are the entries of a document information dictionary, in the order
// ISO 32000-1 Table 317 lists them rather than alphabetically: a person looking
// for the producer should find it where every other tool puts it.
var infoKeys = []reader.Name{
	"Title", "Author", "Subject", "Keywords",
	"Creator", "Producer", "CreationDate", "ModDate", "Trapped",
}

// readAbout shows what the file says about itself, or puts the page back.
func (s *state) readAbout() { s.read(readingAbout) }

// aboutView is the file read rather than the page.
func (s *state) aboutView(src *reader.Document) toolkit.Widget {
	col := newColumn()
	col.add(toolkit.NewLabel("What it says about itself"), lineH)
	for _, row := range s.infoRows() {
		col.add(toolkit.NewLabel("    "+row), lineH)
	}
	col.add(toolkit.NewLabel(""), lineH)
	col.add(toolkit.NewLabel("How big the pages are"), lineH)
	for _, row := range s.pageRows(src) {
		col.add(toolkit.NewLabel("    "+row), lineH)
	}
	col.add(toolkit.NewLabel(""), lineH)
	col.add(toolkit.NewLabel(s.repairLine()), lineH)
	return col.scrollerOf(s.pageW() - 2*margin)
}

// infoRows is every entry the information dictionary carries.
//
// ⛔ IT LISTS WHAT IS THERE, including the keys this does not know. A producer
// is free to add its own, and a readout that showed only the nine standard ones
// would quietly hide them — which is the opposite of the point of a readout.
// The known ones come first, in the order the standard lists them; anything
// else follows, sorted, so the order is stable between two openings of the same
// file.
func (s *state) infoRows() []string {
	if s.doc == nil {
		return []string{"nothing is open"}
	}
	info := s.doc.Info()
	var rows []string
	seen := map[reader.Name]bool{}
	for _, key := range infoKeys {
		seen[key] = true
		if v := s.infoSays(key); v != "" {
			rows = append(rows, string(key)+": "+v)
		}
	}
	var extra []string
	for key := range info {
		if seen[key] {
			continue
		}
		if v := s.infoSays(key); v != "" {
			extra = append(extra, string(key)+": "+v)
		}
	}
	sort.Strings(extra)
	rows = append(rows, extra...)
	if len(rows) == 0 {
		return []string{"it says nothing about itself"}
	}
	return rows
}

// pageRows is every page's size in points and how it is turned.
//
// ⛔ The size is the one the page will be SHOWN at — pageSize prefers the crop
// box over the media box, because a page cropped to half its paper is half its
// paper to everybody who looks at it. A readout quoting the media box of a
// cropped page would disagree with the picture beside it.
func (s *state) pageRows(src *reader.Document) []string {
	if src == nil || s.doc == nil {
		return []string{"nothing is open"}
	}
	rows := make([]string, 0, src.PageCount())
	for i := 1; i <= src.PageCount(); i++ {
		page, _ := src.Page(i)
		w, h := pageSize(src, page)
		row := fmt.Sprintf("page %d: %.0f × %.0f points", i, w, h)
		if turned, err := s.doc.Rotation(i); err == nil && turned != 0 {
			row += fmt.Sprintf(", turned %d°", turned)
		}
		rows = append(rows, row)
	}
	return rows
}

// repairLine says whether the file had to be rebuilt to be read.
//
// ⛔ AND WHAT TO DO ABOUT IT. "This file was repaired" on its own is alarming
// and useless; what a reader needs to know is that Save writes the rebuilt
// document, so the repair is one keystroke away rather than a thing they must
// find another tool for.
func (s *state) repairLine() string {
	if s.doc == nil {
		return "nothing is open"
	}
	if s.doc.Repaired() {
		return "⚠ this file's structure was damaged and had to be rebuilt to be read. " +
			"Saving writes the rebuilt document."
	}
	return "Its structure was sound: nothing had to be rebuilt to read it."
}
