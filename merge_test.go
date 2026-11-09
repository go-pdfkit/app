package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// onePageSaying is a document of one page that draws the given word, so that a
// merged document says which file each of its pages came from.
func onePageSaying(t *testing.T, word string) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
	font := w.Add(reader.Dict{"Type": reader.Name("Font"),
		"Subtype": reader.Name("Type1"), "BaseFont": reader.Name("Helvetica")})
	body := w.Add(&reader.Stream{Dict: reader.Dict{},
		Raw: []byte("BT /F1 12 Tf 20 100 Td (" + word + ") Tj ET")})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pages, "Contents": body,
		"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
	})
	w.Put(pages, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(200), reader.Integer(300)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pages})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSeveralFilesAreAddedInTheOrderTheyWereChosen is the row this answers: a
// merge is nearly always "these scans into one document", and one file at a time
// was five presses and five dialogs, each of which could be cancelled.
//
// ⛔ THE ORDER IS THE PICKER'S, not the order the bytes arrived in. In a browser
// each file is read through a promise of its own, so a small one chosen second
// can arrive first — which is why the host writes each result into its own slot
// and waits for the count. This test reads the pages back out of the written
// document to say the order survived.
func TestSeveralFilesAreAddedInTheOrderTheyWereChosen(t *testing.T) {
	h := &fakeHost{name: "first.pdf", file: onePageSaying(t, "FIRST")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	h.many = []chosen{
		{name: "second.pdf", data: onePageSaying(t, "SECOND")},
		{name: "third.pdf", data: onePageSaying(t, "THIRD")},
		{name: "fourth.pdf", data: onePageSaying(t, "FOURTH")},
	}

	s.merge()

	if got := s.doc.PageCount(); got != 4 {
		t.Fatalf("the document has %d pages: %q", got, s.note)
	}
	if !strings.Contains(s.note, "added 3 files: second.pdf, third.pdf, fourth.pdf") {
		t.Errorf("the status line says %q", s.note)
	}
	for i, want := range []string{"FIRST", "SECOND", "THIRD", "FOURTH"} {
		if said := drawnOn(t, s, i+1); !strings.Contains(said, "("+want+")") {
			t.Errorf("page %d does not draw %s; it draws:\n%s", i+1, want, said)
		}
	}
}

// TestOneFileChosenStillSaysItsName: the plural is for several, and a merge of
// one must not read as "added 1 files".
func TestOneFileChosenStillSaysItsName(t *testing.T) {
	h := &fakeHost{name: "first.pdf", file: onePageSaying(t, "FIRST")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	h.many = []chosen{{name: "second.pdf", data: onePageSaying(t, "SECOND")}}

	s.merge()

	if s.note != "added second.pdf" {
		t.Errorf("the status line says %q", s.note)
	}
	if got := s.doc.PageCount(); got != 2 {
		t.Errorf("the document has %d pages", got)
	}
}

// TestChoosingNoFilesAtAllChangesNothing: a picker closed with nothing chosen is
// somebody changing their mind, which is not an error.
func TestChoosingNoFilesAtAllChangesNothing(t *testing.T) {
	h := &fakeHost{name: "first.pdf", file: onePageSaying(t, "FIRST")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	s.note = ""
	h.many = []chosen{}

	s.merge()

	if s.note != "" {
		t.Errorf("choosing nothing said %q", s.note)
	}
	if got := s.doc.PageCount(); got != 1 {
		t.Errorf("the document has %d pages", got)
	}
}

// TestAFileAmongSeveralThatCannotBeReadStopsTheWholeMerge: appending the ones
// before it and dropping the rest would leave a document nobody asked for, and
// saying nothing about the others would be worse.
func TestAFileAmongSeveralThatCannotBeReadStopsTheWholeMerge(t *testing.T) {
	h := &fakeHost{name: "first.pdf", file: onePageSaying(t, "FIRST")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	h.many = []chosen{
		{name: "second.pdf", data: onePageSaying(t, "SECOND")},
		{name: "rubbish.pdf", data: []byte("just some words")},
		{name: "fourth.pdf", data: onePageSaying(t, "FOURTH")},
	}

	s.merge()

	if !strings.Contains(s.note, "cannot open rubbish.pdf") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := s.doc.PageCount(); got != 1 {
		t.Errorf("a refused merge left the document with %d pages", got)
	}
}

// TestMergingSeveralNeedsADocumentFirst: the picker is not even shown.
func TestMergingSeveralNeedsADocumentFirst(t *testing.T) {
	h := &fakeHost{}
	s := newState(surfaceW, surfaceH, h)

	s.merge()

	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("the status line says %q", s.note)
	}
	if h.asked != 0 {
		t.Errorf("the file chooser was shown %d times with nothing open", h.asked)
	}
}
