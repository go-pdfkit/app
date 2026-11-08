package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// ⛔⛔ THE DEFECT THIS CLOSES: the panel could WRITE a title and could not show
// the title that was there. So anybody typing into those two boxes was
// overwriting a value the workbench had never shown them. The fixture gives
// the file a title and an author, and this asks whether a reader can see them.
func TestTheFileSaysWhatItSaysAboutItself(t *testing.T) {
	s := shown(t, titledPDF(t))
	s.readAbout()
	rows := strings.Join(s.infoRows(), "\n")
	for _, want := range []string{
		"Title: a borrowed title",
		"Author: somebody else",
		"Producer: a producer nobody chose",
	} {
		if !strings.Contains(rows, want) {
			t.Errorf("the readout does not say %q; it says:\n%s", want, rows)
		}
	}
}

// ⛔ AND IT SHOWS THE KEYS IT DOES NOT KNOW. A producer may put anything in
// there, and a readout that printed only the nine standard entries would
// quietly hide the rest — which is the opposite of what a readout is for.
func TestAnUnknownInfoKeyIsShownToo(t *testing.T) {
	s := shown(t, titledPDF(t))
	rows := strings.Join(s.infoRows(), "\n")
	if !strings.Contains(rows, "Department: somewhere") {
		t.Errorf("a key outside the standard nine was dropped; the readout says:\n%s", rows)
	}
}

// A file that says nothing about itself says so, rather than showing an empty
// heading a reader cannot tell from a failure.
func TestAFileThatSaysNothingSaysSo(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.clearInfo()
	if got := strings.Join(s.infoRows(), "\n"); !strings.Contains(got, "says nothing about itself") {
		t.Errorf("the readout says %q", got)
	}
}

// ⛔ THE SIZE IS THE ONE THE PAGE IS SHOWN AT. pageSize prefers the crop box
// over the media box, because a page cropped to half its paper is half its
// paper to everybody looking at it — so a readout quoting the media box would
// disagree with the picture beside it.
func TestThePageSizesAreTheOnesOnScreen(t *testing.T) {
	s := shown(t, samplePDF(t, 2))
	rows := strings.Join(s.pageRows(s.src), "\n")
	if !strings.Contains(rows, "page 1:") || !strings.Contains(rows, "page 2:") {
		t.Errorf("the readout lists:\n%s", rows)
	}
	// Crop the first page and the readout follows.
	s.tools.spec.Set("1")
	s.tools.box.Set("0,0,100,100")
	s.crop()
	if !strings.Contains(s.note, "cropped") {
		t.Fatalf("the crop did not take: %q", s.note)
	}
	// ⛔ s.src, and it is the right thing to read: renderPage writes the
	// document out and reads it back on every refresh, so the readout lists
	// what would come out of Save rather than the file as it was opened.
	cropped := strings.Join(s.pageRows(s.src), "\n")
	if !strings.Contains(cropped, "page 1: 100 × 100 points") {
		t.Errorf("a cropped page is still listed at its paper size:\n%s", cropped)
	}
}

// A turned page says so, because a size without the rotation is the wrong
// shape on the screen.
func TestATurnedPageSaysSo(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.spec.Set("1")
	s.tools.turn.Set(90)
	s.turnRange()
	if got := strings.Join(s.pageRows(s.src), "\n"); !strings.Contains(got, "turned 90°") {
		t.Errorf("the readout says %q", got)
	}
}

// ⛔ A SOUND FILE SAYS IT IS SOUND. "This file was repaired" is the useful
// message, but its absence must not be silence: a reader cannot tell a file
// that was fine from a readout that forgot to check.
func TestASoundFileSaysItsStructureWasSound(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	if got := s.repairLine(); !strings.Contains(got, "structure was sound") {
		t.Errorf("the readout says %q", got)
	}
}

// ⛔⛔ AND A DAMAGED FILE SAYS SO, AND SAYS WHAT TO DO. "This file was
// repaired" on its own is alarming and useless; what a reader needs is that
// Save writes the rebuilt document, so the repair is one keystroke away rather
// than a thing to go and find another tool for.
//
// The fixture breaks the cross-reference offset — the number after the last
// startxref — so the reader cannot follow it and has to rebuild the table by
// scanning. That is the ordinary damage a truncated download or a bad editor
// leaves, and it is why `pdfops repair` exists.
func TestADamagedFileSaysItWasRebuilt(t *testing.T) {
	s := shown(t, brokenXrefPDF(t))
	if !s.doc.Repaired() {
		t.Fatal("the fixture was read without repair, so this measures nothing")
	}
	got := s.repairLine()
	if !strings.Contains(got, "had to be rebuilt") {
		t.Errorf("the readout says %q", got)
	}
	if !strings.Contains(got, "Saving writes the rebuilt document") {
		t.Errorf("the readout does not say what to do about it: %q", got)
	}
}

// brokenXrefPDF is a sound document whose startxref points nowhere.
func brokenXrefPDF(t *testing.T) []byte {
	t.Helper()
	out := samplePDF(t, 2)
	at := bytes.LastIndex(out, []byte("startxref"))
	if at < 0 {
		t.Fatal("the fixture has no startxref to break")
	}
	// ⛔ The offset is found rather than assumed to be on the same line: a
	// writer may put it after the keyword, on the next line, or after any
	// whitespace it likes. So this walks forward to the first run of digits and
	// overwrites it in place — same length, so nothing else in the file moves,
	// and the only damage is that the offset no longer points at the table.
	broken := append([]byte{}, out...)
	i := at + len("startxref")
	for i < len(broken) && (broken[i] < '0' || broken[i] > '9') {
		i++
	}
	start := i
	for i < len(broken) && broken[i] >= '0' && broken[i] <= '9' {
		broken[i] = '9'
		i++
	}
	if i == start {
		t.Fatal("the startxref carries no offset to break")
	}
	return broken
}

// With nothing open, every part of the readout says so rather than showing an
// empty list.
func TestTheReadoutWithNothingOpen(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	for what, got := range map[string]string{
		"what it says":  strings.Join(s.infoRows(), ""),
		"the pages":     strings.Join(s.pageRows(nil), ""),
		"the structure": s.repairLine(),
	} {
		if !strings.Contains(got, "nothing is open") {
			t.Errorf("%s says %q", what, got)
		}
	}
}

// titledPDF is one page whose information dictionary carries the standard
// entries and one nobody standardised.
func titledPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 10 10 50 50 re f")}),
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(200), reader.Integer(300)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	info := w.Add(reader.Dict{
		"Title":      reader.String("a borrowed title"),
		"Author":     reader.String("somebody else"),
		"Producer":   reader.String("a producer nobody chose"),
		"Department": reader.String("somewhere"),
	})
	out, err := w.Finish(reader.Dict{"Root": root, "Info": info})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
