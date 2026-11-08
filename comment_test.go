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

// narrowPagePDF is the same line of text as perGlyphPDF on paper only a little
// wider than the words: a page with no margin to write in.
func narrowPagePDF(t *testing.T) []byte {
	t.Helper()
	return pagePDFOfSize(t, 120, 400)
}

// pagePDFOfSize sets perGlyphPDF's line on paper of the size given, so a test
// can choose how much room there is around the words.
func pagePDFOfSize(t *testing.T, pw, ph int) []byte {
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
			x += 3.3
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
			reader.Integer(pw), reader.Integer(ph)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// annotsOf is the annotations the written document carries on a page, as
// dictionaries.
func annotsOf(t *testing.T, s *state, page int) []reader.Dict {
	t.Helper()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	p, err := src.Page(page)
	if err != nil {
		t.Fatal(err)
	}
	list, err := src.Resolve(p.Get("Annots"))
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := reader.ToArray(list)
	var out []reader.Dict
	for _, ref := range arr {
		obj, err := src.Resolve(ref)
		if err != nil {
			t.Fatal(err)
		}
		if d, ok := reader.ToDict(obj); ok {
			out = append(out, d)
		}
	}
	return out
}

// rectOf is an annotation's /Rect as four numbers.
func rectOf(t *testing.T, s *state, d reader.Dict) [4]float64 {
	t.Helper()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	obj, err := src.Resolve(d.Get("Rect"))
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := reader.ToArray(obj)
	if !ok || len(arr) != 4 {
		t.Fatalf("the /Rect is %v", obj)
	}
	var out [4]float64
	for i, v := range arr {
		n, ok := reader.ToFloat(v)
		if !ok {
			t.Fatalf("the /Rect holds %v", v)
		}
		out[i] = n
	}
	return out
}

// onlyAnnotOfSubtype is the single annotation of a kind on page 1, failing if
// there is not exactly one.
func onlyAnnotOfSubtype(t *testing.T, s *state, want reader.Name) reader.Dict {
	t.Helper()
	var found []reader.Dict
	for _, d := range annotsOf(t, s, 1) {
		if name, _ := d.Get("Subtype").(reader.Name); name == want {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the page carries %d /%s annotation(s)", len(found), want)
	}
	return found[0]
}

// ⛔⛔ THE NOTE LANDS CLEAR OF THE WORDS IT IS ABOUT. A note's icon is twenty
// points square and opaque: dropped on the passage it would hide exactly what
// it comments on, and no test of "is there an annotation" would notice. So this
// asserts the geometry — the icon's right edge is left of the passage's left
// edge — rather than asserting the number 20, which is ops' and may change.
func TestANoteLandsBesideThePassageAndNotOnIt(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	// The passage's own box, before anything is written.
	groups := s.selectionQuads()
	if len(groups) == 0 {
		t.Fatal("the selection reaches no page")
	}
	// ⛔ The promise is NOT "the icon is to the left": this passage starts 19.7
	// points from the edge and a 20-point icon does not fit there, so the right
	// margin is the correct answer. The promise is that the icon does not sit on
	// the words. The first version of this assertion demanded the left side and
	// failed on a correct placement — an assertion about the implementation
	// rather than about the behaviour.
	q := groups[0].quads[0]
	px0, py0, px1, py1 := q.TopLeftX, q.BottomLeftY, q.TopRightX, q.TopLeftY
	s.tools.comment.Set("this needs a citation")
	s.noteOnSelection()
	if !strings.Contains(s.note, "put a note on page 1") {
		t.Fatalf("the status line says %q", s.note)
	}
	d := onlyAnnotOfSubtype(t, s, "Text")
	rect := rectOf(t, s, d)
	if overlapsBox(rect[0], rect[1], rect[2], rect[3], px0, py0, px1, py1) {
		t.Errorf("the note covers the passage: the icon is %v and the words are "+
			"%.1f %.1f %.1f %.1f", rect, px0, py0, px1, py1)
	}
	// And it is on the paper: the page is 300 by 400.
	if rect[0] < 0 || rect[1] < 0 || rect[2] > 300 || rect[3] > 400 {
		t.Errorf("the note is off the paper, at %v", rect)
	}
	// And it says what was typed.
	if got, _ := d.Get("Contents").(reader.String); string(got) != "this needs a citation" {
		t.Errorf("the note says %q", got)
	}
	// ⛔ The selection is let go, as the marks do: the comment has landed in the
	// file, and words still lit afterwards invite a second press that writes a
	// second note saying the same thing.
	if !s.sel.IsEmpty() {
		t.Error("the passage is still selected")
	}
}

// ⛔ The placement rule on its own, each of its three answers and its refusal.
// Through the function rather than through a page, because the third answer and
// the refusal need paper shaped in ways no fixture here has.
func TestWhereANoteGoesBesideAPassage(t *testing.T) {
	const pw, ph = 300.0, 400.0
	for _, c := range []struct {
		name           string
		x0, y0, x1, y1 float64
		pw, ph         float64
		wantX, wantY   float64
		wantRoom       bool
	}{
		{
			name: "the left margin when there is room",
			x0:   100, y0: 290, x1: 200, y1: 300, pw: pw, ph: ph,
			wantX: 100 - noteSide - noteGap, wantY: 300 - noteSide, wantRoom: true,
		},
		{
			// A passage that starts too near the edge for the icon: the right
			// margin is the answer, and the clamp of the first version put the
			// icon on the words instead.
			name: "the right margin when the left has none",
			x0:   19.7, y0: 290, x1: 96, y1: 300, pw: pw, ph: ph,
			wantX: 96 + noteGap, wantY: 300 - noteSide, wantRoom: true,
		},
		{
			// Text running the whole width of the paper: neither side has room,
			// and above the line does.
			name: "above the line when neither side has room",
			x0:   10, y0: 100, x1: 290, y1: 120, pw: pw, ph: ph,
			wantX: 10, wantY: 120 + noteGap, wantRoom: true,
		},
		{
			// A line running nearly the full width AND near the head of the
			// paper: neither margin has room sideways, and above the line falls
			// off the top. There is nowhere, and the verb says so rather than
			// drawing the icon over the words.
			name: "nowhere at all",
			x0:   10, y0: 380, x1: 290, y1: 400, pw: pw, ph: ph,
			wantRoom: false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			x, y, room := notePlace(c.x0, c.y0, c.x1, c.y1, c.pw, c.ph)
			if room != c.wantRoom {
				t.Fatalf("room = %v, want %v (at %.1f, %.1f)", room, c.wantRoom, x, y)
			}
			if !room {
				return
			}
			if x != c.wantX || y != c.wantY {
				t.Errorf("the icon goes at %.1f, %.1f, want %.1f, %.1f",
					x, y, c.wantX, c.wantY)
			}
			// Whatever it answered, the invariant holds.
			if overlapsBox(x, y, x+noteSide, y+noteSide, c.x0, c.y0, c.x1, c.y1) {
				t.Error("the icon sits on the passage")
			}
			if x < 0 || y < 0 || x+noteSide > c.pw || y+noteSide > c.ph {
				t.Error("the icon is off the paper")
			}
		})
	}
}

func TestAMarginCommentLandsInTheMarginBesideThePassage(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	groups := s.selectionQuads()
	if len(groups) == 0 {
		t.Fatal("the selection reaches no page")
	}
	passageRight := groups[0].quads[0].TopRightX
	s.tools.comment.Set("the author disagrees with himself here")
	s.commentInMargin()
	if !strings.Contains(s.note, "wrote in the margin of page 1") {
		t.Fatalf("the status line says %q", s.note)
	}
	d := onlyAnnotOfSubtype(t, s, "FreeText")
	rect := rectOf(t, s, d)
	if rect[0] < passageRight {
		t.Errorf("the comment starts at x=%.1f, inside a passage ending at x=%.1f",
			rect[0], passageRight)
	}
	// The page is 300 points wide; the box must stay on the paper.
	if rect[2] > 300 {
		t.Errorf("the comment runs to x=%.1f on a 300-point page", rect[2])
	}
	if !s.sel.IsEmpty() {
		t.Error("the passage is still selected")
	}
}

// ⛔ A margin too narrow to read is REFUSED rather than filled with one word
// per line. ops clips what does not fit, which is right for a box somebody chose
// and wrong for one this verb chose for them — and the reader cannot widen it
// from here, so a result nobody can read would be a dead end.
func TestAMarginTooNarrowIsRefusedWithItsWidth(t *testing.T) {
	// A page only a little wider than its text leaves no margin.
	s := shown(t, narrowPagePDF(t))
	selectFirstLine(t, s)
	s.tools.comment.Set("no room for this")
	s.commentInMargin()
	if !strings.Contains(s.note, "margin beside that passage") {
		t.Fatalf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "points wide") {
		t.Errorf("the refusal does not say how narrow it is: %q", s.note)
	}
	for _, d := range annotsOf(t, s, 1) {
		if name, _ := d.Get("Subtype").(reader.Name); name == "FreeText" {
			t.Error("a comment was written into a margin too narrow for it")
		}
	}
	// ⛔ And the refusal points at what to do instead, because a note needs no
	// margin at all.
	if !strings.Contains(s.note, "note") {
		t.Errorf("the refusal offers no way forward: %q", s.note)
	}
}

func TestSayingSomethingWithNothingTypedOrNothingSelected(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(*state)
		run  func(*state)
		says string
	}{
		{"a note with nothing typed",
			func(s *state) { selectFirstLine(t, s) }, (*state).noteOnSelection,
			"type the comment beside it first"},
		{"a comment with nothing typed",
			func(s *state) { selectFirstLine(t, s) }, (*state).commentInMargin,
			"type the comment beside it first"},
		{"a note with nothing selected",
			func(s *state) { s.tools.comment.Set("something") }, (*state).noteOnSelection,
			"nothing is selected"},
		{"a comment with nothing selected",
			func(s *state) { s.tools.comment.Set("something") }, (*state).commentInMargin,
			"nothing is selected"},
		{"a note of only blanks",
			func(s *state) {
				selectFirstLine(t, s)
				s.tools.comment.Set("   \t ")
			}, (*state).noteOnSelection,
			"type the comment beside it first"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, perGlyphPDF(t))
			c.set(s)
			c.run(s)
			if !strings.Contains(s.note, c.says) {
				t.Errorf("the status line says %q, not %q", s.note, c.says)
			}
			if n := len(annotsOf(t, s, 1)); n != 0 {
				t.Errorf("%d annotation(s) were written anyway", n)
			}
		})
	}
}

