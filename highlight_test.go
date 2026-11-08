package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// ⛔⛔ THE CONTROL FOR THE WHOLE FILE: placeRun and quadOfRect are inverses.
// placeRun puts a run from page space onto the screen; quadOfRect puts a lit
// rectangle back into page space. If they disagreed, every mark would be
// written somewhere other than where the reader saw it lit, and nothing on the
// screen would say so — the page would look right and the saved file would be
// wrong.
//
// It is checked at three scales, because the error is in the SCALE: a round
// trip that only held at 1:1 would drift the moment a reader zoomed.
func TestAQuadIsTheRunItWasPlacedFrom(t *testing.T) {
	const pw, ph = 595.0, 842.0
	const x, y, width, size = 72.0, 700.0, 123.0, 12.0
	for _, c := range []struct {
		name string
		rect toolkit.Rect
	}{
		{"at natural size", toolkit.Rect{X: 10, Y: 20, W: 595, H: 842}},
		{"zoomed to double", toolkit.Rect{X: 0, Y: 0, W: 1190, H: 1684}},
		{"fitted to half", toolkit.Rect{X: 7, Y: 3, W: 297, H: 421}},
	} {
		t.Run(c.name, func(t *testing.T) {
			run := placeRun(x, width, y, size, pw, ph, c.rect, "hello")
			q := quadOfRect(run.Bounds, pw, ph, c.rect)
			// ⛔ The tolerance is not slack, it is the measurement: placeRun
			// truncates each edge to a whole screen pixel, so a round trip is
			// accurate to one pixel divided by the scale and no better. Two
			// truncations stack on the far edge. There is no sub-pixel position
			// to recover, because the pixels are what the reader saw.
			sx, sy := float64(c.rect.W)/pw, float64(c.rect.H)/ph
			tolX, tolY := 2/sx, 2/sy
			for _, got := range []struct {
				what      string
				got, want float64
				tol       float64
			}{
				{"left", q.TopLeftX, x, tolX},
				{"right", q.TopRightX, x + width, tolX},
				{"top", q.TopLeftY, y + size, tolY},
				{"bottom", q.BottomLeftY, y, tolY},
			} {
				if diff := got.got - got.want; diff > got.tol || diff < -got.tol {
					t.Errorf("%s edge came back %.3f, placed from %.3f (%.3f off, tolerance %.3f)",
						got.what, got.got, got.want, diff, got.tol)
				}
			}
			// A quad is upright: both top corners above both bottom corners.
			if q.TopLeftY <= q.BottomLeftY || q.TopRightY <= q.BottomRightY {
				t.Errorf("the quad is not upright: %+v", q)
			}
		})
	}
}

