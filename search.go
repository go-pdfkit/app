// Finding words in the document, and lighting them where they are.
//
// ⛔⛔ A MATCH BECOMES A SELECTION, and that is the whole design. The workbench
// already knows how to light a stretch of a page's text, copy it, mark it in
// four ways and redact it; a search that drew its own highlight would be a
// second way of saying where words are — a second set of rounding decisions
// against a second layout, which is the one way a mark lands somewhere other
// than where the reader saw it. So this finds the words, turns to the page and
// drives the SAME selection a drag would have built. Everything that works on a
// selection then works on what was found, for free.
//
// ⛔ AND IT SEARCHES THE WAY THE SELECTION READS. A PDF never draws a space; it
// moves the pen, and `wordRuns` is where the gaps become word boundaries. The
// toolkit's own SelectedText joins the words of a line with ONE SPACE. So a
// phrase is looked for in exactly that text — the words of a line joined with
// one space — because anything else would find matches a reader could not then
// select, or miss ones they can see.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// textMatch is one match: the page it is on and the words of that page it
// covers.
//
// ⛔ Not called `hit`: scene.go's `hit` is the function that sends a pointer
// event to a widget, and a type of that name beside it would read as the thing
// a click lands on.
type textMatch struct {
	page     int
	from, to int // indices into the page's words, inclusive
}

// word is one word of a page in the PAGE's own space, with the runs it was
// grouped from.
//
// ⛔ In page space and not on screen, because a search covers pages that are
// not being shown — a hit on page 40 has no pixels until the viewer turns to
// it. The screen placement happens afterwards, through the same placeRun the
// text layer uses.
type word struct {
	text string
	// x, y, width and size are the box, as wordRuns computes it.
	x, y, width, size float64
	// line is which line of the page this word is on, counted in the order the
	// words came out. Words sharing a line are what a phrase may span.
	line int
}

// pageWords groups a page's glyphs into words the way the selection does.
//
// ⛔ The grouping rule is `needsSpace`, SHARED with wordRuns rather than
// repeated: a search that drew its word boundaries differently from the
// selection would find phrases that cannot be selected, which is worse than not
// finding them.
func pageWords(runs []extract.Run) []word {
	var out []word
	var group []extract.Run
	line := 0
	flush := func() {
		if len(group) == 0 {
			return
		}
		first, last := group[0], group[len(group)-1]
		text, size := "", 0.0
		for _, r := range group {
			text += r.Text
			size = max(size, r.Size)
		}
		if text != "" {
			out = append(out, word{
				text:  text,
				x:     first.X,
				y:     first.Y,
				width: last.X + last.Width - first.X,
				size:  size,
				line:  line,
			})
		}
		group = group[:0]
	}
	for j, r := range runs {
		if j > 0 {
			// A new line is a new line whatever the gap: the words of one line
			// are what a phrase may span, and a phrase that ran off the end of a
			// line and onto the next would be two stretches of text with a line
			// break between them, which no single selection covers.
			if runs[j-1].Y != r.Y {
				flush()
				line++
			} else if needsSpace(runs[j-1], r) {
				flush()
			}
		}
		group = append(group, r)
	}
	flush()
	return out
}

// findWords is every place the needle appears in these words, as word ranges.
//
// ⛔ Case-insensitively, which is what a reader means by a search, and ⛔ one
// line at a time, joined with ONE SPACE — the text the selection would give
// back for those words.
func findWords(words []word, needle string) [][2]int {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return nil
	}
	var out [][2]int
	for start := 0; start < len(words); {
		end := start
		for end+1 < len(words) && words[end+1].line == words[start].line {
			end++
		}
		// The line as the selection reads it, with each word's first character
		// recorded so a match's offset can be turned back into words.
		var line strings.Builder
		at := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			if i > start {
				line.WriteByte(' ')
			}
			at = append(at, line.Len())
			line.WriteString(strings.ToLower(words[i].text))
		}
		text := line.String()
		for from := 0; ; {
			k := strings.Index(text[from:], needle)
			if k < 0 {
				break
			}
			k += from
			out = append(out, [2]int{
				wordAt(at, k),
				wordAt(at, k+len(needle)-1),
			})
			// One character on, so "aa" in "aaa" is found twice, which is what
			// every other search does.
			from = k + 1
		}
		start = end + 1
	}
	return out
}

