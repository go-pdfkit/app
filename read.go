// The Read group: what the page says and what it carries, as against what it
// looks like.
//
// Nothing here changes the document, so nothing here can break the promise the
// rest of the workbench makes. It keeps it all the same: the text and the
// pictures are read out of the document written and read back, not out of the
// one being assembled — so what is listed is what would come out of Save.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// The two readings on offer, and the picture of the page they replace.
const (
	readingText   = "text"
	readingImages = "images"
)

// readGroup is the panel.
func (s *state) readGroup() *column {
	box := newColumn()
	box.add(button("What this page says", toolkit.ButtonDefault,
		func() { s.read(readingText) }), bareH)
	box.add(button("What this page carries", toolkit.ButtonDefault,
		func() { s.read(readingImages) }), bareH)
	box.add(button("Show the page again", toolkit.ButtonDefault,
		func() { s.read("") }), bareH)
	box.add(toolkit.NewLabel("Neither of these changes the document."), bareH)
	return box
}

// read puts a reading of the page where the picture of it was, or the picture
// back when it is the reading already showing.
func (s *state) read(what string) {
	if s.tools.reading == what {
		what = ""
	}
	s.tools.reading = what
	s.note = ""
	s.refresh()
}

// readingView is the page read rather than drawn.
func (s *state) readingView(src *reader.Document) toolkit.Widget {
	if s.tools.reading == readingText {
		return s.textView(src)
	}
	return s.imagesView(src)
}

// lineH is how tall one line of a reading is.
const lineH = 16

// textView is everything the page says, in reading order.
func (s *state) textView(src *reader.Document) toolkit.Widget {
	text, err := extract.Text(src, s.at)
	if err != nil {
		return toolkit.NewLabel("this page cannot be read: " + err.Error())
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return toolkit.NewLabel("this page says nothing that can be read as text")
	}
	col := newColumn()
	for _, line := range lines {
		col.add(toolkit.NewLabel(line), lineH)
	}
	return col.scrollerOf(s.pageW() - 2*margin)
}

// imagesView is every picture the page places, and a way of taking each one
// away. What is handed over is the picture as the document holds it: a JPEG
// comes out a JPEG, and what the library has unfiltered into plain samples
// comes out as those, because turning samples into pixels means reading a
// colour space, which is the renderer's work rather than this one's.
func (s *state) imagesView(src *reader.Document) toolkit.Widget {
	images, err := extract.Images(src, s.at)
	if err != nil {
		return toolkit.NewLabel("the pictures on this page cannot be read: " + err.Error())
	}
	if len(images) == 0 {
		return toolkit.NewLabel("this page carries no pictures")
	}
	col := newColumn()
	// What the PAGE places, which is not everything a viewer draws: a picture
	// belonging to an annotation's appearance is drawn on top of the page
	// rather than by it, and a mask is part of the picture it belongs to
	// rather than a picture of its own. Measured against poppler's pdfimages
	// over 157 pages of real forms that carry any, those two account for
	// every page the two count differently.
	col.add(toolkit.NewLabel("What the page itself places:"), lineH)
	for i, im := range images {
		name := fmt.Sprintf("page%03d-%02d%s", s.at, i+1, pictureSuffix(im))
		data := im.Data
		row := toolkit.NewSettingRow(name,
			button("Hand it over", toolkit.ButtonDefault, func() { s.handOver(name, data) }))
		row.Subtitle = fmt.Sprintf("%d by %d, %s, %d bytes, drawn %.0f by %.0f points at %.0f, %.0f",
			im.Width, im.Height, holds(im), len(im.Data),
			im.DrawnWidth, im.DrawnHeight, im.X, im.Y)
		col.add(row, 2*lineH+8)
	}
	return col.scrollerOf(s.pageW() - 2*margin)
}

// holds says what the bytes of a picture are.
func holds(im extract.Image) string {
	switch im.Filter {
	case "DCTDecode":
		return "a JPEG"
	case "JPXDecode":
		return "a JPEG 2000"
	case "JBIG2Decode":
		return "JBIG2"
	}
	return "plain samples"
}

// pictureSuffix names a picture by what it holds.
func pictureSuffix(im extract.Image) string {
	switch im.Filter {
	case "DCTDecode":
		return ".jpg"
	case "JPXDecode":
		return ".jp2"
	case "JBIG2Decode":
		return ".jbig2"
	}
	return ".samples"
}

// handOver gives a picture to the person as a download of its own.
func (s *state) handOver(name string, data []byte) {
	s.host.Save(name, data)
	s.note = fmt.Sprintf("handed over %s, %d bytes", name, len(data))
	s.refresh()
}