// ⛔⛔ AND THE MARK LANDS ON THE GLYPHS. The round trip above is arithmetic
// against itself; this renders the saved file and looks at the pixels, which is
// the only thing that answers "would a reader see the words highlighted".
//
// The fixture draws ONE line, so every dark pixel on the page belongs to the
// line the drag covered: each of them must come out changed, and the change
// must stay inside the band the mark claims. A mark that covered nothing, or
// that covered the whole page, passes neither half.
func TestHighlightingMarksTheWordsTheReaderSaw(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	before, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	selectFirstLine(t, s)

	// ⛔ The whole line, last letter and all. placeRun hands each run a font
	// that measures the run's own box (runFont); with the interface font it had
	// before, the caret could not reach the end and this read
	// "the first chapte".
	if got := s.sel.SelectedText(); got != "the first chapter" {
		t.Fatalf("the drag selected %q", got)
	}
	marks, quads := s.selectionMarks(ops.MarkHighlight, 1, 0.9, 0.15)
	if quads == 0 || len(marks) != 1 {
		t.Fatalf("the selection gave %d mark(s), %d quad(s)", len(marks), quads)
	}
	s.highlightSelection()
	// ⛔ 17 CHARACTERS, NOT 3 QUADS. The fixture sets one glyph at a time, so
	// "the first chapter" is three runs and three quadrilaterals; the first
	// draft of markSelection reported those and said "3 lines" for one line of
	// text. The count a reader can check is the one copySelection reports.
	if !strings.Contains(s.note, "highlighted 17 characters on 1 page") {
		t.Errorf("the status line says %q", s.note)
	}
	if quads == 17 {
		t.Error("the quad count and the character count cannot be told apart here")
	}
	if !s.sel.IsEmpty() {
		t.Error("the selection is still lit after it was marked")
	}

	after, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// The band the mark claims, in rendered rows: PDF counts up from the foot
	// of the page, a raster counts down from the top.
	const pageH = 400.0 // the fixture's MediaBox
	q := marks[0].Quads[0]
	top, bottom := int(pageH-q.TopLeftY)-2, int(pageH-q.BottomLeftY)+2

	was, now := page1(t, before), page1(t, after)
	if was.W != now.W || was.H != now.H {
		t.Fatalf("the page changed size: %dx%d then %dx%d", was.W, was.H, now.W, now.H)
	}
	baseline := int(pageH - q.BottomLeftY)
	ink, inkChanged, changedOutside, missedAbove := 0, 0, 0, 0
	var where []string
	for row := range now.H {
		for col := range now.W {
			i := (row*now.W + col) * 4
			dark := was.Pix[i] < 128 && was.Pix[i+1] < 128 && was.Pix[i+2] < 128
			changed := was.Pix[i] != now.Pix[i] ||
				was.Pix[i+1] != now.Pix[i+1] || was.Pix[i+2] != now.Pix[i+2]
			if dark {
				ink++
				switch {
				case changed:
					inkChanged++
				case row < baseline:
					missedAbove++
					if len(where) < 8 {
						where = append(where, fmt.Sprintf("(%d,%d)", col, row))
					}
				}
			}
			if changed && (row < top || row > bottom) {
				changedOutside++
			}
		}
	}
	if ink == 0 {
		t.Fatal("the page carries no ink, so this measures nothing")
	}
	// ⛔⛔ NOT every ink pixel, and the exception is the measurement. A run's box
	// is the baseline and the font size ABOVE it — placeRun says so — so the
	// descender of a p or a y falls below it and is not lit on screen either.
	// The mark is what is lit, so the mark misses them too, and a test demanding
	// 100% here would be demanding that the annotation and the highlight
	// disagree. What must hold is that nothing ABOVE the baseline is missed:
	// that is where a mis-scaled or mis-flipped quad would show, and it is the
	// 93% this measured. Covering descenders means widening the run box in
	// placeRun, which changes what the reader sees first.
	if missedAbove != 0 {
		t.Errorf("%d ink pixels above the baseline (row %d) are untouched at %s: "+
			"the mark does not cover the words", missedAbove, baseline,
			strings.Join(where, " "))
	}
	if inkChanged*10 < ink*9 {
		t.Errorf("the mark covers %d of %d ink pixels, under the 90%% the "+
			"descenders alone account for", inkChanged, ink)
	}
	if changedOutside != 0 {
		t.Errorf("%d pixels changed outside rows %d..%d, which the mark claims",
			changedOutside, top, bottom)
	}
	// ⛔ And the words are STILL READABLE under it. ops/markup.go found this out
	// the hard way: render has no blend modes, so a /Multiply highlight came out
	// as an opaque block over the text. The ink must still be the darker thing.
	lit, dim := 0, 0
	for row := top + 1; row < bottom; row++ {
		for col := range now.W {
			i := (row*now.W + col) * 4
			if was.Pix[i] < 128 {
				dim++
				if now.Pix[i] < 200 {
					lit++
				}
			}
		}
	}
	if dim == 0 || lit*2 < dim {
		t.Errorf("only %d of %d inked pixels are still dark under the mark", lit, dim)
	}
}

// selectFirstLine drags along the first line of text on page one.
func selectFirstLine(t *testing.T, s *state) {
	t.Helper()
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) == 0 {
		t.Fatal("the page offers no text to select")
	}
	first := runs[0].Bounds
	right := first.X + first.W
	for _, r := range runs {
		if r.Bounds.Y == first.Y {
			right = max(right, r.Bounds.X+r.Bounds.W)
		}
	}
	y := first.Y + first.H/2
	if !s.beginSelect(first.X, y) {
		t.Fatal("a press on the page did not begin a selection")
	}
	s.dragSelect(right, y)
	s.endSelect(right, y)
}

