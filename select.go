// Selecting the text on a page.
//
// ⛔⛔ THE PAGE IS A PICTURE, which is why this file has to exist. PagedView
// says so itself: "Content is a []*image.RGBA of page bitmaps at natural size
// … PagedView never knows how they were rasterised." Nothing in the widget
// tree is text, so there is nothing for a pointer to select — the reader drags
// across a paragraph and the workbench sees a drag across a bitmap.
//
// What this adds is the layer that is missing: the page's own glyphs, read with
// go-pdfkit/extract, placed where they are DRAWN, and handed to the toolkit's
// selection. The pixels on screen do not change.
//
// ⛔ AND RENDERING THE PAGE AS SVG WOULD NOT REMOVE THE NEED FOR IT. That is
// worth writing down, because the go-tex playground deleted its own text layer
// when it moved from bitmaps to SVG and the same move is planned here. It could:
// it typesets the document itself, so its SVG carries real <text>. A PDF
// renderer cannot: go-pdfkit/render turns every glyph into an OUTLINE before it
// paints — opentype.Segments into a vector.Path, filled as coverage (text.go,
// paintGlyph) — so an SVG written from it holds <path> elements and no text at
// all. Crisp at any zoom, still not selectable.

package main

import (
	"fmt"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// runsFor is one page's glyphs, remembered for as long as the document is.
//
// ⛔ Once per PAGE per document. Reading the text of a page is the expensive
// half of this, and the screen placement below is recomputed on every frame a
// selection is live — so what must not be repeated is the reading, not the
// arithmetic. The same memo the outline and the links carry, for the same
// reason.
func (s *state) runsFor(src *reader.Document, page int, stamp [2]uint64) []extract.Run {
	if s.runsDoc != stamp || s.runs == nil {
		s.runsDoc, s.runs = stamp, map[int][]extract.Run{}
	}
	if got, read := s.runs[page]; read {
		return got
	}
	runs, _ := extractRuns(src, page)
	// Never nil once read, so a page that genuinely carries no text is
	// remembered as carrying none rather than read again on every frame.
	if runs == nil {
		runs = []extract.Run{}
	}
	s.runs[page] = runs
	return runs
}

// textRuns is the text of every page on screen, placed where it is drawn.
//
// ⛔ The placement comes from PagedView.PageRect — the same layout the viewer
// renders its bitmaps from (see drawVisible) — so the text cannot drift from
// the picture it sits on through a scroll, a zoom, a page flip or a resize.
func (s *state) textRuns(src *reader.Document, stamp [2]uint64) []toolkit.TextRun {
	var out []toolkit.TextRun
	s.eachVisiblePage(src, func(i int, w, h float64, rect toolkit.Rect) {
		out = append(out, wordRuns(s.runsFor(src, i, stamp), w, h, rect)...)
	})
	return out
}

// eachVisiblePage calls f for every page the viewer is showing, with the page's
// size in points and the pixels it occupies.
//
// ⛔ ONE PAGE WALK, because there are now two things that need it: the text
// layer places runs onto the pages, and the Mark verbs put a selection back
// into page space (highlight.go). Both need the same pages, the same rectangles
// and the same four guards, and a second copy of them would be a second set of
// rounding and fallback decisions — the text would be placed against one layout
// and the annotation written against another, which is the one way a highlight
// can land somewhere other than where the reader saw it lit.
func (s *state) eachVisiblePage(src *reader.Document, f func(page int, w, h float64, rect toolkit.Rect)) {
	if src == nil || s.pv == nil {
		return
	}
	for i := 1; i <= src.PageCount(); i++ {
		rect, clip, ok := s.pv.PageRect(i)
		if !ok || clip.W <= 0 || clip.H <= 0 || rect.W <= 0 || rect.H <= 0 {
			continue // not on screen, or scaled away
		}
		// ⛔ The error is dropped rather than branched on, which is what
		// drawVisible does two files away and for the same reason: pageSize
		// falls back on a real paper size for a page it cannot read, so a page
		// this document cannot resolve gets a size and no text rather than a
		// second way of saying the same thing. A page that declares a size of
		// NOTHING is the one case worth stepping over, because a scale derived
		// from it would not be a number.
		page, _ := src.Page(i)
		w, h := pageSize(src, page)
		if w <= 0 || h <= 0 {
			continue
		}
		f(i, w, h, rect)
	}
}

// wordRuns groups a page's glyphs into one run per WORD, placed on screen.
//
// ⛔⛔ ONE RUN PER WORD, NOT PER GLYPH, AND THE DIFFERENCE IS THE WHOLE POINT.
// A PDF sets glyphs one at a time — the page this was measured on has 152 runs
// for four lines — and the toolkit's selection joins the runs it covers WITH A
// SPACE. Handing it the glyphs gives a reader who selects a line and copies it
//
//	t h e f i r s t c h a p t e r
//
// which is what this did until it was measured. Grouped into words, the same
// gesture gives "the first chapter".
//
// ⛔ And the grouping is where the spaces come from. A PDF never draws a space;
// it moves the pen. So a word ends where the gap to the next glyph is wider
// than half a space in that face — see needsSpace, which has its own reason to
// be careful about what a face calls a space.
func wordRuns(runs []extract.Run, pw, ph float64, rect toolkit.Rect) []toolkit.TextRun {
	var out []toolkit.TextRun
	var word []extract.Run
	flush := func() {
		if len(word) == 0 {
			return
		}
		first, last := word[0], word[len(word)-1]
		text, size := "", 0.0
		for _, r := range word {
			text += r.Text
			size = max(size, r.Size)
		}
		if text != "" {
			out = append(out, placeRun(first.X, last.X+last.Width-first.X,
				first.Y, size, pw, ph, rect, text))
		}
		word = word[:0]
	}
	for j, r := range runs {
		if j > 0 && needsSpace(runs[j-1], r) {
			flush()
		}
		word = append(word, r)
	}
	flush()
	return out
}

// placeRun maps one run from the page's own space to the surface's pixels.
//
// PDF counts upward from the foot of the page and a screen counts downward
// from the top, so the baseline is flipped; the run's box is the baseline and
// the size above it, which is the same box wordsUnder tests a link rectangle
// against.
func placeRun(x, width, y, size, pw, ph float64, rect toolkit.Rect, text string) toolkit.TextRun {
	sx := float64(rect.W) / pw
	sy := float64(rect.H) / ph
	box := toolkit.Rect{
		X: rect.X + int(x*sx),
		Y: rect.Y + int((ph-y-size)*sy),
		W: max(int(width*sx), 1),
		H: max(int(size*sy), 1),
	}
	return toolkit.TextRun{Text: text, Bounds: box, Font: runFont{
		w: box.W, h: box.H, n: len([]rune(text)),
	}}
}

// runFont measures a run against the run's OWN box.
//
// ⛔⛔ THE INTERFACE FONT CANNOT MEASURE A PDF'S GLYPHS, and handing it the job
// cost the last character of every line. This used to be toolkit.CurrentFont().
// A TextSelection subdivides a run with the run's Font, while the run's BOX
// comes from where the glyphs actually sit on the page — so the two metrics
// disagreed, and on perGlyphPDF, which is what a real producer writes:
//
//   - dragging to the right-hand end of a line selected "the first chapte".
//     The interface font measures "chapter" wider than the glyphs do, so the
//     caret at the box's right edge landed at the sixth character and the
//     seventh was NOT REACHABLE — copying a line lost its last letter.
//   - the word "first", wholly inside the selection, was lit 6 points short of
//     its own box, because the lit width is the font's measurement of the text
//     and not the box: the final t stood outside the highlight.
//
// A run is one word at one size, so measuring n characters as n/len of the box
// makes prefixWidth(the whole text) == Bounds.W BY CONSTRUCTION. The caret can
// then reach both ends, and a covered run is lit exactly edge to edge. Within a
// word it assumes even spacing, so a caret can sit a fraction of a glyph away
// from the ink in a proportional face — bounded by one glyph, where the old
// error was unbounded and grew with the length of the run.
type runFont struct {
	w, h, n int
}

func (f runFont) Height() int { return f.h }

func (f runFont) Advance() int { return f.w / max(f.n, 1) }

// ⛔ max(f.n, 1) rather than a guard: wordRuns never emits a run with no text,
// so a zero n is unreachable and a branch for it would be a statement no test
// can enter. Dividing by one gives a length rather than a panic, which is the
// same protection without the dead line.
func (f runFont) Measure(s string) int {
	return len([]rune(s)) * f.w / max(f.n, 1)
}

// Draw is never called: these runs are a layer over a page the renderer has
// already drawn, and nothing in the toolkit paints a selection's text.
func (f runFont) Draw(_ painter.Painter, _, _ int, _ string, _ toolkit.RGBA) {}

// beginSelect takes a press that lands on a page, and says whether it did.
//
// ⛔ ONLY ON A PAGE. A press anywhere else — the scrollbar down the pane's
// edge, the gap between two cards, the rail, the ribbon — goes to the widget
// under it as before. PageAt answers that question for the same layout the
// pages are drawn from, so there is no second idea of where a page is.
func (s *state) beginSelect(x, y int) bool {
	if s.shown != s.pv || s.src == nil {
		return false
	}
	// ⛔⛔ INSIDE THE VIEWER'S OWN BOX FIRST. PageAt maps a point to a page
	// through the pane's scroll and does NOT refuse a point outside the pane:
	// asked about a spot on the ribbon it answers with a page, so every press
	// on a ribbon control became a selection and no verb ever ran. Measured —
	// with this guard taken out, "no press on a ribbon control reached a verb"
	// and eleven tests fall over.
	if r := s.pv.Bounds(); x < r.X || y < r.Y || x >= r.X+r.W || y >= r.Y+r.H {
		return false
	}
	if _, _, _, ok := s.pv.PageAt(x, y); !ok {
		return false
	}
	s.sel.SetRuns(s.textRuns(s.src, s.docStamp))
	s.sel.Begin(x, y)
	s.selecting = true
	s.dirty = true
	return true
}

// dragSelect extends a selection in progress.
func (s *state) dragSelect(x, y int) bool {
	if !s.selecting {
		return false
	}
	s.sel.Drag(x, y)
	s.dirty = true
	return true
}

// endSelect finishes the gesture.
func (s *state) endSelect(x, y int) bool {
	if !s.selecting {
		return false
	}
	s.sel.Drag(x, y)
	s.sel.End()
	s.selecting = false
	s.dirty = true
	return true
}

// drawSelection paints the highlight over the page.
//
// ⛔ The runs are placed again first, every frame the selection is up. The
// reader can scroll or zoom with text selected, and a highlight drawn where
// the words USED to be is worse than none: it says the wrong words are chosen.
func (s *state) drawSelection(p painter.Painter) {
	if s.sel.IsEmpty() || s.src == nil {
		return
	}
	s.sel.SetRuns(s.textRuns(s.src, s.docStamp))
	s.sel.Draw(p, s.theme.Accent)
}

// copySelection puts the selected text on the clipboard.
//
// ⛔ It says HOW MUCH it copied. A clipboard is somewhere else: the one thing
// a reader cannot do is look at it to find out whether the gesture worked, and
// a verb that did its job in silence is one they will press twice.
func (s *state) copySelection() {
	text := s.sel.SelectedText()
	if text == "" {
		s.note = "nothing is selected: drag across the words on a page first"
		s.status = toolkit.NewStatusbar(s.statusLine())
		s.dirty = true
		return
	}
	toolkit.SetClipboardText(text)
	s.note = fmt.Sprintf("%d characters copied", len([]rune(text)))
	s.status = toolkit.NewStatusbar(s.statusLine())
	s.dirty = true
}

// clearSelection drops the selection, which is what Escape does.
func (s *state) clearSelection() bool {
	if s.sel.IsEmpty() {
		return false
	}
	s.sel.Clear()
	s.dirty = true
	return true
}
