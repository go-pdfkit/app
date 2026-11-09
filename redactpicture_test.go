package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

// ⛔⛔ A LINE OVER A PICTURE IS REDACTED, AND THE PICTURE'S OWN SAMPLES GO WITH
// IT. Until ops v0.35.0 this was the refusal a reader of scanned paper met on
// every selection — a scanned page is one picture, so "across an image" was
// everywhere — and the only thing left to do with it was the black rectangle
// this verb exists to stop them doing. ISO 32000-1 12.5.6.23 is explicit that a
// clip or a mask will not do: the data has to be destroyed.
//
// ⛔ What this asks is the SAMPLES READ BACK OUT OF THE SAVED FILE, not the
// screen. A redaction paints the region afterwards, so a rendering of the
// result looks the same whether the pixels under the paint were destroyed or
// merely covered — which is the whole defect, and why a pixel test here would
// measure nothing. The picture's rows each carry their own grey, so the rows
// that must go and the rows that must stay are told apart by value.
//
// The row-by-row proof, and the walk over every object that catches an original
// picture left behind in the file, are go-pdfkit/ops' gate. What is asked here
// is what the workbench adds: that the selection a reader dragged reaches the
// picture at all, and that no refusal arrives in the status line in place of
// the redaction.
func TestRedactingALineOverAPictureDestroysTheSamplesUnderIt(t *testing.T) {
	// ⛔ TWO ROWS, forty points each, with the line of text in the middle of the
	// lower one. A finer grid would make this a test of where exactly a line's
	// box ends — the ascent of a font with no descriptor, rounded up to the next
	// whole sample — which is ops' arithmetic and has its own tests there. What
	// is asked here is coarser and is the thing that used to be impossible: the
	// half of the picture the words sit on is destroyed and the other half is
	// not.
	const rows = 2
	s := shown(t, textOverAPicturePDF(t, rows))
	selectFirstLine(t, s)
	s.redactSelection()
	if !strings.Contains(s.note, "removed") {
		t.Fatalf("the status line says %q, which is not a redaction", s.note)
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
	if len(im.Data) != rows {
		t.Fatalf("the picture holds %d sample(s), want %d", len(im.Data), rows)
	}
	// The picture covers y 280..360, so row 0 is y 320..360 and row 1 is
	// y 280..320. The line is drawn at y 300: the lower row is the one the words
	// are on, and the upper one is forty points away from them.
	if im.Data[1] != 0 {
		t.Errorf("the row the words are drawn on came back %d: it went in as %d, so "+
			"the picture was covered rather than changed", im.Data[1], 20)
	}
	if im.Data[0] != 10 {
		t.Errorf("the row above the words came back %d rather than 10: this destroyed "+
			"more of the picture than was asked for", im.Data[0])
	}
}

// textOverAPicturePDF is a page with a line of text drawn across a picture of
// `rows` rows, each a different grey, filling y 280..360 — the shape of a
// scanned page, where the words a reader selects are over a picture rather than
// beside one.
//
// ⛔ The samples go in UNFILTERED, so that what comes back out can be compared
// with what went in without this test carrying a decoder of its own.
func textOverAPicturePDF(t *testing.T, rows int) []byte {
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
	samples := make([]byte, 0, rows)
	for row := range rows {
		samples = append(samples, byte(10*(row+1)))
	}
	pic := w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(1), "Height": reader.Integer(rows),
		"ColorSpace": reader.Name("DeviceGray"), "BitsPerComponent": reader.Integer(8),
	}, Raw: samples})
	var body strings.Builder
	// The picture first and the words over it, as a stamp on a scan would be.
	// Which was drawn first does not change where the region lands in the
	// picture's samples, which is what this is about.
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
	return out
}