// page1 renders the first page of a document, so a claim about a mark can be
// made about pixels rather than about the dictionary that asked for them.
func page1(t *testing.T, b []byte) *raster.Image {
	t.Helper()
	src, err := reader.Open(b)
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.Page(src, 1, render.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Underline, strike out and squiggle are the same path with another subtype and
// another colour, and a reader reaches for all four.
//
// ⛔ EACH CASE ASSERTS ITS OWN SUBTYPE. Checking only that the file carries
// /QuadPoints would pass for all four however the kind was wired — the four
// would be indistinguishable, and a squiggle written as a highlight would look
// like a working verb in every test here. The subtype is the only thing in the
// file that says which mark it is (ISO 32000-1, Table 179).
func TestTheFourMarksSayWhatTheyDidAndWriteTheirOwnSubtype(t *testing.T) {
	for _, c := range []struct {
		name    string
		run     func(*state)
		says    string
		subtype string
	}{
		{"underline", (*state).underlineSelection, "underlined 17 characters on 1 page", "/Underline"},
		{"strike out", (*state).strikeSelection, "struck out 17 characters on 1 page", "/StrikeOut"},
		{"squiggle", (*state).squiggleSelection, "squiggled under 17 characters on 1 page", "/Squiggly"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, perGlyphPDF(t))
			selectFirstLine(t, s)
			c.run(s)
			if !strings.Contains(s.note, c.says) {
				t.Errorf("the status line says %q, not %q", s.note, c.says)
			}
			out, err := s.doc.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "/QuadPoints") {
				t.Error("the saved document carries no QuadPoints")
			}
			if !strings.Contains(string(out), c.subtype) {
				t.Errorf("the saved document carries no %s annotation", c.subtype)
			}
		})
	}
}

// ⛔ A VERB THAT DID NOTHING SAYS SO. Pressing Highlight with nothing selected
// is the commonest way to press it, because the ribbon is where the eye goes
// first; a silent no-op is a workbench that looks broken.
func TestMarkingNothingSaysNothingIsSelected(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	s.highlightSelection()
	if !strings.Contains(s.note, "nothing is selected") {
		t.Errorf("the status line says %q", s.note)
	}
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "/QuadPoints") {
		t.Error("a document was marked although nothing was selected")
	}
}

// ⛔ ONE ANNOTATION PER PAGE, and the pages it does not reach get none. A
// selection dragged from one page onto the next is two marks in the file,
// because QuadPoints are in a page's own user space — one annotation cannot
// span two pages (ISO 32000-1 §12.5.6.10).
func TestASelectionAcrossTwoPagesMarksBoth(t *testing.T) {
	// ⛔ CONTINUOUS, because the workbench opens PAGINATED — one page centred,
	// fitted — and in that mode a selection cannot reach a second page at all.
	// Continuous is the reader's own choice from the viewer's toolbar, and it is
	// the only mode in which a drag can cross a page boundary; a test left in
	// the default mode would have proved that one page is marked and called it
	// proof about two.
	s := shown(t, twoLinePagesPDF(t))
	s.pv.Mode().Set(toolkit.PagedContinuous)
	s.draw(buffer())
	if _, clip, ok := s.pv.PageRect(2); !ok || clip.W <= 0 || clip.H <= 0 {
		t.Fatal("the second page is not on screen, so this measures nothing")
	}
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) < 2 {
		t.Fatalf("the two pages offer %d runs", len(runs))
	}
	first, last := runs[0].Bounds, runs[len(runs)-1].Bounds
	if first.Y == last.Y {
		t.Fatal("both lines are at the same height, so the drag covers one page")
	}

	// A drag that covers only the first page leaves the second alone, even
	// though the second is on screen.
	if !s.beginSelect(first.X, first.Y+first.H/2) {
		t.Fatal("a press on the first page did not begin a selection")
	}
	s.dragSelect(first.X+first.W, first.Y+first.H/2)
	s.endSelect(first.X+first.W, first.Y+first.H/2)
	marks, quads := s.selectionMarks(ops.MarkHighlight, 1, 0.9, 0.15)
	if len(marks) != 1 || marks[0].Page != 1 || quads != 1 {
		t.Fatalf("a drag on page one gave %d mark(s) on %v, %d quad(s)",
			len(marks), pagesOf(marks), quads)
	}

	// A drag that reaches the second page marks both.
	s.sel.Clear()
	if !s.beginSelect(first.X, first.Y+first.H/2) {
		t.Fatal("a press on the first page did not begin a selection")
	}
	s.dragSelect(last.X+last.W, last.Y+last.H/2)
	s.endSelect(last.X+last.W, last.Y+last.H/2)
	marks, quads = s.selectionMarks(ops.MarkHighlight, 1, 0.9, 0.15)
	if len(marks) != 2 || quads != 2 {
		t.Fatalf("a drag across both pages gave %d mark(s) on %v, %d quad(s)",
			len(marks), pagesOf(marks), quads)
	}
	if marks[0].Page != 1 || marks[1].Page != 2 {
		t.Errorf("the marks landed on %v, not pages 1 and 2", pagesOf(marks))
	}
	s.highlightSelection()
	if !strings.Contains(s.note, "on 2 pages") {
		t.Errorf("the status line says %q", s.note)
	}
}