// ⛔ A comment goes on the page the PASSAGE is on. A selection dragged across a
// page boundary is two stretches of text and a note is one point: attaching it
// to the second page because the drag ended there would put the comment where
// the reader was not looking.
func TestACommentGoesOnTheFirstPageTheSelectionReaches(t *testing.T) {
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
	if !s.beginSelect(first.X, first.Y+first.H/2) {
		t.Fatal("a press on the first page did not begin a selection")
	}
	s.dragSelect(last.X+last.W, last.Y+last.H/2)
	s.endSelect(last.X+last.W, last.Y+last.H/2)
	if len(s.selectionQuads()) != 2 {
		t.Fatalf("the selection reaches %d page(s), so this measures nothing",
			len(s.selectionQuads()))
	}
	s.tools.comment.Set("about the first page")
	s.noteOnSelection()
	if !strings.Contains(s.note, "page 1") {
		t.Errorf("the status line says %q", s.note)
	}
	if n := len(annotsOf(t, s, 2)); n != 0 {
		t.Errorf("the SECOND page carries %d annotation(s)", n)
	}
	if n := len(annotsOf(t, s, 1)); n != 1 {
		t.Errorf("the first page carries %d annotation(s)", n)
	}
}

// ⛔ The author of the file is the author of the comment. A note with no /T is
// a comment nobody signed, and the panel already knows who is writing: the two
// boxes on the File tab are what Save writes into the document.
// ⛔ The refusal has to REACH THE STATUS LINE, not just exist in notePlace. A
// verb that worked out there was nowhere to put the note and then said nothing
// is a press that did nothing, which reads as a broken workbench.
func TestANoteWithNowhereToGoSaysSo(t *testing.T) {
	// Paper 120 by 330: the line runs to x=96 so neither margin takes a
	// 20-point icon, and the line sits too near the head for one above it.
	s := shown(t, pagePDFOfSize(t, 120, 330))
	selectFirstLine(t, s)
	s.tools.comment.Set("nowhere for this")
	s.noteOnSelection()
	if !strings.Contains(s.note, "nowhere beside that passage") {
		t.Fatalf("the status line says %q", s.note)
	}
	// And it offers the way forward, because a margin comment needs no icon.
	if !strings.Contains(s.note, "margin") {
		t.Errorf("the refusal offers no way forward: %q", s.note)
	}
	if n := len(annotsOf(t, s, 1)); n != 0 {
		t.Errorf("%d annotation(s) were written anyway", n)
	}
}

