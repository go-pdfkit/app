package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// ⛔ THE PAPER IS NOT THE CROP BOX, and this is the control for it. Cropping
// cuts a page down to part of itself; setting the paper says which sheet it is
// on. A page cropped to a postcard on A4 paper is still on A4, so the two are
// separate verbs with separate rows.
func TestSettingThePaperChangesTheSheetAndNotTheDrawing(t *testing.T) {
	s := shown(t, samplePDF(t, 2))
	s.tools.spec.Set("1")
	s.tools.paper.Set(0) // A4
	s.setPaper()
	if !strings.Contains(s.note, "A4") {
		t.Fatalf("the status line says %q", s.note)
	}
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	// ⛔ 595.276 × 841.89, not 595 × 842. A4 is 210 × 297 mm at 72 points to
	// the inch, and rounding it is a tenth of a millimetre that shows as a
	// hairline against a trimmed edge.
	page, _ := src.Page(1)
	w, h := pageSize(src, page)
	if w < 595.2 || w > 595.3 || h < 841.8 || h > 841.9 {
		t.Errorf("page 1 is %g × %g points, want A4 at 595.276 × 841.89", w, h)
	}
	// And the page it did not name is untouched.
	other, _ := src.Page(2)
	ow, oh := pageSize(src, other)
	if ow > 595 && oh > 841 {
		t.Errorf("page 2 was resized too: %g × %g", ow, oh)
	}
}

// Every paper on the list reaches the verb, and each is a different sheet.
func TestEveryPaperOnTheListIsASheet(t *testing.T) {
	seen := map[[2]float64]string{}
	for i, p := range papers {
		if p.w <= 0 || p.h <= 0 {
			t.Errorf("%q is %g × %g", p.name, p.w, p.h)
		}
		if was, ok := seen[[2]float64{p.w, p.h}]; ok {
			t.Errorf("%q is the same sheet as %q", p.name, was)
		}
		seen[[2]float64{p.w, p.h}] = p.name
		if names := paperNames(); names[i] != p.name {
			t.Errorf("the chooser's %d%s entry is %q and the table's is %q",
				i, "th", names[i], p.name)
		}
	}
}

// A chooser out of step with its table picks the first paper rather than
// reading past the end of it.
func TestAPaperOutsideTheListFallsBackToTheFirst(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	for _, at := range []int{-1, len(papers)} {
		s.tools.paper.Set(at)
		s.setPaper()
		if !strings.Contains(s.note, papers[0].name) {
			t.Errorf("a chooser at %d says %q", at, s.note)
		}
	}
}

// ⛔⛔ PAINTING THE TEXT SAYS WHAT IT COULD NOT REACH. ops.Doc.RecolourText
// only reaches the text in each page's own content stream: text drawn through
// a Form XObject is shared with every page that draws it, so recolouring one
// would change pages nobody named. The library counts what it stepped over,
// and a workbench that dropped that count would hand back a document that
// looks unchanged with nothing said — which is worse than refusing.
func TestPaintingTheTextSaysWhatItCouldNotReach(t *testing.T) {
	s := shown(t, textInAFormPDF(t))
	s.tools.spec.Set("all")
	s.tools.ink.Set(2) // red
	s.recolourText()
	if !strings.Contains(s.note, "red") {
		t.Fatalf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "left as they were") {
		t.Errorf("the text inside the form was not reported: %q", s.note)
	}
	if !strings.Contains(s.note, "inside a form") {
		t.Errorf("the reason was not given: %q", s.note)
	}
}

// A page whose text this can reach is painted with nothing left over, and the
// status line does not invent a caveat.
func TestPaintingAPlainPageLeavesNothingBehind(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	s.tools.spec.Set("all")
	s.tools.ink.Set(0) // black
	s.recolourText()
	if !strings.Contains(s.note, "black") {
		t.Fatalf("the status line says %q", s.note)
	}
	if strings.Contains(s.note, "left as they were") {
		t.Errorf("a page with no form reported unreached text: %q", s.note)
	}
}

// Every ink on the list is a colour the verb will take, and each is distinct.
func TestEveryInkOnTheListIsAColour(t *testing.T) {
	seen := map[[3]float64]string{}
	for i, ink := range inks {
		for _, v := range []float64{ink.r, ink.g, ink.b} {
			if v < 0 || v > 1 {
				t.Errorf("%q has a component of %g, and ops takes 0 to 1", ink.name, v)
			}
		}
		key := [3]float64{ink.r, ink.g, ink.b}
		if was, ok := seen[key]; ok {
			t.Errorf("%q is the same colour as %q", ink.name, was)
		}
		seen[key] = ink.name
		if names := inkNames(); names[i] != ink.name {
			t.Errorf("the chooser's entry %d is %q and the table's is %q",
				i, names[i], ink.name)
		}
	}
}

func TestAnInkOutsideTheListFallsBackToTheFirst(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	for _, at := range []int{-1, len(inks)} {
		s.tools.ink.Set(at)
		s.recolourText()
		if !strings.Contains(s.note, inks[0].name) {
			t.Errorf("a chooser at %d says %q", at, s.note)
		}
	}
}

// Both verbs need a document, and say so.
func TestThePaperAndInkVerbsNeedADocument(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"the paper", (*state).setPaper},
		{"the ink", (*state).recolourText},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newState(surfaceW, surfaceH, &fakeHost{})
			c.run(s)
			if !strings.Contains(s.note, "open a document first") {
				t.Errorf("the status line says %q", s.note)
			}
		})
	}
}

// textInAFormPDF draws one line on the page itself and one through a Form
// XObject, which is the shape that makes UnreachedText non-zero.
func textInAFormPDF(t *testing.T) []byte {
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
	res := reader.Dict{"Font": reader.Dict{"F1": font}}
	form := w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Form"),
		"BBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)},
		"Resources": res,
	}, Raw: []byte("BT /F1 10 Tf 20 60 Td (shared with other pages) Tj ET")})
	res["XObject"] = reader.Dict{"X1": form}
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{},
			Raw: []byte("BT /F1 10 Tf 20 300 Td (on the page itself) Tj ET q /X1 Do Q")}),
		"Resources": res,
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