func pagesOf(marks []ops.Markup) []int {
	out := make([]int, 0, len(marks))
	for _, m := range marks {
		out = append(out, m.Page)
	}
	return out
}

// ⛔ THE CENTRE, NOT A CORNER. placeRun rounds a run's width and height UP to at
// least one pixel, so a rectangle on the last line of a page can end a pixel
// past its edge. A corner test would hand that line to the next page — a mark
// on the wrong sheet — and a containment test would drop it in silence.
func TestAPageClaimsALineByItsCentre(t *testing.T) {
	// The page occupies rows 200..599; the next card starts at 610.
	page := toolkit.Rect{X: 100, Y: 200, W: 300, H: 400}
	next := toolkit.Rect{X: 100, Y: 610, W: 300, H: 400}
	// A line on the last baseline, whose box ends four pixels past the foot of
	// the page but whose centre is still on it.
	overhang := toolkit.Rect{X: 110, Y: 595, W: 80, H: 8}
	if !inPage(overhang, page) {
		t.Error("a line overhanging the foot of its page was disowned by it")
	}
	if inPage(overhang, next) {
		t.Error("a line on one page was claimed by the next one as well")
	}
	// And a line on the next page is not claimed by this one.
	if inPage(toolkit.Rect{X: 110, Y: 620, W: 80, H: 8}, page) {
		t.Error("a page claimed a line that is on the one below it")
	}
	// Nor is a line to the side of it, which is what a two-up layout has.
	if inPage(toolkit.Rect{X: 450, Y: 300, W: 80, H: 8}, page) {
		t.Error("a page claimed a line beside it")
	}
}

// A document whose pages declare no size offers nothing to mark, and marking
// nothing must not be the same act as marking everything.
func TestADegeneratePageIsNotMarked(t *testing.T) {
	s := shown(t, degeneratePDF(t))
	s.sel.SetRuns([]toolkit.TextRun{
		{Text: "ghost", Bounds: toolkit.Rect{X: 0, Y: 0, W: 50, H: 10}, Font: toolkit.CurrentFont()},
	})
	s.sel.Begin(0, 5)
	s.sel.Drag(50, 5)
	s.sel.End()
	if s.sel.IsEmpty() {
		t.Fatal("the fixture selection is empty, so this measures nothing")
	}
	if marks, quads := s.selectionMarks(ops.MarkHighlight, 1, 1, 0); len(marks) != 0 || quads != 0 {
		t.Errorf("a page with no size was given %d mark(s), %d quad(s)", len(marks), quads)
	}
}

// twoLinePagesPDF is two pages, each one line of per-glyph text, WIDE AND SHORT
// so that both cards fit on the test surface at once — which is what makes a
// drag from one page to the next, and a page the selection does not reach,
// reachable at all.
func twoLinePagesPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	font := w.Add(reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	})
	kids := reader.Array{}
	for _, word := range []string{"first", "second"} {
		var body strings.Builder
		x := 20.0
		for _, ch := range word {
			fmt.Fprintf(&body, "BT /F1 10 Tf %.2f 30 Td (%c) Tj ET\n", x, ch)
			x += 5
		}
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body.String())})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": kids, "Count": reader.Integer(len(kids)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(600), reader.Integer(80)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
