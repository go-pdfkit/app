// The links a document carries, as rows of the contents pane.
//
// ⛔⛔ THIS IS WHAT THE CONTENTS VIEW FALLS BACK TO. It shipped falling back to
// a list of pages, and the person who asked for the view said, on seeing it,
// that what they meant was a display of the document's links. They were right,
// and the difference is not presentational: a link is structure the AUTHOR
// wrote — this phrase leads there — while a list of pages is arithmetic that
// reads the same for every file of the same length.
//
// What a row SAYS is the words the link sits on, which is the only name a link
// has. A PDF link is a rectangle and a destination; nothing in it is a title.
// So the words under the rectangle are read off the page, and only when there
// are none does the row fall back to naming where it goes.

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// The bounds on what a file may ask this to do.
//
// ⛔ A document chooses how many links it carries, and reading the words under
// each one means reading the text of the page it is on. Both are the file's to
// make enormous, so both are bounded here, the same way the outline walk is —
// and the bound is reported in the last row rather than hidden, because a list
// that silently stops is a list that lies about what the document holds.
const (
	maxLinkRows = 1000
	// linkTitleMax is counted in RUNES, the ellipsis included.
	linkTitleMax = 200
)

// linkList is one row per link the document carries, in page order.
func (s *state) linkList(src *reader.Document) []bookmark {
	if s.doc == nil || src == nil {
		return nil
	}
	links := s.doc.Links()
	if len(links) == 0 {
		return nil
	}

	// ⛔ The text of a page is read ONCE however many links sit on it, and only
	// for the pages that carry one. A document with a link on every page of a
	// thousand still reads a thousand pages, which is why maxLinkRows stops
	// before that; a document with four links on one page reads one page.
	words := map[int][]extract.Run{}
	out := make([]bookmark, 0, len(links))
	more := false
	for _, l := range links {
		if len(out) >= maxLinkRows {
			more = true
			break
		}
		if _, read := words[l.Page]; !read {
			runs, _ := extract.Runs(src, l.Page)
			words[l.Page] = runs
		}
		out = append(out, bookmark{
			title: linkTitle(words[l.Page], l),
			page:  l.Target,
			uri:   l.URI,
		})
	}
	if more {
		out = append(out, bookmark{
			title: fmt.Sprintf("… and %d more links, not shown", len(links)-len(out)),
		})
	}
	return out
}

// linkTitle is what a link's row says: the words it sits on, or where it goes
// when it sits on none.
//
// ⛔ A link with no words under it is still a row. An image that is a link, a
// link over a blank area of a scan, a link whose text this cannot read: all of
// them exist, and a row naming the destination is more use than no row at all —
// the reader at least learns the document points there.
func linkTitle(runs []extract.Run, l ops.Link) string {
	if words := wordsUnder(runs, l.Rect); words != "" {
		return clipTitle(words)
	}
	switch {
	case l.URI != "":
		return clipTitle(l.URI)
	case l.Target != 0:
		return fmt.Sprintf("Page %d", l.Target)
	}
	return "(a link that leads nowhere)"
}

// wordsUnder joins the text whose runs overlap a rectangle, left to right.
//
// A run's own box is its baseline and the size above it, and a link rectangle
// is normally drawn a little below the baseline and a little above the cap
// height, so the test is an OVERLAP of the two boxes rather than containment: a
// link whose rectangle is one point tighter than the text it underlines must
// still find its words.
func wordsUnder(runs []extract.Run, rect [4]float64) string {
	var found []extract.Run
	for _, r := range runs {
		if r.Text == "" {
			continue
		}
		x0, x1 := r.X, r.X+r.Width
		y0, y1 := r.Y, r.Y+r.Size
		if x1 <= rect[0] || x0 >= rect[2] || y1 <= rect[1] || y0 >= rect[3] {
			continue
		}
		found = append(found, r)
	}
	// Down the page and then left to right, which is the order they are read
	// in. Runs arrive in the order the page DRAWS them, and a page is free to
	// draw its last word first.
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Y != found[j].Y {
			return found[i].Y > found[j].Y
		}
		return found[i].X < found[j].X
	})
	var b strings.Builder
	for i, r := range found {
		if i > 0 && needsSpace(found[i-1], r) {
			b.WriteByte(' ')
		}
		b.WriteString(r.Text)
	}
	return strings.TrimSpace(b.String())
}

// needsSpace says whether two runs drawn side by side are separate words. The
// run carries what a space would be worth here, which is what tells a word
// break from kerning.
func needsSpace(before, after extract.Run) bool {
	if before.Y != after.Y {
		return true
	}
	gap := after.X - (before.X + before.Width)
	return gap > before.Space/2
}

// clipTitle keeps a row to a line, counting RUNES rather than bytes so that a
// title cut in the middle of a character does not come back as a question mark.
func clipTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= linkTitleMax {
		return s
	}
	return string(r[:linkTitleMax-len([]rune(outlineEllipsis))]) + outlineEllipsis
}
