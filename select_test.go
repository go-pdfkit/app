// Selecting the text on a page.

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// shown opens a document and draws it once, which is what gives the pages
// their place on screen — nothing can be selected before there is a layout.
func shown(t *testing.T, b []byte) *state {
	t.Helper()
	s := openedWith(t, b)
	s.draw(buffer())
	return s
}

// onPage is a point inside the first page's card, and whether there is one.
func onPage(s *state) (int, int, bool) {
	rect, clip, ok := s.pv.PageRect(1)
	if !ok || clip.W <= 0 || clip.H <= 0 {
		return 0, 0, false
	}
	return rect.X + rect.W/2, rect.Y + rect.H/2, true
}

// ⛔⛔ THE WHOLE POINT: THE WORDS ON A PAGE CAN BE SELECTED. The page is a
// bitmap, so before this there was nothing under the pointer but pixels.
func TestDraggingAcrossAPageSelectsItsWords(t *testing.T) {
	s := shown(t, linkedPDF(t))
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) == 0 {
		t.Fatal("the page offers no text to select")
	}

	// Drag along the first line, from its left edge to its right.
	first := runs[0].Bounds
	var right int
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

	got := s.sel.SelectedText()
	if !strings.Contains(got, "the first chapter") {
		t.Errorf("dragging along the first line selected %q", got)
	}
}

// ⛔⛔ AND THE WORDS COME OUT WITH SPACES BETWEEN THEM. A PDF never draws a
// space: it moves the pen. So the glyphs arrive touching, and text copied from
// a selection would read "thefirstchapter" unless the gaps are put back as
// runs of their own — which is also what lets a reader select the space
// between two words, as they can in any other text.
func TestTheSelectedTextHasItsSpaces(t *testing.T) {
	// ⛔ A PER-GLYPH PAGE, because that is what a real one is. The first
	// version of this test used a fixture that draws a whole line with one Tj,
	// so extract returned ONE run with the spaces already inside it and the
	// code being tested never ran. A real producer sets each glyph at its own
	// position — go-tex does, and the page this was measured on has 152 runs
	// for four lines — which is exactly the case where the spaces are gone.
	s := shown(t, perGlyphPDF(t))
	runs := s.textRuns(s.src, s.docStamp)

	// ⛔ THREE RUNS, ONE PER WORD. The toolkit joins the runs a selection
	// covers WITH A SPACE, so the granularity of a run IS the spacing of the
	// copied text: one run per glyph gives "t h e f i r s t c h a p t e r",
	// which is what this did until it was measured.
	var said []string
	for _, r := range runs {
		said = append(said, r.Text)
	}
	if len(said) != 3 {
		t.Fatalf("the page came back as %d runs: %q", len(said), said)
	}
	if strings.Join(said, " ") != "the first chapter" {
		t.Errorf("the page's runs read %q", said)
	}
	// And each word's box spans the whole word, not its first letter: that is
	// what a drag has to cover.
	for i, r := range runs {
		if r.Bounds.W < len(said[i])*2 {
			t.Errorf("the word %q is %d pixels wide", said[i], r.Bounds.W)
		}
	}
}

// perGlyphPDF sets one line with every glyph placed on its own, the way a
// typesetter does: 5pt characters, and 3.3pt between words.
func perGlyphPDF(t *testing.T) []byte {
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
	var body strings.Builder
	x := 20.0
	for _, ch := range "the first chapter" {
		if ch == ' ' {
			x += 3.3 // the pen moves; nothing is drawn
			continue
		}
		fmt.Fprintf(&body, "BT /F1 10 Tf %.2f 300 Td (%c) Tj ET\n", x, ch)
		x += 5
	}
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body.String())})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
		"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔ THE PROMISE runFont MAKES: the whole of a run's text measures the whole of
