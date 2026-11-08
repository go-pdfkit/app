package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"image/color"
	"io"
	"strings"
	"testing"

	"github.com/go-pdfkit/convert"
	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
)

// litPage renders the first page of the document the workbench would save and
// returns how light it is on average, and whether every pixel is grey.
//
// ⛔ PIXELS, because every verb in this group is a pixel operation and nothing
// else can tell whether it happened. A status line saying "turned the pages
// inside out" is what a verb that did nothing also says.
func litPage(t *testing.T, s *state) (light float64, allGrey bool) {
	t.Helper()
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	img, err := render.Page(src, 1, render.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	allGrey = true
	sum := 0.0
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
		if r != g || g != b {
			allGrey = false
		}
		sum += float64(r) + float64(g) + float64(b)
	}
	return sum / float64(len(img.Pix)/4*3) / 255, allGrey
}

// ⛔⛔ TURNING THE PAGES INSIDE OUT ACTUALLY INVERTS THEM. The fixture is
// mostly white paper with a black square, so inverting it must make the
// average DARK — and a verb that only wrote a message would leave it light.
func TestTurningThePagesInsideOutInvertsThePixels(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	before, _ := litPage(t, s)
	if before < 0.5 {
		t.Fatalf("the fixture is already dark (%.2f), so inverting proves nothing", before)
	}
	s.invert()
	after, _ := litPage(t, s)
	if after >= before {
		t.Errorf("inverting left the page at %.2f light, from %.2f", after, before)
	}
	if after > 0.5 {
		t.Errorf("the inverted page is still mostly light: %.2f", after)
	}
}