// ⛔⛔ THE WRITE SEAM IS NOT THE RIGHT PROBE FOR THESE TWO, and finding that out
// is worth keeping. The first version of this test made `docBytes` fail and
// expected "cannot be written": both verbs reported success instead, and they
// were RIGHT. changeSaying applies the operation to the document in memory and
// returns false only for a nil document or an ops refusal; a write-back that
// fails does not unmake the comment, it only spoils the preview, and Save is
// what reports it. So the behaviour to pin down is the one that IS true — the
// comment is in the document even when the preview could not be rebuilt.
func TestACommentSurvivesAPreviewThatCannotBeRebuilt(t *testing.T) {
	for _, c := range []struct {
		name    string
		run     func(*state)
		subtype reader.Name
	}{
		{"a note", (*state).noteOnSelection, "Text"},
		{"a margin comment", (*state).commentInMargin, "FreeText"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, perGlyphPDF(t))
			selectFirstLine(t, s)
			s.tools.comment.Set("something")
			was := docBytes
			docBytes = func(*ops.Doc) ([]byte, error) {
				return nil, errors.New("the ink ran out")
			}
			c.run(s)
			// The write works again: what the verb put in the document is
			// still there, which is what Save would hand over.
			docBytes = was
			found := false
			for _, d := range annotsOf(t, s, 1) {
				if name, _ := d.Get("Subtype").(reader.Name); name == c.subtype {
					found = true
				}
			}
			if !found {
				t.Errorf("the document carries no /%s: %q", c.subtype, s.note)
			}
		})
	}
}

func TestACommentIsSignedByWhoeverTheFileSaysIsWriting(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	s.tools.author.Set("a careful reader")
	selectFirstLine(t, s)
	s.tools.comment.Set("check this")
	s.noteOnSelection()
	d := onlyAnnotOfSubtype(t, s, "Text")
	if got, _ := d.Get("T").(reader.String); string(got) != "a careful reader" {
		t.Errorf("the note is signed %q", got)
	}
}

func TestAPageThatDoesNotSayHowLargeItIsCannotTakeAComment(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	s.tools.comment.Set("something")
	was := s.src
	t.Cleanup(func() { s.src = was })
	s.src = nil // the page size cannot be read
	s.noteOnSelection()
	if !strings.Contains(s.note, "nothing is selected") &&
		!strings.Contains(s.note, "how large it is") {
		t.Errorf("the status line says %q", s.note)
	}
}