// the run's box. Everything the selection does inside a run — where the caret
// lands, how wide a covered run is lit, and so where a mark is written — is
// prefixWidth against this font, so if it did not measure the box the box would
// be the one thing the arithmetic could not reach.
func TestARunsFontMeasuresTheRunsOwnBox(t *testing.T) {
	const pw, ph = 300.0, 400.0
	rect := toolkit.Rect{X: 40, Y: 7, W: 351, H: 468}
	run := placeRun(20, 25, 300, 10, pw, ph, rect, "first")
	if got := run.Font.Measure(run.Text); got != run.Bounds.W {
		t.Errorf("the whole text measures %d, the box is %d wide", got, run.Bounds.W)
	}
	if got := run.Font.Measure(""); got != 0 {
		t.Errorf("no text measures %d", got)
	}
	// A prefix is its share of the box, so the caret walks it evenly.
	if got, want := run.Font.Measure("fi"), 2*run.Bounds.W/5; got != want {
		t.Errorf("two of five characters measure %d, want %d", got, want)
	}
	if got := run.Font.Height(); got != run.Bounds.H {
		t.Errorf("the font is %d high, the box is %d", got, run.Bounds.H)
	}
	if got, want := run.Font.Advance(), run.Bounds.W/5; got != want {
		t.Errorf("one character advances %d, want %d", got, want)
	}
}

// ⛔⛔ A DRAG TO THE END OF A LINE KEEPS ITS LAST LETTER, and until placeRun
// measured a run against the run's own box it did not: the interface font it
// handed every run measures "chapter" wider than the page's glyphs do, so the
// caret at the right-hand edge of the box landed at the sixth character and the
// seventh was not reachable by any gesture. A reader who selected a line and
// pressed Copy got "the first chapte".
//
// It is tested on the per-glyph fixture because that is where it shows: a page
// that draws a whole line with one Tj is one run, and one run measured by the
// wrong font loses however many characters the disagreement comes to.
func TestADragToTheEndOfALineKeepsItsLastLetter(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
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
	if got := s.sel.SelectedText(); got != "the first chapter" {
		t.Errorf("a drag over the whole line selected %q", got)
	}
	// And the verb a reader actually presses reports the whole line.
	s.copySelection()
	if !strings.Contains(s.note, "17 characters copied") {
		t.Errorf("Copy says %q", s.note)
	}
}

// ⛔ A PRESS THAT IS NOT ON A PAGE IS NOT A SELECTION. The scrollbar, the rail,
// the ribbon and a form being filled in all keep the press they always had.
func TestAPressOffThePageIsNotASelection(t *testing.T) {
	s := shown(t, linkedPDF(t))
	if s.beginSelect(0, 0) { // the ribbon, at the top left of the surface
		t.Error("a press on the ribbon began a selection")
	}
	if s.selecting {
		t.Error("the workbench thinks a selection is in progress")
	}
	// And a drag with no selection in progress is not one either, so the
	// widget under the pointer still gets it.
	if s.dragSelect(10, 10) || s.endSelect(10, 10) {
		t.Error("a drag outside a selection was taken for one")
	}
}

// ⛔ And not while the band holds something that is NOT the viewer: a form
// being filled in needs its presses.
func TestAFormKeepsItsPresses(t *testing.T) {
	s := shown(t, linkedPDF(t))
	x, y, ok := onPage(s)
	if !ok {
		t.Fatal("no page on screen")
	}
	if !s.beginSelect(x, y) {
		t.Fatal("precondition: the viewer should take a press on a page")
	}
	s.sel.Clear()
	s.selecting = false

	s.tools.reading.Set("text") // the band now holds a reading
	s.refresh()
	s.draw(buffer())
	if s.beginSelect(x, y) {
		t.Error("a reading's press was taken for a selection")
	}
}

// Escape lets go, and says whether there was anything to let go of — so
// Escape with nothing selected still reaches whatever else wants it.
func TestEscapeClearsTheSelection(t *testing.T) {
	s := shown(t, linkedPDF(t))
	if s.clearSelection() {
		t.Error("there was nothing selected and Escape claimed the key")
	}
	runs := s.textRuns(s.src, s.docStamp)
	b := runs[0].Bounds
	s.beginSelect(b.X, b.Y+b.H/2)
	s.endSelect(b.X+b.W*4, b.Y+b.H/2)
	if s.sel.IsEmpty() {
		t.Fatal("precondition: something should be selected")
	}
	if !s.clearSelection() {
		t.Error("Escape did not take the key when text was selected")
	}
	if !s.sel.IsEmpty() {
		t.Error("the selection survived Escape")
	}
}

