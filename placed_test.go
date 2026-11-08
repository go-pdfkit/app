package main

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/codec"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/render"
)

// redPNG is a picture nothing else on the page is: the sample document draws in
// black on white, so a red pixel in the rendering can only have come from here.
func redPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := raster.New(w, h)
	for i := 0; i < w*h; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = 255, 0, 0, 255
	}
	var buf bytes.Buffer
	if err := codec.Encode(&buf, img, codec.PNG); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// redSpan is how wide and how tall the red part of a rendered page is, in
// pixels, and how many pixels of it there are.
func redSpan(img *raster.Image) (wide, tall, count int) {
	minX, maxX, minY, maxY := img.W, -1, img.H, -1
	for y := 0; y < img.H; y++ {
		for x := 0; x < img.W; x++ {
			i := img.PixOffset(x, y)
			r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]
			if r < 200 || g > 80 || b > 80 {
				continue
			}
			count++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	return maxX - minX + 1, maxY - minY + 1, count
}

// ⛔ THE POINT OF THE VERB IS MEASURED IN PIXELS, not in the status line. A
// placed picture that reached the file at the wrong size would say exactly the
// same thing it says now, and the one number a person chose — the width — is
// the one a test can check by looking at the page.
func TestAPlacedPictureIsDrawnAtTheWidthThatWasAskedFor(t *testing.T) {
	s, h := opened(t, 1)
	h.file = redPNG(t, 100, 50)
	s.tools.wide.Set(144)
	s.placePicture()
	if !strings.Contains(s.note, "drew") {
		t.Fatalf("the status line says %q", s.note)
	}
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	img, err := render.Page(src, 1, render.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	wide, tall, count := redSpan(img)
	if count == 0 {
		t.Fatal("nothing red was drawn on the page")
	}
	// At scale 1 a point is a pixel. One pixel either way is the rasteriser's
	// edge, not a disagreement about the size.
	if wide < 143 || wide > 145 {
		t.Errorf("the picture came out %d points wide, not the 144 asked for", wide)
	}
	// ⛔ AND THE PROPORTIONS ARE KEPT, which is what Place promises when only a
	// width is given: 100 by 50 at 144 wide is 72 tall. A verb that stretched
	// the picture to a square would pass the width check alone.
	if tall < 71 || tall > 73 {
		t.Errorf("the picture came out %d points tall, not the 72 its shape asks for", tall)
	}
}

func TestAPlacedPictureFollowsTheWidthBeside(t *testing.T) {
	// The control for the test above: another width, another span. Without this
	// a hard-coded 144 anywhere in the chain would pass.
	s, h := opened(t, 1)
	h.file = redPNG(t, 100, 50)
	s.tools.wide.Set(72)
	s.placePicture()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	img, err := render.Page(src, 1, render.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	wide, _, count := redSpan(img)
	if count == 0 {
		t.Fatal("nothing red was drawn on the page")
	}
	if wide < 71 || wide > 73 {
		t.Errorf("the picture came out %d points wide, not the 72 asked for", wide)
	}
}

// ⛔ A chooser holding a number no place answers to lands in the middle, which
// is what the stamp beside it does with the same number. The index comes from a
// control, and a control that has been rebuilt with a shorter list — or a saved
// panel from an older build — can hand over one that is past the end; falling
// through to places[0] by accident would index out of range and take the whole
// workbench down.
func TestPlacingAPictureSomewhereThatIsNotAPlaceLandsInTheMiddle(t *testing.T) {
	s, h := opened(t, 1)
	h.file = redPNG(t, 40, 20)
	s.tools.at.Set(len(places) + 7)
	s.placePicture()
	if !strings.Contains(s.note, places[0].name) {
		t.Errorf("the status line says %q, not %q", s.note, places[0].name)
	}
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	img, err := render.Page(src, 1, render.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, count := redSpan(img); count == 0 {
		t.Error("nothing was drawn on the page")
	}
}

func TestPlacingAPictureWithNothingOpenSaysSo(t *testing.T) {
	h := &fakeHost{}
	s := newState(surfaceW, surfaceH, h)
	s.placePicture()
	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("the status line says %q", s.note)
	}
	if h.asked != 0 {
		t.Error("the file dialog was opened with nothing to draw on")
	}
}

func TestPlacingSomethingThatIsNotAPictureSaysSo(t *testing.T) {
	// A PDF is in the dialog's list because Open takes one. Place cannot draw
	// one, and what reaches the status line is why rather than silence.
	s, h := opened(t, 1)
	h.file = samplePDF(t, 1)
	s.placePicture()
	if s.note == "" || strings.Contains(s.note, "drew") {
		t.Errorf("the status line says %q", s.note)
	}
}

// ⛔⛔ THE SECOND FILE IS WHATEVER THE FIRST MAY BE. One dialog, one filter
// list: the moment Open accepted a picture, this chooser offered one too, and
// refusing it here would be a file the dialog showed and the code would not
// take — the second direction of the gate in opened_test.go.
func TestASecondFileMayBeAPictureOrADrawingOrAnArchive(t *testing.T) {
	for _, c := range []struct {
		name  string
		file  func(t *testing.T) []byte
		pages int
	}{
		{"a picture", func(t *testing.T) []byte { return redPNG(t, 40, 20) }, 3},
		{"a drawing", func(t *testing.T) []byte { return []byte(vectorSVG) }, 3},
		{"an archive", func(t *testing.T) []byte {
			return comicArchive(t, "page1.png", "page2.png")
		}, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, h := opened(t, 2)
			h.file = c.file(t)
			s.merge()
			if s.doc == nil {
				t.Fatalf("the merge lost the document: %q", s.note)
			}
			if got := s.doc.PageCount(); got != c.pages {
				t.Errorf("the document has %d pages, not %d: %q",
					got, c.pages, s.note)
			}
		})
	}
}

func TestASecondFileNothingCanReadSaysSo(t *testing.T) {
	s, h := opened(t, 2)
	h.file = []byte("just some words")
	s.merge()
	if !strings.Contains(s.note, "cannot open") {
		t.Errorf("the status line says %q", s.note)
	}
	if s.doc.PageCount() != 2 {
		t.Errorf("the document has %d pages, so something was joined to it",
			s.doc.PageCount())
	}
}

func TestASecondFileRecognisedAndRefusedSaysWhichArmRefusedIt(t *testing.T) {
	// A .docx is a zip, so the archive arm takes it and then finds no pictures.
	// Its own sentence is what reaches the status line, not "cannot open".
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("<w:document/>")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	s, h := opened(t, 2)
	h.file = buf.Bytes()
	s.merge()
	if !strings.Contains(s.note, "no pictures in the archive") {
		t.Errorf("the status line says %q", s.note)
	}
	if strings.Contains(s.note, "cannot open") {
		t.Errorf("the arm's own reason was replaced by the reader's: %q", s.note)
	}
	if s.doc.PageCount() != 2 {
		t.Errorf("the document has %d pages", s.doc.PageCount())
	}
}
