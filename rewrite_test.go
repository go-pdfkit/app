package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// twoWordsPage draws two strings in ONE BT/ET with no Td between them, so the
// second starts wherever the pen ended after the first.
//
// ⛔⛔ THAT IS WHAT MAKES THIS FIXTURE ABLE TO FAIL. Replace the first with a
// string of another width and the second MOVES, unless the replacement carries
// the kern that cancels the difference. A fixture giving each string its own Td
// would place the second absolutely and every test here would pass whatever the
// arithmetic did.
func twoWordsPage(t *testing.T) []byte {
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
	content := w.Add(&reader.Stream{Dict: reader.Dict{},
		Raw: []byte("BT /F1 12 Tf 40 300 Td (Hello) Tj (World) Tj ET")})
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

// selectTheWord drags across the PAGE's own run of that text, converted to
// screen pixels.
//
// ⛔⛔ NOT ACROSS A SCREEN RUN, AND THE DIFFERENCE IS THE WHOLE POINT OF THIS
// FIXTURE. The selection layer groups glyphs into words by the GAPS between
// them, and `(Hello) Tj (World) Tj` leaves no gap — the pen runs straight on —
// so the screen offers one word, "HelloWorld", covering both operations. A test
// that selected that would be refused for naming two operations, correctly, and
// would never reach the arithmetic it exists to check.
//
// What a reader does in that situation is drag across PART of the line, so this
// drags across exactly the glyphs of one operation, mapping the page's own
// coordinates to the screen the way placeRun does.
func selectTheWord(t *testing.T, s *state, text string) {
	t.Helper()
	runs, err := extractRuns(s.src, 1)
	if err != nil {
		t.Fatal(err)
	}
	var want extract.Run
	var said []string
	for _, r := range runs {
		said = append(said, r.Text)
		if r.Text == text {
			want = r
		}
	}
	if want.Text != text {
		t.Fatalf("the page draws no %q; it draws %q", text, said)
	}
	rect, _, ok := s.pv.PageRect(1)
	if !ok {
		t.Fatal("the page is not on screen")
	}
	page, err := s.src.Page(1)
	if err != nil {
		t.Fatal(err)
	}
	pw, ph := pageSize(s.src, page)
	box := placeRun(want.X, want.Width, want.Y, want.Size, pw, ph, rect, want.Text)
	y := box.Bounds.Y + box.Bounds.H/2
	if !s.beginSelect(box.Bounds.X+1, y) {
		t.Fatal("a press on the word did not begin a selection")
	}
	s.dragSelect(box.Bounds.X+box.Bounds.W-1, y)
	s.endSelect(box.Bounds.X+box.Bounds.W-1, y)
}

// wordStartsAt is where a word starts on the page, in the page's own points.
// Not `wordAt`: search.go already has one, about a byte offset within a line.
func wordStartsAt(t *testing.T, s *state, text string) float64 {
	t.Helper()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	runs, err := extractRuns(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Text == text {
			return r.X
		}
	}
	t.Fatalf("the page no longer says %q", text)
	return 0
}

// ⛔⛔ THE WORDS CHANGE AND THE LAYOUT DOES NOT. The verb's whole claim is that
// what follows on the line stays where it was, and the only way to ask is to
// read the page back and measure.
func TestReplacingWordsLeavesTheRestOfTheLineWhereItWas(t *testing.T) {
	for _, now := range []string{"Hi", "Greetings", "Howdy"} {
		t.Run(now, func(t *testing.T) {
			s := shown(t, twoWordsPage(t))
			wasX := wordStartsAt(t, s, "World")
			selectTheWord(t, s, "Hello")
			s.tools.replacement.Set(now)
			s.replaceWords()
			if !strings.Contains(s.note, fmt.Sprintf("replaced %q with %q", "Hello", now)) {
				t.Fatalf("the status line says %q", s.note)
			}
			// The words changed...
			if got := wordStartsAt(t, s, now); got == 0 {
				t.Fatalf("the page does not say %q", now)
			}
			// ...and what follows did not move.
			if nowX := wordStartsAt(t, s, "World"); nowX != wasX {
				t.Errorf("the next word moved from x=%.3f to x=%.3f", wasX, nowX)
			}
			if !s.sel.IsEmpty() {
				t.Error("the words are still selected after being replaced")
			}
		})
	}
}

// ⛔ A page that draws its text one glyph at a time cannot have a word replaced,
// and the refusal says how many pieces it found and what to do instead. On such
// a page — TeX output, for one — every word is several operations, and a bare
// "cannot" would read as a broken verb rather than as a page shaped the wrong
// way for this.
func TestAWordDrawnInSeveralPiecesIsRefusedWithTheCount(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	s.tools.replacement.Set("something else")
	s.replaceWords()
	if !strings.Contains(s.note, "separate operations") {
		t.Fatalf("the status line says %q", s.note)
	}
	// The count is the useful part, and it is the real one: "the first chapter"
	// is fifteen glyphs, each its own operation.
	if !strings.Contains(s.note, "15 separate operations") {
		t.Errorf("the refusal does not name the right count: %q", s.note)
	}
	// And it offers a way forward.
	if !strings.Contains(s.note, "one at a time") {
		t.Errorf("the refusal offers nothing to do instead: %q", s.note)
	}
	// Nothing was changed.
	if said := saidByTheFile(t, s); !strings.Contains(said, "the first chapter") {
		t.Errorf("the page now says %q", said)
	}
}

func TestReplacingWithNothingTypedOrNothingSelected(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(*state)
		says string
	}{
		{"nothing typed", func(s *state) { selectTheWord(t, s, "Hello") },
			"type the words to put there instead"},
		{"only blanks typed", func(s *state) {
			selectTheWord(t, s, "Hello")
			s.tools.replacement.Set("  ")
		}, "type the words to put there instead"},
		{"nothing selected", func(s *state) { s.tools.replacement.Set("Hi") },
			"nothing is selected"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, twoWordsPage(t))
			c.set(s)
			s.replaceWords()
			if !strings.Contains(s.note, c.says) {
				t.Errorf("the status line says %q, not %q", s.note, c.says)
			}
			if said := saidByTheFile(t, s); !strings.Contains(said, "Hello") {
				t.Errorf("the page now says %q", said)
			}
		})
	}
}

