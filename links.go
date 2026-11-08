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
	//
	// ⛔ It bounds what is KEPT, not what is shown, and reading it as the second
	// was a defect: two hundred runes is something like fifteen times what fits
	// in a rail 132 logical pixels wide, so every row overflowed its pane and
	// the view came back reported as "unusable as it is". What a row DISPLAYS is
	// clipped to the pane's own width, which the reader changes by dragging the
	// divider — see contentsLabel. This number only stops one pathological page
	// of text from being held in memory a thousand times over.
	linkTitleMax = 200
)

// extractRuns is the seam a test counts through. The text of a page is the
// expensive half of building this list, and what it costs is a COUNT rather
// than a time: the machine this is measured on runs two dozen other things.
var extractRuns = extract.Runs

// linksFor is the document's links, remembered between repaints.
//
// ⛔⛔ ONCE PER DOCUMENT, NOT ONCE PER FRAME — AND THIS WAS THE LAG. marksFor
// carries the same memo for the OUTLINE, with a comment saying a repaint must
// not pay for a walk of the file. The links fallback shipped beside it with no
// memo at all, and it is the more expensive of the two: it reads the TEXT of
// every page that carries a link, where the outline walk only follows
// references. The rail is rebuilt on every page change and on every frame while
// the minipages are still filling, so a document with links on twenty pages was
// re-extracting twenty pages of text sixty times a second — which is what
// "resizing the bars is very slow, the whole tab lags" was.
//
// It is the same class as the defect goTo carries a comment about: an invariant
// about CHANGES charged to something that changes nothing.
func (s *state) linksFor(src *reader.Document, stamp [2]uint64) []bookmark {
	if s.linksDoc == stamp && s.links != nil {
		return s.links
	}
	s.linksDoc = stamp
	// Never nil once read, so a document that genuinely carries no links is
	// remembered as carrying none rather than searched again on every frame.
	s.links = append([]bookmark{}, s.linkList(src)...)
	return s.links
}

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
			runs, _ := extractRuns(src, l.Page)
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
	found := runsUnder(runs, rect)
	var b strings.Builder
	for i, r := range found {
		if i > 0 && needsSpace(found[i-1], r) {
			b.WriteByte(' ')
		}
		b.WriteString(r.Text)
	}
	return strings.TrimSpace(b.String())
}

// runsUnder is the runs overlapping a rectangle of page space, in reading
// order.
//
// ⛔ LIFTED OUT OF wordsUnder RATHER THAN COPIED BESIDE IT. Two things now ask
// which runs a rectangle covers — a link, to find the words that name it, and
// `Replace the words`, to find the operation that drew them — and a second copy
// of the overlap test would be a second set of edge decisions about a run that
// touches a rectangle by a hair. One of them would be fixed one day and the
// other not.
//
// ⛔ Down the page and then left to right, which is the order they are READ in.
// Runs arrive in the order the page DRAWS them, and a page is free to draw its
// last word first.
func runsUnder(runs []extract.Run, rect [4]float64) []extract.Run {
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
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Y != found[j].Y {
			return found[i].Y > found[j].Y
		}
		return found[i].X < found[j].X
	})
	return found
}

// needsSpace says whether two runs drawn side by side are separate words.
func needsSpace(before, after extract.Run) bool {
	if before.Y != after.Y {
		return true
	}
	gap := after.X - (before.X + before.Width)
	return gap > wordSpace(before)/2
}

// wordSpace is what a space is worth in this run's face, which is what tells a
// word break from kerning.
//
// ⛔⛔ Run.Space IS NOT ALWAYS A SPACE. On real documents it comes back as the
// font SIZE — go-pdfkit/extract#28 — and this used half of it as the threshold,
// so at 10pt a gap had to exceed 5 points to be a word break. A real space at
// that size is about 2.8 points and a gap between two letters a fraction of
// one, so EVERY gap failed and no space was ever found: every link row in the
// contents pane read as one long word, on every real document.
//
// Measured on the opening line of a real page: "Lemoteurgotex", where the gaps
// are 3.2 points between words and 0.15 between letters.
//
// go-pdfkit/latex met the same producer and says why it happens: "a composite
// font has no character at 32 and answers with whatever its fallback width is;
// one real producer answers with the font size, and taking that at face value
// turns a page into one long word."
//
// ⛔ A BELIEVABLE number is still believed. This is a fallback for a face that
// answers with something that cannot be a space, not a replacement for the
// face's own metrics: a document whose spacing is genuinely unusual is read as
// it is set, not as a third of its size.
func wordSpace(r extract.Run) float64 {
	if r.Space > r.Size*0.05 && r.Space < r.Size*0.9 {
		return r.Space
	}
	return r.Size * 0.33
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