// ⛔ Copying says HOW MUCH, and says so when there is nothing: a clipboard is
// somewhere else, and a verb that worked in silence is one a reader presses
// twice.
func TestCopyingSaysWhatItDid(t *testing.T) {
	s := shown(t, linkedPDF(t))
	s.copySelection()
	if !strings.Contains(s.note, "nothing is selected") {
		t.Errorf("copying nothing said %q", s.note)
	}

	runs := s.textRuns(s.src, s.docStamp)
	b := runs[0].Bounds
	s.beginSelect(b.X, b.Y+b.H/2)
	s.endSelect(b.X+b.W*8, b.Y+b.H/2)
	s.copySelection()
	if !strings.Contains(s.note, "characters copied") {
		t.Errorf("copying a selection said %q", s.note)
	}
	if got := toolkit.ClipboardText(); got != s.sel.SelectedText() {
		t.Errorf("the clipboard holds %q and the selection is %q", got, s.sel.SelectedText())
	}
}

// ⛔ THE PAGE'S TEXT IS READ ONCE PER DOCUMENT. The runs are placed again on
// every frame a selection is up — the reader can scroll with text lit, and a
// highlight where the words USED to be says the wrong words are chosen — so
// what must not be repeated is the READING.
func TestAPagesTextIsReadOncePerDocument(t *testing.T) {
	s := shown(t, linkedPDF(t))

	calls := 0
	was := extractRuns
	extractRuns = func(src *reader.Document, page int) ([]extract.Run, error) {
		calls++
		return was(src, page)
	}
	t.Cleanup(func() { extractRuns = was })

	stamp := [2]uint64{21, 21} // one this document has not been read under
	for range 30 {
		s.textRuns(s.src, stamp)
	}
	// The fixture is three pages and they are not all on screen, so what this
	// asserts is that the count does not grow with the frames.
	if calls > s.src.PageCount() {
		t.Errorf("30 frames read the pages' text %d times", calls)
	}
	if calls == 0 {
		t.Error("no page was read at all, so this test is measuring nothing")
	}

	// And a different document is read again.
	before := calls
	s.textRuns(s.src, [2]uint64{22, 22})
	if calls == before {
		t.Error("a different document got the previous one's text")
	}
}

// ⛔ A selection drawn where the words are NOT is worse than none: it says the
// wrong words are chosen. The runs follow the page when it moves.
func TestTheRunsFollowThePage(t *testing.T) {
	s := shown(t, linkedPDF(t))
	before := s.textRuns(s.src, s.docStamp)
	if len(before) == 0 {
		t.Fatal("no runs")
	}
	// ⛔ MOVE THE PAGE, not the reader's place in it. Going to another page is
	// no test at all here: the viewer shows one page centred in the same card,
	// and this fixture sets the same words on every page — so page 2's first
	// run is the same text at the same pixel, and an assertion that "it moved"
	// fails against code that is working. Widening the rail narrows the pane,
	// which moves the card itself.
	s.railWidth.Set(railMaxW)
	s.refresh()
	s.draw(buffer())
	after := s.textRuns(s.src, s.docStamp)
	if len(after) == 0 {
		t.Fatal("no runs after the pane changed")
	}
	if before[0].Bounds == after[0].Bounds {
		t.Errorf("the text stayed at %+v while the page moved", before[0].Bounds)
	}
	// And it still says the same words: it moved, it did not change.
	if before[0].Text != after[0].Text {
		t.Errorf("the first run reads %q, was %q", after[0].Text, before[0].Text)
	}
}

// ⛔⛔ THE WHOLE GESTURE, THROUGH THE HANDLERS A BROWSER CALLS. Every other
// test here reaches past them into beginSelect and its pair. That proves the
// selection works and not that a reader can make one: what arrives from the
// page is handleClick, handleMove and handleRelease, and a press that never
// got routed to the selection would leave all of them green.
func TestADragOnThePageSelectsThroughTheHandlers(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) == 0 {
		t.Fatal("the page offers no text")
	}
	b := runs[0].Bounds
	y := b.Y + b.H/2
	var right int
	for _, r := range runs {
		right = max(right, r.Bounds.X+r.Bounds.W)
	}

	if !s.handleClick(b.X, y) {
		t.Fatal("a press on the page was not handled")
	}
	if !s.handleMove(b.X+(right-b.X)/2, y) {
		t.Error("a drag across the page was not handled")
	}
	if !s.handleRelease(right, y) {
		t.Error("letting go on the page was not handled")
	}
	if s.selecting {
		t.Error("the workbench still thinks a drag is in progress")
	}
	if got := s.sel.SelectedText(); !strings.Contains(got, "the first") {
		t.Errorf("the gesture selected %q", got)
	}
	// And the highlight is painted: with a selection up, drawing reaches the
	// code that places the runs again and paints over them.
	s.draw(buffer())

	// ⛔ Escape through the key handler, which is what a reader presses.
	if !s.handleKeyDown("Escape") {
		t.Error("Escape did not take the key while text was selected")
	}
	if !s.sel.IsEmpty() {
		t.Error("the selection survived Escape")
	}
}

