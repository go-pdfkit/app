package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
)

// ⛔⛔ A DOCUMENT CAN CARRY OTHER FILES, and the workbench never said so — a
// PDF arriving with a spreadsheet inside it looked like a PDF with nothing
// inside it. `ops` has had Attach, Attachments and Detach throughout.
func TestTheFilesInsideADocumentAreShown(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	if err := s.doc.Attach("figures.csv", []byte("a,b\n1,2\n"), "the numbers behind figure 3"); err != nil {
		t.Fatal(err)
	}
	s.readCarried()
	if s.tools.reading.Get() != readingCarried {
		t.Fatalf("the reading is %q", s.tools.reading.Get())
	}
	if w := s.carriedView(); w == nil {
		t.Fatal("the view is nothing")
	}
	carried := s.doc.Attachments()
	if len(carried) != 1 || carried[0].Name != "figures.csv" {
		t.Fatalf("the document carries %v", carried)
	}
	if got := carriedSays(carried[0]); !strings.Contains(got, "8 bytes") ||
		!strings.Contains(got, "figure 3") {
		t.Errorf("the row says %q", got)
	}
}

// A file with nothing said about it says that, rather than showing an empty
// half-sentence.
func TestACarriedFileWithNoDescriptionSaysSo(t *testing.T) {
	got := carriedSays(ops.Attachment{Name: "x", Data: []byte("abc")})
	if !strings.Contains(got, "3 bytes") {
		t.Errorf("the row says %q", got)
	}
	if !strings.Contains(got, "nothing said about what it is") {
		t.Errorf("the row says %q", got)
	}
}

// A document carrying nothing says so, and one that is not open says that
// instead.
func TestTheCarriedViewWithNothingToShow(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	if w := s.carriedView(); w == nil {
		t.Fatal("the view is nothing")
	}
	if got := s.doc.Attachments(); len(got) != 0 {
		t.Fatalf("a plain document carries %v", got)
	}
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	if w := empty.carriedView(); w == nil {
		t.Fatal("the view is nothing with no document")
	}
}

// ⛔ PUTTING A FILE IN SHOWS THE LIST, because a file that went somewhere
// invisible is a file somebody will attach twice.
func TestPuttingAFileInsideShowsWhatIsInside(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "notes.txt", []byte("keep this")
	s.attach()
	if !strings.Contains(s.note, "put notes.txt inside") {
		t.Fatalf("the status line says %q", s.note)
	}
	if s.tools.reading.Get() != readingCarried {
		t.Errorf("the reading is %q, so the file went somewhere invisible",
			s.tools.reading.Get())
	}
	carried := s.doc.Attachments()
	if len(carried) != 1 || string(carried[0].Data) != "keep this" {
		t.Errorf("the document carries %v", carried)
	}
	// And it survives being written out and read back, which is the only
	// question that matters about a file inside a file.
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ops.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	back := again.Attachments()
	if len(back) != 1 || back[0].Name != "notes.txt" || string(back[0].Data) != "keep this" {
		t.Errorf("after a write and a read it carries %v", back)
	}
}

// ⛔⛔ THE LIBRARY'S REFUSAL OF A NAME ALREADY USED REACHES THE READER. ops
// refuses rather than replacing, because "two files under one name is a
// document that has lost one of them" — and a workbench that swallowed that
// would be the thing losing it.
func TestAttachingTheSameNameTwiceIsRefused(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "notes.txt", []byte("first")
	s.attach()
	h.name, h.file = "notes.txt", []byte("second")
	s.attach()
	if s.note == "" || strings.Contains(s.note, "put notes.txt inside the document") {
		t.Errorf("the second attachment said %q", s.note)
	}
	carried := s.doc.Attachments()
	if len(carried) != 1 {
		t.Fatalf("the document carries %d file(s): %v", len(carried), carried)
	}
	if string(carried[0].Data) != "first" {
		t.Errorf("the first file was replaced after all: %q", carried[0].Data)
	}
}