// ⛔ ops refuses what it cannot rewrite, and the refusal has to REACH THE STATUS
// LINE rather than being swallowed into a verb that looks as though it worked.
func TestWhatOpsRefusesReachesTheStatusLine(t *testing.T) {
	s := shown(t, twoWordsPage(t))
	selectTheWord(t, s, "Hello")
	// A character the page's font cannot draw.
	s.tools.replacement.Set("Hello☃")
	s.replaceWords()
	if !strings.Contains(s.note, "cannot draw") {
		t.Errorf("the status line says %q", s.note)
	}
	if said := saidByTheFile(t, s); !strings.Contains(said, "Hello") {
		t.Errorf("the page now says %q", said)
	}
}

// ⛔ Several runs can share ONE operation — a TJ array shows several strings
// with a single operator — so the verb counts OPERATIONS and not runs. Counting
// runs would refuse a selection that is one operation drawn in pieces, which is
// the commonest shape of a justified line.
func TestOperationsAreCountedAndNotRuns(t *testing.T) {
	span := extract.Span{From: 10, To: 20}
	other := extract.Span{From: 30, To: 40}
	got := operationsOf([]extract.Run{
		{Text: "a", Op: span},
		{Text: "b", Op: span},  // the same operation
		{Text: "c", Op: other}, // a second one
		{Text: "d"},            // no span at all: not editable, skipped
	})
	if len(got) != 2 {
		t.Fatalf("%d operation(s) were counted: %+v", len(got), got)
	}
	if got[0].Text != "a" || got[1].Text != "c" {
		t.Errorf("the operations came back as %q and %q", got[0].Text, got[1].Text)
	}
}

// ⛔⛔ TEXT WITH NO SPAN IS REFUSED RATHER THAN SKIPPED, which is the same rule
// ops.Doc.Redact keeps and for the same reason: extract reports a run with no
// span when it could not locate the operation that drew it, and an edit cannot
// name an operation nobody can point at.
//
// ⛔ And taking the runs away ENTIRELY would not reach this: the selection is
// derived from the same runs, so a page with none has nothing selected and says
// that instead. What is needed is a run the selection can be built from whose
// Op is empty — which is exactly the state extract documents.
func TestTextWithNoSpanIsRefusedRatherThanSkipped(t *testing.T) {
	s := shown(t, twoWordsPage(t))
	// What the page really draws, so the stand-in sits where the words do.
	real, err := extractRuns(s.src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(real) == 0 {
		t.Fatal("the fixture draws no text")
	}
	noSpan := real[0]
	noSpan.Op = extract.Span{}
	was := extractRuns
	t.Cleanup(func() { extractRuns = was })
	extractRuns = func(*reader.Document, int) ([]extract.Run, error) {
		return []extract.Run{noSpan}, nil
	}
	s.runs = nil // the memo has to be re-read for the seam to be seen
	s.draw(buffer())
	selectTheWord(t, s, noSpan.Text)
	s.tools.replacement.Set("Hi")
	s.replaceWords()
	if !strings.Contains(s.note, "not text this page drew") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔ The box around a selection is the box around all of it, not around its
// first piece — a selection of two words on one line is two quads.
func TestTheBoxAroundASelectionCoversAllOfIt(t *testing.T) {
	x0, y0, x1, y1 := quadsBox([]ops.Quad{
		ops.QuadOf(50, 100, 80, 112),
		ops.QuadOf(10, 100, 40, 112),
	})
	if x0 != 10 || x1 != 80 || y0 != 100 || y1 != 112 {
		t.Errorf("the box is %g %g %g %g", x0, y0, x1, y1)
	}
}
