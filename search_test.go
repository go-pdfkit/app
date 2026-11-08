package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// searchablePDF sets the lines given, one page per slice, each glyph placed on
// its own the way a typesetter does — which is the only honest fixture for a
// search, because a PDF that held whole words as single strings would let a
// broken word grouping pass.
func searchablePDF(t *testing.T, pages ...[]string) []byte {
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
	kids := make(reader.Array, 0, len(pages))
	for _, lines := range pages {
		var body strings.Builder
		y := 300.0
		for _, line := range lines {
			x := 20.0
			for _, ch := range line {
				if ch == ' ' {
					x += 3.3 // the pen moves; nothing is drawn
					continue
				}
				fmt.Fprintf(&body, "BT /F1 10 Tf %.2f %.2f Td (%c) Tj ET\n", x, y, ch)
				x += 5
			}
			y -= 20
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
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔ The arithmetic on its own, over words made here rather than read from a
// page: what a phrase match covers is the part a page cannot make obvious.
func TestWhatAPhraseMatchCovers(t *testing.T) {
	line := func(words ...string) []word {
		out := make([]word, 0, len(words))
		for i, w := range words {
			out = append(out, word{text: w, line: 0, x: float64(i) * 10})
		}
		return out
	}
	for _, c := range []struct {
		name   string
		words  []word
		needle string
		want   [][2]int
	}{
		{"inside one word", line("the", "first", "chapter"), "irs", [][2]int{{1, 1}}},
		{"a whole word", line("the", "first", "chapter"), "first", [][2]int{{1, 1}}},
		{"two words", line("the", "first", "chapter"), "first chapter", [][2]int{{1, 2}}},
		{"three words", line("the", "first", "chapter"), "the first chapter", [][2]int{{0, 2}}},
		{"across a word boundary", line("the", "first", "chapter"), "e fir", [][2]int{{0, 1}}},
		{"whatever the case", line("The", "First"), "the first", [][2]int{{0, 1}}},
		{"twice on a line", line("ha", "ha"), "ha", [][2]int{{0, 0}, {1, 1}}},
		{"overlapping", line("aaa"), "aa", [][2]int{{0, 0}, {0, 0}}},
		{"not there at all", line("the", "first"), "zebra", nil},
		{"nothing to look for", line("the"), "   ", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := findWords(c.words, c.needle)
			if len(got) != len(c.want) {
				t.Fatalf("found %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("match %d is %v, want %v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// ⛔⛔ A PHRASE NEVER SPANS TWO LINES, and that is not a shortcut. One selection
// covers one stretch of one line's words — the toolkit's own selection joins
// different lines with a newline, not a space — so a match reported across a
// line break could not be lit, copied or marked as what it claims to be.
func TestAPhraseIsNotFoundAcrossALineBreak(t *testing.T) {
	words := []word{
		{text: "the", line: 0},
		{text: "first", line: 0},
		{text: "chapter", line: 1},
	}
	if got := findWords(words, "first chapter"); got != nil {
		t.Errorf("a phrase was found across two lines: %v", got)
	}
	// The control: the same two words on one line ARE found, so the test above
	// is measuring the line break and not a broken search.
	words[2].line = 0
	if got := findWords(words, "first chapter"); len(got) != 1 {
		t.Errorf("the same phrase on one line was found %v", got)
	}
}

func TestFindingAWordTurnsToItAndLightsIt(t *testing.T) {
	s := shown(t, searchablePDF(t,
		[]string{"the first chapter"},
		[]string{"the second chapter", "and a note"},
	))
	s.tools.find.Set("second")
	s.find()
	if !strings.Contains(s.note, "1 of 1") {
		t.Errorf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "page 2 of 2") {
		t.Errorf("the status line says %q, which does not name the page", s.note)
	}
	if s.at != 2 {
		t.Errorf("the workbench is showing page %d", s.at)
	}
	// ⛔⛔ THE POINT OF THE WHOLE FILE: the match is a SELECTION, so everything
	// that works on a selection works on what was found. Checked through the
	// selection's own text rather than through a field, because that text is
	// what Copy would put on the clipboard and what the marks would cover.
	if got := s.sel.SelectedText(); got != "second" {
		t.Errorf("the selection holds %q, not the word that was found", got)
	}
}

func TestFindingAPhraseLightsAllOfIt(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	s.tools.find.Set("first chapter")
	s.find()
	if got := s.sel.SelectedText(); got != "first chapter" {
		t.Errorf("the selection holds %q", got)
	}
}

func TestFindingIsNotFussyAboutCase(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"The First Chapter"}))
	s.tools.find.Set("first")
	s.find()
	if !strings.Contains(s.note, "1 of 1") {
		t.Errorf("the status line says %q", s.note)
	}
	// And what is lit is the page's own capitals, not what was typed.
	if got := s.sel.SelectedText(); got != "First" {
		t.Errorf("the selection holds %q", got)
	}
}

func TestFindingTheNextOneWalksThemAndComesRound(t *testing.T) {
	s := shown(t, searchablePDF(t,
		[]string{"a note here"},
		[]string{"another note"},
	))
	s.tools.find.Set("note")
	s.find()
	if !strings.Contains(s.note, "1 of 2") || s.at != 1 {
		t.Fatalf("the first match is %q on page %d", s.note, s.at)
	}
	s.findNext()
	if !strings.Contains(s.note, "2 of 2") || s.at != 2 {
		t.Fatalf("the second match is %q on page %d", s.note, s.at)
	}
	// ⛔ And round again rather than stopping at the end: a search that goes
	// quiet on the last match looks like a search that broke.
	s.findNext()
	if !strings.Contains(s.note, "1 of 2") || s.at != 1 {
		t.Errorf("after the last match it says %q on page %d", s.note, s.at)
	}
}

// ⛔ The box is a live binding, so what is typed and what was found can differ
// by the time `Find the next one` is pressed. Stepping through the matches of a
// word nobody is looking for any more is what makes a search feel broken.
func TestFindingTheNextOneAfterChangingTheWordSearchesAgain(t *testing.T) {
	s := shown(t, searchablePDF(t,
		[]string{"a note here"},
		[]string{"the second chapter"},
	))
	s.tools.find.Set("note")
	s.find()
	s.tools.find.Set("second")
	s.findNext()
	if !strings.Contains(s.note, "1 of 1") {
		t.Errorf("the status line says %q", s.note)
	}
	if s.at != 2 {
		t.Errorf("the workbench is showing page %d", s.at)
	}
	if got := s.sel.SelectedText(); got != "second" {
		t.Errorf("the selection holds %q", got)
	}
}

func TestFindingTheNextOneWithNothingFoundYetSearches(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	s.tools.find.Set("first")
	s.findNext()
	if !strings.Contains(s.note, "1 of 1") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestFindingNothingSaysSoAndLeavesThePageAlone(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	s.tools.find.Set("zebra")
	s.find()
	if !strings.Contains(s.note, `nothing in this document says "zebra"`) {
		t.Errorf("the status line says %q", s.note)
	}
	if !s.sel.IsEmpty() {
		t.Error("something was lit although nothing was found")
	}
}

func TestFindingWithAnEmptyBoxAsksForAWord(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	s.tools.find.Set("   ")
	s.find()
	if !strings.Contains(s.note, "type what to look for") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestFindingInADocumentThatCannotBeReadBackSaysSo(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errors.New("the ink ran out") }
	s.tools.find.Set("first")
	s.find()
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("the status line says %q", s.note)
	}
	// And the same on the way through Find the next one, which reopens too.
	s.tools.find.Set("first")
	s.hits, s.found = []textMatch{{page: 1}}, "first"
	s.findNext()
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔ A page carrying no text at all is a page, not an error: a scan that has
// not been read, a cover, a blank. The grouping has to come back empty rather
// than reach into an empty group, and the search has to carry on to the pages
// that do say something.
func TestAPageWithNoTextIsSearchedAndSkipped(t *testing.T) {
	if got := pageWords(nil); got != nil {
		t.Errorf("a page of no runs gave %d word(s)", len(got))
	}
	s := shown(t, searchablePDF(t,
		nil, // a page with nothing drawn on it at all
		[]string{"the second chapter"},
	))
	s.tools.find.Set("second")
	s.find()
	if !strings.Contains(s.note, "1 of 1") || s.at != 2 {
		t.Errorf("the status line says %q, on page %d", s.note, s.at)
	}
}

// ⛔ In continuous mode more than one page is on screen, and the match belongs
// to exactly one of them. Placing it against the wrong page's rectangle would
// light the right words at the wrong height — the defect the one page walk in
// select.go exists to prevent — so the page is checked before it is used.
func TestAMatchIsLitOnItsOwnPageWhenTwoAreOnScreen(t *testing.T) {
	s := shown(t, searchablePDF(t,
		[]string{"the first chapter"},
		[]string{"the second chapter"},
	))
	s.pv.Mode().Set(toolkit.PagedContinuous)
	s.draw(buffer())
	if _, clip, ok := s.pv.PageRect(2); !ok || clip.W <= 0 || clip.H <= 0 {
		t.Fatal("the second page is not on screen, so this measures nothing")
	}
	s.tools.find.Set("second")
	s.find()
	if got := s.sel.SelectedText(); got != "second" {
		t.Errorf("the selection holds %q", got)
	}
}

// ⛔ A verb between the search and the press can take the page the match was
// on. The hits are not re-read on every press — that is the point of keeping
// them — so the words they index may be gone, and reaching past the end of a
// slice is how that would show up.
func TestAMatchWhoseWordsHaveGoneDoesNotReachPastTheEnd(t *testing.T) {
	s := shown(t, searchablePDF(t, []string{"the first chapter"}))
	s.tools.find.Set("first")
	s.find()
	s.hits[0] = textMatch{page: 1, from: 99, to: 99}
	s.hitAt = 0
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	s.showHit(src) // must not panic
	if !s.sel.IsEmpty() {
		t.Error("something was lit for a match whose words are gone")
	}
}
