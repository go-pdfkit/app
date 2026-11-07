package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// saidByTheFile is what the document the workbench would save actually says,
// read back the way anybody else would read it.
//
// ⛔ Not the screen. A redaction that only looked right is the defect; the
// question is what is in the bytes.
func saidByTheFile(t *testing.T, s *state) string {
	t.Helper()
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for i := 1; i <= src.PageCount(); i++ {
		text, err := extract.Text(src, i)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		sb.WriteString(text)
	}
	return sb.String()
}

// ⛔⛔ THE WORDS ARE GONE FROM THE FILE. Every other verb on the Read tab shows
// the document or writes something over it; this one takes text out, and the
// only way to ask whether it did is to read the document back.
func TestRedactingTheSelectionTakesItOutOfTheFile(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	if said := saidByTheFile(t, s); !strings.Contains(said, "the first chapter") {
		t.Fatalf("the fixture says %q, so this measures nothing", said)
	}
	selectFirstLine(t, s)
	s.redactSelection()
	if said := saidByTheFile(t, s); strings.Contains(said, "chapter") {
		t.Errorf("the file still says %q", said)
	}
	if !strings.Contains(s.note, "removed 17 characters from 1 page") {
		t.Errorf("the status line says %q", s.note)
	}
	// ⛔ And it says what it did in the words that matter, because a reader who
	// believes a redaction is a black rectangle is the person this is for.
	if !strings.Contains(s.note, "gone from the file, not covered") {
		t.Errorf("the status line does not say the text was removed: %q", s.note)
	}
	if !s.sel.IsEmpty() {
		t.Error("the selection is still lit after the words were removed")
	}
}

// Pressing Redact with nothing selected says so, rather than doing nothing.
func TestRedactingNothingSaysSo(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	s.redactSelection()
	if !strings.Contains(s.note, "nothing is selected") {
		t.Errorf("the status line says %q", s.note)
	}
	if said := saidByTheFile(t, s); !strings.Contains(said, "the first chapter") {
		t.Errorf("a redaction of nothing changed the file to %q", said)
	}
}

// ⛔⛔ THE LIBRARY'S REFUSAL REACHES THE READER, unchanged. ops.Doc.Redact will
// not remove text it cannot remove completely, and its message is the useful
// part — a reader told why can select differently. A workbench that swallowed
// the refusal and drew a box over the words would be the leak this exists to
// prevent.
//
// The case here is the one a reader walks into: highlight a passage, then
// decide to redact it. The highlight is an annotation across the region, and
// its own contents can hold the very words, so the library says no.
func TestTheLibrarysRefusalReachesTheStatusLine(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	s.highlightSelection()
	if !strings.Contains(s.note, "highlighted") {
		t.Fatalf("the highlight did not take: %q", s.note)
	}
	selectFirstLine(t, s)
	s.redactSelection()
	// ⛔ The reason AND the way out. "Refused" on its own leaves a reader
	// stuck; "drop the mark first" is a thing they can do, and it is the
	// library's own wording rather than one this file invented — which is the
	// point of passing the error through untouched.
	if !strings.Contains(s.note, "drop the mark first") {
		t.Errorf("the status line says %q, and not what to do about it", s.note)
	}
	// And the document is untouched: a refused redaction removes nothing.
	if said := saidByTheFile(t, s); !strings.Contains(said, "the first chapter") {
		t.Errorf("a refused redaction changed the file to %q", said)
	}
}

// ⛔ A selection dragged across two pages is redacted on both, and said in the
// plural. Each page's text lives in its own content stream, so this is two
// removals rather than one — and a reader who dragged across a page break has
// to be told that both pages changed.
func TestRedactingAcrossTwoPagesSaysSo(t *testing.T) {
	s := shown(t, twoLinePagesPDF(t))
	s.pv.Mode().Set(toolkit.PagedContinuous)
	s.draw(buffer())
	if _, clip, ok := s.pv.PageRect(2); !ok || clip.W <= 0 || clip.H <= 0 {
		t.Fatal("the second page is not on screen, so this measures nothing")
	}
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) < 2 {
		t.Fatalf("the two pages offer %d run(s)", len(runs))
	}
	first, last := runs[0].Bounds, runs[len(runs)-1].Bounds
	if !s.beginSelect(first.X, first.Y+first.H/2) {
		t.Fatal("a press on the first page did not begin a selection")
	}
	s.dragSelect(last.X+last.W, last.Y+last.H/2)
	s.endSelect(last.X+last.W, last.Y+last.H/2)
	if groups := s.selectionQuads(); len(groups) != 2 {
		t.Fatalf("the drag reaches %d page(s), want 2", len(groups))
	}
	s.redactSelection()
	if !strings.Contains(s.note, "from 2 pages") {
		t.Errorf("the status line says %q", s.note)
	}
	if said := saidByTheFile(t, s); strings.Contains(said, "first") ||
		strings.Contains(said, "second") {
		t.Errorf("the file still says %q", said)
	}
}

// The count is the characters a reader selected, not the quadrilaterals the
// mark was written as — the fixture draws one glyph at a time, so those are
// three and seventeen.
func TestTheCountIsCharactersAndNotQuads(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	groups := s.selectionQuads()
	if len(groups) != 1 {
		t.Fatalf("the selection reaches %d page(s), want 1", len(groups))
	}
	if n := len(groups[0].quads); n == 17 {
		t.Fatalf("the quads number %d too, so this fixture cannot tell them apart", n)
	}
	s.redactSelection()
	if !strings.Contains(s.note, "17 characters") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔ A mark and a redaction of the same selection cover the same words, because
// both come from selectionQuads. A reader who highlights a line and then
// redacts it must not get a different passage the second time.
func TestAMarkAndARedactionAgreeOnWhere(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	marks, _ := s.selectionMarks(ops.MarkHighlight, 1, 1, 0)
	groups := s.selectionQuads()
	if len(marks) != len(groups) {
		t.Fatalf("the mark reaches %d page(s) and the redaction %d", len(marks), len(groups))
	}
	for i := range marks {
		if marks[i].Page != groups[i].page {
			t.Errorf("the mark is on page %d and the redaction on %d",
				marks[i].Page, groups[i].page)
		}
		if len(marks[i].Quads) != len(groups[i].quads) {
			t.Fatalf("page %d: %d quad(s) marked, %d redacted",
				marks[i].Page, len(marks[i].Quads), len(groups[i].quads))
		}
		for j := range marks[i].Quads {
			if marks[i].Quads[j] != groups[i].quads[j] {
				t.Errorf("page %d quad %d: marked %+v, redacted %+v",
					marks[i].Page, j, marks[i].Quads[j], groups[i].quads[j])
			}
		}
	}
}