// Dropping a file takes it out.
func TestDroppingACarriedFileTakesItOut(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "notes.txt", []byte("keep this")
	s.attach()
	s.detach("notes.txt")
	if !strings.Contains(s.note, "dropped notes.txt") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := s.doc.Attachments(); len(got) != 0 {
		t.Errorf("the document still carries %v", got)
	}
}

// ⛔ AND DROPPING ONE THAT IS NOT THERE SAYS SO. Detach reports whether it
// found anything, and a verb that said "dropped" either way would be telling
// somebody their file is gone when it was never there.
func TestDroppingAFileThatIsNotThereSaysSo(t *testing.T) {
	s, _ := openedCarrier(t)
	s.detach("nothing.txt")
	if !strings.Contains(s.note, "carries no file called nothing.txt") {
		t.Errorf("the status line says %q", s.note)
	}
}

// Both need a document, and say so.
func TestTheCarriedVerbsNeedADocument(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.attach()
	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("attaching said %q", s.note)
	}
	s.note = ""
	s.detach("x")
	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("dropping said %q", s.note)
	}
}

// ⛔ MIXING ANOTHER FILE IN takes one page from each in turn, which is how two
// single-sided scans of the same document become one.
func TestMixingAnotherFileInAlternatesThePages(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "backs.pdf", samplePDF(t, 2)
	was := s.doc.PageCount()
	s.interleave()
	if !strings.Contains(s.note, "mixed in backs.pdf") {
		t.Fatalf("the status line says %q", s.note)
	}
	if got := s.doc.PageCount(); got != was+2 {
		t.Errorf("%d pages after mixing two in, from %d", got, was)
	}
}

// ⛔⛔ THE TWO BUTTONS BESIDE A CARRIED FILE ARE WIRED, and where they are is
// SWEPT FOR rather than computed — the same way the picture list beside them
// is tested, and for the reason that test gives: "the list is put where the
// page was, not in the panel, and what says the control is wired is that a
// press finds it".
//
// Left to right, so "Take it out" is met before "Drop it": the first hands the
// file over and leaves the list alone, the second empties it. One sweep
// therefore exercises both, and the order is what makes that possible — a
// sweep from the right would drop the file before anything could take it out.
func TestTheButtonsBesideACarriedFileAreWired(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "notes.txt", []byte("keep this")
	s.attach()
	if len(s.doc.Attachments()) != 1 {
		t.Fatalf("the fixture carries %v", s.doc.Attachments())
	}
	h.as, h.saved = "", nil

	tookOut, dropped := false, false
	for y := s.viewTop(); y < s.viewTop()+s.viewH(); y += 4 {
		for x := margin; x < s.pageW(); x += 8 {
			s.handleClick(x, y)
			s.draw(buffer())
			if h.as == "notes.txt" {
				tookOut = true
			}
			if len(s.doc.Attachments()) == 0 {
				dropped = true
			}
			if tookOut && dropped {
				break
			}
		}
		if tookOut && dropped {
			break
		}
	}
	if !tookOut {
		t.Errorf("no press beside the file took it out; the status line says %q", s.note)
	}
	if !dropped {
		t.Errorf("no press beside the file dropped it; the status line says %q", s.note)
	}
	if tookOut && string(h.saved) != "keep this" {
		t.Errorf("what was taken out holds %q", h.saved)
	}
}

// openedCarrier opens a two-page document and hands back the host, whose
// picker can be told what to offer.
func openedCarrier(t *testing.T) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "carrier.pdf", file: samplePDF(t, 2)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	s.draw(buffer())
	return s, h
}

// A carried file survives every other verb, because losing one quietly is the
// failure nobody notices until they need it.
func TestACarriedFileSurvivesTheOtherVerbs(t *testing.T) {
	s, h := openedCarrier(t)
	h.name, h.file = "notes.txt", []byte("keep this")
	s.attach()
	s.tools.spec.Set("all")
	s.tools.turn.Set(90)
	s.turnRange()
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ops.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Attachments(); len(got) != 1 {
		t.Errorf("turning the pages lost the carried file: %v", got)
	}
}
