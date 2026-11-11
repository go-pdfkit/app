package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

// ⛔⛔ A REAL SCAN IS A JPEG, AND THAT WAS THE REFUSAL A READER OF SCANNED PAPER
// STILL MET. The picture in the test beside this one goes in unfiltered, which
// no scanner produces: every one of them writes DCTDecode. So "redacting a
// scanned page works" was true of a page no reader has, and the reader who
// opened their own scan got the refusal and the black rectangle this verb
// exists to stop them drawing. ops v0.38.0 destroys those pixels too.
//
// ⛔ WHAT IS KEPT IS COMPARED AGAINST THIS TEST'S OWN DECODE OF THE JPEG, not
// against the greys that went in. A JPEG does not give back what it was given,
// so asserting the input values would be asserting the encoder; and asserting
// "near enough" would pass for a picture that came back re-encoded into mud,
// which is exactly what a redaction must not do to the part nobody selected.
func TestRedactingALineOverAScanDestroysTheSamplesUnderIt(t *testing.T) {
	// ⛔ ONE COLUMN, like the picture in the test beside this one. A wider
	// grid makes this a test of WHERE THE WORDS END: the line runs from x 20
	// to about x 106 and the picture covers x 10 to 290, so with eight columns
	// three go and five stay — correct, and ops' arithmetic, which has its own
	// tests there. What is new here is the JPEG, and a column count is a way
	// of being wrong about something else.
	const rows, cols = 2, 1
	file, raw := textOverAScanPDF(t, cols, rows)

	want, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	s := shown(t, file)
	selectFirstLine(t, s)
	s.redactSelection()
	if !strings.Contains(s.note, "removed") {
		t.Fatalf("the status line says %q, which is not a redaction — a scanned page is the "+
			"case this verb is for", s.note)
	}
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	src, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if text, err := extract.Text(src, 1); err != nil {
		t.Fatal(err)
	} else if strings.Contains(text, "chapter") {
		t.Errorf("the saved page still says %q", text)
	}
	images, err := extract.Images(src, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 {
		t.Fatalf("the saved page draws %d picture(s), want 1", len(images))
	}
	im := images[0]
	if im.Filter == "DCTDecode" {
		t.Fatal("the picture came back a JPEG: its pixels were covered rather than destroyed")
	}
	if len(im.Data) != cols*rows {
		t.Fatalf("the picture holds %d sample(s), want %d", len(im.Data), cols*rows)
	}
	// The picture covers y 280..360, so row 0 is y 320..360 and row 1 is
	// y 280..320. The line is drawn at y 300: the lower row is the one the
	// words are on, and the upper one is forty points away from them.
	for x := range cols {
		if got := im.Data[cols+x]; got != 0 {
			t.Errorf("column %d of the row the words are drawn on came back %d: the picture "+
				"was covered rather than changed", x, got)
		}
		g, _, _, _ := want.At(x, 0).RGBA()
		if got, was := im.Data[x], byte(g>>8); got != was {
			t.Errorf("column %d of the row above the words came back %d where a reader of the "+
				"scan would have seen %d: this changed the pixels nobody selected", x, got, was)
		}
	}
}

// textOverAScanPDF is textOverAPicturePDF's twin with the picture encoded as a
// JPEG, which is the shape every scanner actually produces. It returns the
// file and the JPEG, so that the test can decode the original itself.
func textOverAScanPDF(t *testing.T, cols, rows int) (file, raw []byte) {
	t.Helper()
	grid := image.NewGray(image.Rect(0, 0, cols, rows))
	for y := range rows {
		for x := range cols {
			grid.SetGray(x, y, color.Gray{Y: uint8(40 * (y + 1))})
		}
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, grid, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	raw = enc.Bytes()

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
	pic := w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(cols), "Height": reader.Integer(rows),
		"ColorSpace": reader.Name("DeviceGray"), "BitsPerComponent": reader.Integer(8),
		"Filter": reader.Name("DCTDecode"),
	}, Raw: raw})
	var body strings.Builder
	body.WriteString("q 280 0 0 80 10 280 cm /P0 Do Q\n")
	x := 20.0
	for _, ch := range "the first chapter" {
		if ch == ' ' {
			x += 3.3 // the pen moves; nothing is drawn
			continue
		}
		fmt.Fprintf(&body, "BT /F1 10 Tf %.2f 300 Td (%c) Tj ET\n", x, ch)
		x += 5
	}
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body.String())})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
		"Resources": reader.Dict{
			"Font":    reader.Dict{"F1": font},
			"XObject": reader.Dict{"P0": pic},
		},
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
	return out, raw
}