// ⛔⛔ AND THE WARNING IS TRUE: after a redraw the text is gone, because the
// page is a picture. This is the test that keeps the tip honest — if a future
// convert kept the text layer, the tip would be the thing that is wrong, and
// this would say so.
func TestRedrawingTurnsTheTextIntoPicturesAndSaysSo(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	was, err := extract.Text(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(was, "the first chapter") {
		t.Fatalf("the fixture says %q, so this measures nothing", was)
	}

	s.greyscale()
	if !strings.Contains(s.note, "the pages are pictures now") {
		t.Errorf("the status line does not warn: %q", s.note)
	}
	if !strings.Contains(s.note, "cannot be selected, searched or read out") {
		t.Errorf("the status line does not say what was lost: %q", s.note)
	}
	out, err = s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err = reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	now, err := extract.Text(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(now, "chapter") {
		t.Errorf("the page still reads back as %q, so the warning is wrong", now)
	}
}

// Dropping the colour leaves every pixel grey.
func TestDroppingTheColourLeavesEveryPixelGrey(t *testing.T) {
	s := shown(t, colouredPDF(t))
	if _, grey := litPage(t, s); grey {
		t.Fatal("the fixture is already grey, so this measures nothing")
	}
	s.greyscale()
	if _, grey := litPage(t, s); !grey {
		t.Error("a pixel came back with unequal channels after dropping the colour")
	}
}

// ⛔⛔ THE GROUND COLOUR REACHES THE PAPER, which is the one thing convert's
// own documentation warns about: "A first version painted the colour under the
// finished raster — which does nothing at all, because the renderer has
// already filled the page with opaque white". The option was documented,
// shipped, and had no effect; a screenshot found it. So this asks the pixels.
func TestDrawingThePagesOnAColourReachesThePaper(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.ground.Set(3) // black, for the night
	s.onGround()
	if !strings.Contains(s.note, "black") {
		t.Fatalf("the status line says %q", s.note)
	}
	light, _ := litPage(t, s)
	if light > 0.5 {
		t.Errorf("the page drawn on black is %.2f light", light)
	}
}

// Leaving it white is not a redrawing, and saying so beats rasterising a
// document for no change at all.
func TestDrawingThemOnNothingIsRefused(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.ground.Set(0) // leave it white
	s.onGround()
	if !strings.Contains(s.note, "choose a colour") {
		t.Errorf("the status line says %q", s.note)
	}
	if _, err := extract.Text(s.src, 1); err != nil {
		t.Errorf("the document was redrawn anyway: %v", err)
	}
}

// A ground outside the list falls back to the first, which is "leave it
// white" — so it refuses rather than painting something nobody chose.
func TestAGroundOutsideTheListIsRefused(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	for _, at := range []int{-1, len(grounds)} {
		s.tools.ground.Set(at)
		s.onGround()
		if !strings.Contains(s.note, "choose a colour") {
			t.Errorf("a chooser at %d says %q", at, s.note)
		}
	}
}

// ⛔ BOTH AT NONE IS REFUSED, because convert treats a contrast of 0 as 1 and
// a brightness of 0 as none — so the call would rasterise every page for no
// change whatever, which is the most expensive way to do nothing.
func TestBrighteningByNothingIsRefused(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	s.tools.bright.Set(0)
	s.tools.contrast.Set(100)
	s.adjust()
	if !strings.Contains(s.note, "both are at none") {
		t.Errorf("the status line says %q", s.note)
	}
}

// Brightening makes the page lighter, and the panel's percentages reach
// convert's own scale.
func TestBrighteningMakesThePageLighter(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	before, _ := litPage(t, s)
	s.tools.bright.Set(60)
	s.tools.contrast.Set(100)
	s.adjust()
	if !strings.Contains(s.note, "60% brightness") {
		t.Errorf("the status line says %q", s.note)
	}
	after, _ := litPage(t, s)
	if after <= before {
		t.Errorf("brightening left the page at %.2f, from %.2f", after, before)
	}
}

// ⛔ A ZERO CONTRAST IS SENT AS ONE, never as zero: convert says a zero there
// "would turn every page into a flat grey rectangle for every caller who set
// only the other fields". The panel can be spun to zero, so this is the guard
// that keeps it off the wire.
func TestAZeroContrastIsNotSentAsZero(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	var sent convert.RedrawOptions
	was := redrawPDF
	redrawPDF = func(pdf []byte, opt convert.RedrawOptions) ([]byte, error) {
		sent = opt
		return was(pdf, opt)
	}
	t.Cleanup(func() { redrawPDF = was })
	s.tools.bright.Set(20)
	s.tools.contrast.Set(0)
	s.adjust()
	if sent.Contrast != 1 {
		t.Errorf("a contrast of 0%% went out as %v", sent.Contrast)
	}
}

// The scanner effect is reproducible, which is convert's own promise and the
// reason its seed is fixed when it is zero: "two runs over the same file must
// produce the same bytes, or nothing downstream can be compared, cached or
// checksummed".
//
// ⛔⛔ THE PIXELS, NOT THE FILE. The first version of this compared
// doc.Bytes() of two runs and failed — and the grain was not the reason. A
// written PDF carries an information dictionary, and a date in it differs
// between two runs a moment apart, so the comparison was of two different
// moments rather than of two renderings. Comparing the whole file would make
// this test fail for every document the workbench ever writes twice, which is
// a test that reports the clock as a defect in the scanner.
func TestTheScannerEffectIsReproducible(t *testing.T) {
	pixels := func(s *state) []byte {
		t.Helper()
		out, err := s.doc.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		src, err := reader.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		img, err := render.Page(src, 1, render.Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		return img.Pix
	}
	first, second := shown(t, samplePDF(t, 1)), shown(t, samplePDF(t, 1))
	first.scanned()
	second.scanned()
	if !strings.Contains(first.note, "through a scanner") {
		t.Fatalf("the status line says %q", first.note)
	}
	a, b := pixels(first), pixels(second)
	if !bytes.Equal(a, b) {
		t.Error("two runs of the scanner effect over the same file drew different pages")
	}
	// And it did something: a scanned page is not the page that went in.
	plain := pixels(shown(t, samplePDF(t, 1)))
	if bytes.Equal(a, plain) {
		t.Error("the scanner effect left the page exactly as it was")
	}
}

// ⛔ A REAL .cbz: one picture per page in a zip, under the name a comic reader
// opens. The zip beside it holds the same pictures under .zip, which is the
// whole difference between a file somebody can use and one they must rename.
func TestTheDocumentIsHandedOverAsAComic(t *testing.T) {
	s, h := openedCarrier(t) // two pages
	s.asArchive()
	if !strings.HasSuffix(h.as, ".cbz") {
		t.Fatalf("it was handed over as %q", h.as)
	}
	zr, err := zip.NewReader(bytes.NewReader(h.saved), int64(len(h.saved)))
	if err != nil {
		t.Fatalf("what was handed over is not a zip: %v", err)
	}
	pages := 0
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			pages++
		}
	}
	if pages < 2 {
		t.Errorf("the archive holds %d file(s) for a two-page document", pages)
	}
}

// Every redrawing says so rather than handing back a document of blank pages,
// and every one needs a document to begin with.
func TestARedrawingThatFailsIsReported(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	was := redrawPDF
	redrawPDF = func([]byte, convert.RedrawOptions) ([]byte, error) {
		return nil, errors.New("the lamp is out")
	}
	t.Cleanup(func() { redrawPDF = was })
	s.invert()
	if !strings.Contains(s.note, "cannot be redrawn") {
		t.Errorf("the status line says %q", s.note)
	}
	// And the document is untouched: its text is still readable.
	if _, err := extract.Text(s.src, 1); err != nil {
		t.Errorf("the document was changed anyway: %v", err)
	}
}

// What comes back from a redrawing has to be readable as a document, and a
// workbench that could not read it must say so rather than keep the old one
// and claim success.
func TestARedrawingThatCannotBeReadBackIsReported(t *testing.T) {
	s := shown(t, samplePDF(t, 1))
	was := redrawPDF
	redrawPDF = func([]byte, convert.RedrawOptions) ([]byte, error) {
		return []byte("this is not a PDF"), nil
	}
	t.Cleanup(func() { redrawPDF = was })
	s.invert()
	if !strings.Contains(s.note, "cannot be read back") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestAnArchiveThatCannotBeWrittenIsReported(t *testing.T) {
	s, h := openedCarrier(t)
	was := archivePDF
	archivePDF = func(io.Writer, []byte, convert.RasterOptions, string) error {
		return errors.New("the press jammed")
	}
	t.Cleanup(func() { archivePDF = was })
	s.asArchive()
	if h.as != "" {
		t.Errorf("%q was handed over anyway", h.as)
	}
	if !strings.Contains(s.note, "cannot be written as an archive") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔ A document that cannot be written back cannot be redrawn either, and for
// the same reason the readings give: convert.Redraw takes a whole PDF, so the
// document has to be written out before it can be handed over. If that write
// fails there is nothing to redraw, and the verb says so rather than keeping
// the old pages and claiming it redrew them.
func TestADocumentThatCannotBeWrittenBackIsNotRedrawn(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"redrawing", (*state).invert},
		{"an archive", (*state).asArchive},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, h := openedCarrier(t)
			was := docBytes
			t.Cleanup(func() { docBytes = was })
			docBytes = func(*ops.Doc) ([]byte, error) {
				return nil, errors.New("the ink ran out")
			}
			c.run(s)
			if h.as != "" {
				t.Errorf("%q was handed over anyway", h.as)
			}
			if !strings.Contains(s.note, "cannot be written") {
				t.Errorf("the status line says %q", s.note)
			}
		})
	}
}