// ⛔ A PRESS INSIDE THE PANE BUT NOT ON THE VIEWER. PageAt maps a point to a
// page through the pane's scroll and does not refuse one outside the pane, so
// without a bounds test of its own every press on a ribbon control became a
// selection and no verb ever ran — eleven tests fell over at once.
func TestAPressOutsideTheViewerIsNotASelection(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	r := s.pv.Bounds()
	for _, c := range []struct {
		what string
		x, y int
	}{
		{"above the viewer", r.X + r.W/2, r.Y - 5},
		{"left of it", r.X - 5, r.Y + r.H/2},
		{"below it", r.X + r.W/2, r.Y + r.H + 5},
		{"right of it", r.X + r.W + 5, r.Y + r.H/2},
	} {
		if s.beginSelect(c.x, c.y) {
			t.Errorf("a press %s began a selection", c.what)
			s.sel.Clear()
			s.selecting = false
		}
	}

	// ⛔ And INSIDE the viewer but beside the page: the pane is wider than the
	// card it centres, so there is paper-less room to press in. PageAt is what
	// answers that, and it is a different question from the bounds above.
	if s.beginSelect(r.X+1, r.Y+r.H/2) {
		t.Error("a press beside the page began a selection")
	}
}

// ⛔ A page that declares a size of NOTHING is stepped over: a scale derived
// from it would not be a number, and every run on it would land at infinity.
func TestAPageWithNoSizeOffersNoText(t *testing.T) {
	s := shown(t, degeneratePDF(t))
	if got := s.textRuns(s.src, s.docStamp); len(got) != 0 {
		t.Errorf("a page with a zero MediaBox offered %d runs", len(got))
	}
}

// degeneratePDF is one page that says it is no size at all.
func degeneratePDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(0), reader.Integer(0)},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(0), reader.Integer(0)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔ A page that carries NO text is remembered as carrying none, rather than
// read again on every frame — the same reason the outline and the links are
// remembered as empty.
func TestAPageWithNoTextIsRememberedAsEmpty(t *testing.T) {
	s := shown(t, blankPagePDF(t))
	stamp := [2]uint64{31, 31}
	if got := s.runsFor(s.src, 1, stamp); len(got) != 0 {
		t.Fatalf("a blank page offered %d runs", len(got))
	}
	calls := 0
	was := extractRuns
	extractRuns = func(src *reader.Document, page int) ([]extract.Run, error) {
		calls++
		return was(src, page)
	}
	t.Cleanup(func() { extractRuns = was })
	for range 10 {
		s.runsFor(s.src, 1, stamp)
	}
	if calls != 0 {
		t.Errorf("a blank page was read again %d times", calls)
	}
}

// ⛔ A page of a real size with nothing on it offers no words: the grouping
// has to cope with being handed none, which is the case where it is asked to
// close a word it never opened.
func TestABlankPageOffersNoWords(t *testing.T) {
	s := shown(t, blankPagePDF(t))
	if got := s.textRuns(s.src, s.docStamp); len(got) != 0 {
		t.Errorf("a blank page offered %d words", len(got))
	}
}

// blankPagePDF is one page of a real size with nothing drawn on it.
func blankPagePDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// With no document there is nothing to select and nothing to crash on.
func TestNoDocumentOffersNoText(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	if got := s.textRuns(nil, [2]uint64{}); got != nil {
		t.Errorf("an empty workbench offers %d runs", len(got))
	}
	if s.beginSelect(10, 10) {
		t.Error("an empty workbench began a selection")
	}
	s.drawSelection(painter.NewPixelPainter(buffer(), surfaceW, surfaceH))
}
