package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
)

// errNotDrawn stands in for whatever went wrong.
var errNotDrawn = errors.New("not drawn")

// mixedPDF writes a document whose pages carry ink or do not, so which ones a
// verb drops can be read rather than inferred.
func mixedPDF(t *testing.T, inked ...bool) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	kids := make(reader.Array, 0, len(inked))
	for _, ink := range inked {
		content := "" // a page with nothing on it at all
		if ink {
			content = "0 g 10 10 180 180 re f"
		}
		kids = append(kids, w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
			"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(content)})}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count":    reader.Integer(len(kids)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(200)}})
	out, err := w.Finish(reader.Dict{"Root": w.Add(reader.Dict{
		"Type": reader.Name("Catalog"), "Pages": pagesRef})})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheBlankPagesGo(t *testing.T) {
	// A duplex stack run through a single-sided feeder comes back with a blank
	// behind every one-sided sheet.
	h := &fakeHost{name: "scan.pdf", file: mixedPDF(t, true, false, true, false)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("the document did not open")
	}
	s.dropBlank()
	if s.doc.PageCount() != 2 {
		t.Fatalf("%d pages left, want 2: %q", s.doc.PageCount(), s.note)
	}
	if !strings.Contains(s.note, "dropped 2 blank page") {
		t.Errorf("it said %q", s.note)
	}
}

func TestADocumentWithNothingToDrop(t *testing.T) {
	h := &fakeHost{name: "scan.pdf", file: mixedPDF(t, true, true)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	s.dropBlank()
	if s.doc.PageCount() != 2 {
		t.Errorf("%d pages left", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "carries something") {
		t.Errorf("it said %q", s.note)
	}
}

func TestADocumentThatIsAllBlank(t *testing.T) {
	// A document needs a page, so this refuses rather than leaving none.
	h := &fakeHost{name: "scan.pdf", file: mixedPDF(t, false, false)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	s.dropBlank()
	if s.doc.PageCount() != 2 {
		t.Errorf("%d pages left", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "needs one") {
		t.Errorf("it said %q", s.note)
	}
}

func TestAPageThatCannotBeDrawnIsNotBlank(t *testing.T) {
	// Not being able to see a page is not evidence that there is nothing on
	// it, and deleting on that footing would throw away exactly the pages this
	// program had most trouble with.
	was := drawPage
	t.Cleanup(func() { drawPage = was })
	drawPage = func(*reader.Document, int, render.Options) (*raster.Image, error) {
		return nil, errNotDrawn
	}
	h := &fakeHost{name: "scan.pdf", file: mixedPDF(t, true, false)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	s.dropBlank()
	if s.doc.PageCount() != 2 {
		t.Errorf("%d pages left; a page nobody could draw was dropped", s.doc.PageCount())
	}
}

func TestNothingOpenToLookAt(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.dropBlank()
	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("it said %q", s.note)
	}
}

func TestADocumentThatCannotBeWrittenBack(t *testing.T) {
	h := &fakeHost{name: "scan.pdf", file: mixedPDF(t, true, false)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errNotDrawn }
	s.dropBlank()
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("it said %q", s.note)
	}
}

func TestHowPageNumbersAreWritten(t *testing.T) {
	// A document whose every other page is blank must not produce a range as
	// long as itself.
	for _, tc := range []struct {
		in   []int
		want string
	}{
		{nil, ""},
		{[]int{3}, "3"},
		{[]int{1, 2, 3}, "1-3"},
		{[]int{1, 3, 5}, "1,3,5"},
		{[]int{1, 2, 5, 6, 7, 9}, "1-2,5-7,9"},
	} {
		if got := rangeOf(tc.in); got != tc.want {
			t.Errorf("rangeOf(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