func TestTheRedrawVerbsNeedADocument(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"invert", (*state).invert},
		{"greyscale", (*state).greyscale},
		{"on a colour", (*state).onGround},
		{"brighten", (*state).adjust},
		{"scanned", (*state).scanned},
		{"as a comic", (*state).asArchive},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newState(surfaceW, surfaceH, &fakeHost{})
			s.tools.ground.Set(1)
			s.tools.bright.Set(20)
			c.run(s)
			if !strings.Contains(s.note, "open a document first") {
				t.Errorf("the status line says %q", s.note)
			}
		})
	}
}

// Every ground on the list is a colour, and each is distinct.
func TestEveryGroundOnTheListIsAColour(t *testing.T) {
	seen := map[color.RGBA]string{}
	for i, g := range grounds {
		if names := groundNames(); names[i] != g.name {
			t.Errorf("the chooser's entry %d is %q and the table's is %q",
				i, names[i], g.name)
		}
		if g.rgba == nil {
			continue
		}
		if was, ok := seen[*g.rgba]; ok {
			t.Errorf("%q is the same colour as %q", g.name, was)
		}
		seen[*g.rgba] = g.name
	}
	if grounds[0].rgba != nil {
		t.Error("the first ground is meant to be the one that changes nothing")
	}
}

// colouredPDF is one page with three coloured squares, so dropping the colour
// has something to drop.
func colouredPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
		"1 0 0 rg 10 10 60 60 re f " +
			"0 1 0 rg 80 10 60 60 re f " +
			"0 0 1 rg 150 10 60 60 re f")})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(220), reader.Integer(80)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