// wordAt is which word of the line a byte offset falls in: the last word that
// begins at or before it.
func wordAt(at []int, off int) int {
	which := 0
	for i, begins := range at {
		if begins > off {
			break
		}
		which = i
	}
	return which
}

// find looks for what was typed and shows the first match.
func (s *state) find() {
	needle := strings.TrimSpace(s.tools.find.Get())
	if needle == "" {
		s.fail("type what to look for beside it first")
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	s.hits, s.hitAt = s.hitsFor(src, needle), 0
	if len(s.hits) == 0 {
		s.found = ""
		s.note = fmt.Sprintf("nothing in this document says %q", needle)
		s.refresh()
		return
	}
	s.found = needle
	s.showHit(src)
}

// findNext moves to the match after the one being shown, wrapping round.
//
// ⛔ It re-searches when the needle has changed since the last Find, rather
// than stepping through matches of a word nobody is looking for any more. The
// box is a live binding, so what is in it and what was found can differ by the
// time this is pressed.
func (s *state) findNext() {
	needle := strings.TrimSpace(s.tools.find.Get())
	if needle == "" || needle != s.found || len(s.hits) == 0 {
		s.find()
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	s.hitAt = (s.hitAt + 1) % len(s.hits)
	s.showHit(src)
}

// hitsFor is every match in the document, in reading order.
func (s *state) hitsFor(src *reader.Document, needle string) []textMatch {
	var out []textMatch
	for page := 1; page <= src.PageCount(); page++ {
		words := pageWords(s.runsFor(src, page, s.docStamp))
		for _, at := range findWords(words, needle) {
			out = append(out, textMatch{page: page, from: at[0], to: at[1]})
		}
	}
	return out
}

// showHit turns to the match's page, lights it, and says where it is.
func (s *state) showHit(src *reader.Document) {
	at := s.hits[s.hitAt]
	s.at = at.page
	s.tools.reading.Set("") // a reading of the page is not the page
	s.note = fmt.Sprintf("%s — %d of %d, on page %d of %d",
		s.found, s.hitAt+1, len(s.hits), at.page, src.PageCount())
	s.refresh()
	// ⛔ AFTER the refresh, because the selection is in SCREEN space and the
	// page it is on has just been turned to: PageRect answers for the layout as
	// it now is, and asking before the refresh would place the match against
	// where the page used to be.
	s.lightHit(src, at)
}

// lightHit drives the selection across the words of a match.
//
// ⛔ It drives Begin/Drag/End rather than reaching into the selection, because
// a drag is the only thing the toolkit's selection knows how to be, and the
// workbench's own anti-divergence control says the painted rectangles ARE what
// a drag produced. A second way in would be a second way to be wrong.
func (s *state) lightHit(src *reader.Document, at textMatch) {
	s.sel.Clear()
	words := pageWords(s.runsFor(src, at.page, s.docStamp))
	if at.from >= len(words) || at.to >= len(words) {
		// The document changed under the hits — a verb ran between the search
		// and this press. Saying nothing is right: the page is turned to and the
		// note stands, and the next Find re-reads.
		return
	}
	s.eachVisiblePage(src, func(page int, w, h float64, rect toolkit.Rect) {
		if page != at.page {
			return
		}
		a := placeRun(words[at.from].x, words[at.from].width, words[at.from].y,
			words[at.from].size, w, h, rect, words[at.from].text)
		b := placeRun(words[at.to].x, words[at.to].width, words[at.to].y,
			words[at.to].size, w, h, rect, words[at.to].text)
		s.sel.SetRuns(s.textRuns(src, s.docStamp))
		s.sel.Begin(a.Bounds.X+1, a.Bounds.Y+a.Bounds.H/2)
		s.sel.Drag(b.Bounds.X+b.Bounds.W-1, b.Bounds.Y+b.Bounds.H/2)
		s.sel.End()
	})
}
