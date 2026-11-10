package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// wordsOnPages is a document whose pages say what they are given to say, as
// real drawn text, so that the comparison has something to read.
func wordsOnPages(t *testing.T, pages ...string) []byte {
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
	for _, says := range pages {
		var body bytes.Buffer
		fmt.Fprintf(&body, "BT /F1 10 Tf 20 300 Td (%s) Tj ET\n", says)
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: body.Bytes()})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count": reader.Integer(len(pages)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// comparing is a workbench with the given document open and a host that can
// then be told what the picker should hand back.
func comparing(t *testing.T, mine []byte) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "this.pdf", file: mine}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	s.draw(buffer())
	return s, h
}

// theReport is every line the view holds, which is how a report built out of
// labels is read back.
func theReport(s *state) string {
	s.draw(buffer())
	return strings.Join(labelsIn(s.view), "\n")
}

// ⛔⛔ THE DIRECTION, SAID IN THE REPORT. A comparison has two documents in it,
// and a reader who has the order backwards reads every addition as a deletion.
// The file chosen is the EARLIER version and the document open is the later
// one; the heading says so in those words, and this asks the report rather
// than the code.
func TestComparingWithAFileSaysWhichWayRound(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha beta gamma", "delta epsilon zeta"))
	h.name, h.file = "draft1.pdf", wordsOnPages(t, "alpha beta gamma", "delta epsilon theta")

	pressVerb(t, s, "compareFile")
	if s.comparison == nil {
		t.Fatalf("nothing was compared; the status line says %q", s.note)
	}
	if s.comparison.against != "draft1.pdf" {
		t.Errorf("the report is against %q", s.comparison.against)
	}
	if !strings.Contains(s.note, "compared with draft1.pdf") ||
		!strings.Contains(s.note, "1 of 2 page(s) changed") {
		t.Errorf("the status line says %q", s.note)
	}
	said := theReport(s)
	for _, want := range []string{
		"against draft1.pdf",
		"the left page is draft1.pdf; the right page is this document",
		"page 2 → page 2",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the report does not say %q:\n%s", want, said)
		}
	}
	// ⛔ The word the OTHER file had is REMOVED and the word this document has
	// is ADDED. Backwards, both lines are still there and both are wrong.
	removed := strings.Index(said, "text removed")
	added := strings.Index(said, "text added")
	if removed < 0 || added < 0 {
		t.Fatalf("the report has no text change in it:\n%s", said)
	}
	if !strings.Contains(said[removed:added], "theta") {
		t.Errorf("the report says %q was added rather than removed: the comparison is the wrong "+
			"way round\n%s", "theta", said)
	}
	if !strings.Contains(said[added:], "zeta") {
		t.Errorf("the report does not say zeta was added:\n%s", said)
	}
}

// Pressing it again puts the picture of the page back, like every other
// reading.
func TestTheComparisonIsPutAwayLikeAnyOtherReading(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha beta gamma"))
	h.name, h.file = "other.pdf", wordsOnPages(t, "alpha beta gamma")

	pressVerb(t, s, "compareFile")
	if s.tools.reading.Get() != readingCompare {
		t.Fatalf("the reading is %q", s.tools.reading.Get())
	}
	if !strings.Contains(s.note, "the two say the same thing on all 1 page(s)") {
		t.Errorf("two documents that agree say %q", s.note)
	}
	// Nothing changed, so the report is a heading and nothing under it.
	if said := theReport(s); strings.Contains(said, "page 1 → page 1") {
		t.Errorf("a pair with nothing on it was listed:\n%s", said)
	}
	s.read(readingCompare)
	if s.tools.reading.Get() != "" {
		t.Errorf("the reading is still %q", s.tools.reading.Get())
	}
}

// A page added and a page dropped read as such, with a dash for the side that
// has no page.
func TestTheReportNamesAPageAddedAndAPageDropped(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha beta gamma", "a wholly new page"))
	h.name, h.file = "draft1.pdf", wordsOnPages(t, "alpha beta gamma", "one that went away")

	pressVerb(t, s, "compareFile")
	said := theReport(s)
	for _, want := range []string{"page 2 → —", "page removed", "— → page 2", "page added"} {
		if !strings.Contains(said, want) {
			t.Errorf("the report does not say %q:\n%s", want, said)
		}
	}
	if !strings.Contains(s.note, "2 of 3 page(s) changed") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔⛔ A REPORT GOES STALE THE MOMENT THE DOCUMENT CHANGES. It names pages and
// differences, and after a page is dropped it names pages that are not there —
// while looking exactly as authoritative as it did a second earlier. So any
// change drops it, in changeSaying, which is the one place every verb goes
// through.
func TestChangingTheDocumentDropsTheComparison(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha beta gamma", "delta epsilon zeta"))
	h.name, h.file = "draft1.pdf", wordsOnPages(t, "alpha beta gamma")

	pressVerb(t, s, "compareFile")
	if s.comparison == nil {
		t.Fatal("nothing was compared")
	}
	s.tools.spec.Set("1")
	pressVerb(t, s, "drop")
	if s.comparison != nil {
		t.Error("the report of what changed survived a page being dropped, and it names pages " +
			"the document no longer has")
	}
}

func TestComparingBeforeAnythingIsOpen(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.compareWithAFile()
	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("the status line says %q", s.note)
	}
	if s.comparison != nil {
		t.Error("something was compared with nothing")
	}
}

// A picker closed without choosing is not an error, and leaves what was on the
// screen where it was. Both ways a picker can say "nothing": by not calling
// back at all, and by calling back with no bytes.
func TestChangingYourMindAboutTheOtherFile(t *testing.T) {
	for _, c := range []struct {
		name string
		file []byte
	}{
		{"the picker never answered", nil},
		{"the picker answered with nothing", []byte{}},
	} {
		s, h := comparing(t, wordsOnPages(t, "alpha"))
		h.name, h.file = "other.pdf", c.file
		s.compareWithAFile()
		if s.comparison != nil || s.note != "" {
			t.Errorf("%s: it left %q and %+v", c.name, s.note, s.comparison)
		}
	}
}

// ⛔ The comparison is against what SAVE would write, so a document that
// cannot be written cannot be compared — and says so rather than comparing the
// other file with nothing.
func TestADocumentThatCannotBeWrittenCannotBeCompared(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha"))
	h.name, h.file = "other.pdf", wordsOnPages(t, "beta")
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errors.New("the ink ran out") }

	s.compareWithAFile()
	if s.comparison != nil {
		t.Fatal("a document that could not be written was compared anyway")
	}
	if !strings.Contains(s.note, "the ink ran out") {
		t.Errorf("the status line says %q", s.note)
	}
}

// The file chosen has to be a document this can read, and the library's own
// refusal is what reaches the status line.
func TestComparingWithSomethingThatIsNotADocument(t *testing.T) {
	s, h := comparing(t, wordsOnPages(t, "alpha"))
	h.name, h.file = "notes.txt", []byte("this is not a PDF at all")
	s.compareWithAFile()
	if s.comparison != nil {
		t.Fatal("a file that is not a document was compared")
	}
	if !strings.Contains(s.note, "could not be read") {
		t.Errorf("the status line says %q", s.note)
	}
}

// Before anything has been compared, the reading says what to press rather
// than showing an empty report.
func TestTheReportBeforeThereIsOne(t *testing.T) {
	s, _ := comparing(t, wordsOnPages(t, "alpha"))
	s.read(readingCompare) // the reading, with nothing behind it yet
	if said := theReport(s); !strings.Contains(said, "press Compare with a file") {
		t.Errorf("the empty report says %q", said)
	}
}
