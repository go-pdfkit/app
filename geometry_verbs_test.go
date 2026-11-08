package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// pagesOfDoc is how many pages the document the workbench would save has.
func pagesOfDoc(t *testing.T, s *state) int {
	t.Helper()
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	return src.PageCount()
}

// ⛔⛔ THE POINT OF THE WHOLE BATCH: these four verbs were in `ops`, `ops` was
// linked into the workbench, and none of them was on the ribbon. So each test
// here is first a test that the control exists and reaches the library at all.
func TestCuttingEachPageUpMakesMorePages(t *testing.T) {
	s := shown(t, samplePDF(t, 2))
	s.tools.cutAcross.Set(2)
	s.tools.cutDown.Set(1)
	s.divide()
	if got := pagesOfDoc(t, s); got != 4 {
		t.Errorf("cutting 2 pages into 2 × 1 gave %d pages, want 4", got)
	}
	if !strings.Contains(s.note, "cut each page into 2 × 1") {
		t.Errorf("the status line says %q", s.note)
	}
}

// A cut into one piece is not a cut, and saying so beats writing the document
// out unchanged and calling it done.
func TestCuttingIntoOnePieceIsRefused(t *testing.T) {
	s := shown(t, samplePDF(t, 2))
	for _, c := range [][2]int{{1, 1}, {0, 2}, {2, 0}} {
		s.tools.cutAcross.Set(c[0])
		s.tools.cutDown.Set(c[1])
		s.divide()
		if !strings.Contains(s.note, "at least two pieces") {
			t.Errorf("%d × %d says %q", c[0], c[1], s.note)
		}
		if got := pagesOfDoc(t, s); got != 2 {
			t.Fatalf("%d × %d changed the document to %d pages", c[0], c[1], got)
		}
	}
}

func TestPuttingEveryPageOnOneLeavesOnePage(t *testing.T) {
	s := shown(t, samplePDF(t, 3))
	s.onePage()
	if got := pagesOfDoc(t, s); got != 1 {
		t.Errorf("three pages onto one gave %d pages", got)
	}
	if !strings.Contains(s.note, "every page onto one") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔ A POSTER IS NOT A CUT, and the panel keeps the two sets of numbers apart.
// This is the control for that: tiling is driven by its own pair, so a cut set
// to 1 × 1 does not stop a 2 × 2 poster.
func TestAPosterSpreadsAPageOverSheets(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.cutAcross.Set(1)
	s.tools.cutDown.Set(1)
	s.tools.tilesAcross.Set(2)
	s.tools.tilesDown.Set(2)
	s.poster()
	if got := pagesOfDoc(t, s); got != 4 {
		t.Errorf("a 2 × 2 poster of one page gave %d sheets, want 4", got)
	}
	if !strings.Contains(s.note, "over 2 × 2 sheets") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestAPosterOfOneTileIsRefused(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.tilesAcross.Set(1)
	s.tools.tilesDown.Set(1)
	s.poster()
	if !strings.Contains(s.note, "at least two tiles") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := pagesOfDoc(t, s); got != 1 {
		t.Errorf("a refused poster changed the document to %d pages", got)
	}
}

// ⛔⛔ DESKEW SAYS WHAT IT MEASURED, and that is the whole value of it. A page
// already level is left alone deliberately — turning it by a hundredth of a
// degree would cost it a transformation it did not need — so "done" would leave
// a reader wondering whether the verb had worked at all. It has to distinguish
// "measured and left alone" from "not measured".
func TestStraighteningSaysWhatItMeasured(t *testing.T) {
	s := shown(t, samplePDF(t, 2))
	s.deskew()
	if !strings.Contains(s.note, "measured 2 pages") {
		t.Errorf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "already level") {
		t.Errorf("a straight scan was not reported as level: %q", s.note)
	}
}

// One page is one page, not "1 pages".
func TestStraighteningOnePageSaysPageNotPages(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.deskew()
	if strings.Contains(s.note, "1 pages") {
		t.Errorf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "measured 1 page") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔⛔ AND A CROOKED PAGE IS ACTUALLY STRAIGHTENED, which is the half of the
// verb a level scan cannot show. The fixture draws its bar through a rotation
// matrix of five degrees, so the tilt is in the PAGE rather than in the
// fixture's description of it: a test that asserted "straightened" against a
// level page would be asserting the message and not the work.
//
// The second page says it is no size at all, so it cannot be measured — which
// is a third answer, distinct from "tilted" and from "level", and the status
// line has to carry it. A page that defeats the measurement is no reason to
// leave the other one crooked.
func TestStraighteningReportsWhatItTurnedAndWhatItCouldNot(t *testing.T) {
	s := shown(t, tiltedAndDegeneratePDF(t))
	s.deskew()
	if !strings.Contains(s.note, "straightened") {
		t.Errorf("a five-degree tilt was not straightened: %q", s.note)
	}
	if !strings.Contains(s.note, "could not be measured") {
		t.Errorf("the page with no size was not reported: %q", s.note)
	}
}

// tiltedAndDegeneratePDF is two pages: a bar drawn five degrees off level, and
// a page that declares no size at all.
func tiltedAndDegeneratePDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	// cos 5° and sin 5°, so the bar is drawn turned rather than described as
	// turned.
	tilted := w.Add(&reader.Stream{Dict: reader.Dict{},
		Raw: []byte("0 g 0.9962 0.0872 -0.0872 0.9962 60 200 cm 0 0 300 60 re f")})
	page1 := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": tilted,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(500), reader.Integer(500)},
	})
	page2 := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")}),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(0), reader.Integer(0)},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page1, page2}, "Count": reader.Integer(2)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔ With nothing open, each of the four says so and stops. They all go through
// changeSaying, which refuses a document that is not there — and the verb has
// to return rather than go on to move the page or report a measurement it
// never made.
func TestTheGeometryVerbsNeedADocument(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"cut up", (*state).divide},
		{"on one page", (*state).onePage},
		{"poster", (*state).poster},
		{"straighten", (*state).deskew},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newState(surfaceW, surfaceH, &fakeHost{})
			s.tools.cutAcross.Set(2)
			s.tools.tilesAcross.Set(2)
			c.run(s)
			if !strings.Contains(s.note, "open a document first") {
				t.Errorf("the status line says %q", s.note)
			}
		})
	}
}
