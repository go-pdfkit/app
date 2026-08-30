// The Read group: what the page says and what it carries, as against what it
// looks like.
//
// Nothing here changes the document, so nothing here can break the promise the
// rest of the workbench makes. It keeps it all the same: the text and the
// pictures are read out of the document written and read back, not out of the
// one being assembled — so what is listed is what would come out of Save.

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-gfx/gfx/codec"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
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
	// The chooser sits on the same row as the verb it governs, so that what
	// the file will be is beside the press that makes it rather than three
	// rows above. It governs the zip below it as well: both of them draw the
	// page and write it, and two choosers saying different things about the
	// same picture would be a question nobody asked.
	formats := toolkit.NewCycleButton(formatNames()...)
	formats.Index().Subscribe(func(i int) { s.tools.picture = i })
	box.add(buttons(formats, button("Hand over this page", toolkit.ButtonDefault,
		s.pageAsPicture)), bareH)
	box.add(button("Hand over every page, zipped", toolkit.ButtonDefault,
		s.everyPageZipped), bareH)
	box.add(button("Hand over what this page says", toolkit.ButtonDefault,
		s.textAsFile), bareH)
	box.add(toolkit.NewLabel("None of these changes the document."), bareH)
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

// pageAsPicture draws the page and hands it over in a format anything opens.
//
// A picture already in the document comes out of "what this page carries" as
// the bytes it is stored in, which is right: a JPEG handed over as a JPEG is
// the file itself, losing nothing. But a page is not a picture in the document
// — it is a drawing of everything on it — and a scanned page's own pictures
// are stored as JPEG 2000 and JBIG2, which almost nothing opens. This draws
// the page and writes it out in whichever of the formats the chooser beside it
// names.
//
// It is drawn at twice the size it is shown at, because what this is for is
// taking away rather than looking at, and a page fitted to a window is smaller
// than the page.
func (s *state) pageAsPicture() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	pic := s.chosenFormat()
	data, partial, err := s.drawnPicture(src, s.at, pic)
	if err != nil {
		s.fail(err.Error())
		return
	}
	s.handOver(fmt.Sprintf("page%03d%s", s.at, pic.suffix), data)
	if partial {
		s.note += fmt.Sprintf("; this page was still being drawn after %s, so that is as far as it got", pageBudget)
		s.refresh()
	}
}

// drawnPicture draws one page and writes it in one format, saying whether what
// came back is all of it.
//
// Alpha is the library's business rather than this one's: PNG and TIFF carry
// it, and codec.Encode composites onto white for JPEG, GIF and BMP, which do
// not. Doing it again here would be doing it twice. What is this one's
// business is that the name matches — a page written as a BMP and handed over
// as a .png is a file nothing can open.
func (s *state) drawnPicture(src *reader.Document, at int, pic pictureFormat) (data []byte, partial bool, err error) {
	img, err := drawPage(src, at, render.Options{
		Scale:       2 * s.fitScale(src),
		MaxDuration: pageBudget,
	})
	// A page that ran out of time comes back as far as it got. Handing that
	// over silently would be handing over half a page as though it were the
	// page, so the caller is told which it is.
	partial = errors.Is(err, render.ErrTimedOut) && img != nil
	if err != nil && !partial {
		return nil, false, fmt.Errorf("this page cannot be drawn: %w", err)
	}
	var buf bytes.Buffer
	if err := encodeImage(&buf, img, pic.format); err != nil {
		return nil, false, fmt.Errorf("this page cannot be written as a %s: %w", pic.format, err)
	}
	return buf.Bytes(), partial, nil
}

// everyPageZipped hands over the whole document as pictures, in one file.
//
// A page at a time is no use for a document of two hundred, and a browser that
// is handed two hundred downloads at once asks about each of them. A zip of
// PNGs or JPEGs is also what a comic reader opens under the name CBZ, which is
// the same file with another suffix.
//
// The pages go in in whichever format the chooser on the row above names, and
// the entries carry that format's suffix.
func (s *state) everyPageZipped() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	pic := s.chosenFormat()
	short := 0
	for at := 1; at <= src.PageCount(); at++ {
		data, partial, err := s.drawnPicture(src, at, pic)
		if err != nil {
			s.fail(err.Error())
			return
		}
		if partial {
			short++
		}
		// The zip is built in memory: Create only refuses a name already
		// used or a writer already closed, neither of which can happen with
		// one entry per page number, and a bytes.Buffer never fails to take
		// bytes. Close only flushes.
		w, _ := zw.Create(fmt.Sprintf("page%03d%s", at, pic.suffix))
		w.Write(data)
	}
	zw.Close()
	s.handOver(strings.TrimSuffix(s.name, ".pdf")+"-pages.zip", buf.Bytes())
	if short > 0 {
		s.note += fmt.Sprintf("; %d of them were still being drawn after %s", short, pageBudget)
		s.refresh()
	}
}

// textAsFile hands over what the page says, which the reading beside it shows
// on the screen and had no way of taking away.
func (s *state) textAsFile() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	text, err := extract.Text(src, s.at)
	if err != nil {
		s.fail("this page cannot be read: " + err.Error())
		return
	}
	s.handOver(fmt.Sprintf("page%03d.txt", s.at), []byte(text))
}

// encodeImage is a variable so a test can watch what happens when writing the
// picture fails, which is the branch that decides whether a person is handed a
// truncated file or told.
var encodeImage = func(w io.Writer, img *raster.Image, f codec.Format) error {
	return codec.Encode(w, img, f)
}

// pictureFormat is one way of handing a drawn page over: the format itself,
// and the suffix a person expects the file to carry. The two are kept together
// because the one mistake worth designing out is a file whose name says one
// thing and whose bytes say another.
type pictureFormat struct {
	format codec.Format
	suffix string
}

// everyKnownFormat is every container gfx can read, with its usual suffix.
// Only some of them can be written, and which is the library's contract rather
// than a guess made here: codec.CanEncode below decides what reaches the
// chooser. Written this way round, a reference encoder arriving in gfx — WEBP
// is the obvious one — puts that format in front of a person without a line
// changing here.
var everyKnownFormat = []pictureFormat{
	{codec.PNG, ".png"},
	{codec.JPEG, ".jpg"},
	{codec.GIF, ".gif"},
	{codec.WEBP, ".webp"},
	{codec.TIFF, ".tif"},
	{codec.BMP, ".bmp"},
	{codec.ICO, ".ico"},
	{codec.ICNS, ".icns"},
	{codec.PNM, ".pnm"},
	{codec.QOI, ".qoi"},
	{codec.JP2, ".jp2"},
	{codec.JBIG2, ".jbig2"},
}

// pictureFormats is what the chooser offers, in that order — PNG first,
// because it is the one anything opens and the one this handed over when there
// was no choice to make.
//
// The order is also worth reading as a warning about size. Over forty real
// documents, the first page of each drawn at twice the size it is shown at and
// written five ways, the mean file came to 209 kB as a PNG, 129 kB as a JPEG
// and 105 kB as a GIF — and 4.9 MB as a BMP and 6.6 MB as a TIFF, which are
// uncompressed. That is not this program's to fix (the encoders are gfx's, and
// the ratio between the two large ones is exactly four bytes a pixel against
// three, which is the alpha channel TIFF carries and BMP does not), but it is
// worth knowing before choosing one for a document of two hundred pages.
var pictureFormats = writableFormats()

// writableFormats keeps the formats that can actually be written.
func writableFormats() []pictureFormat {
	out := make([]pictureFormat, 0, len(everyKnownFormat))
	for _, p := range everyKnownFormat {
		if codec.CanEncode(p.format) {
			out = append(out, p)
		}
	}
	return out
}

// formatNames is what the chooser says, which is the format's own name: a
// person looking for a JPEG is looking for the word, not for ".jpg".
func formatNames() []string {
	names := make([]string, len(pictureFormats))
	for i, p := range pictureFormats {
		names[i] = p.format.String()
	}
	return names
}

// chosenFormat is the format the two hand-over verbs write. A choice that is
// not one of them falls back on the first rather than reaching past the end of
// the list: the chooser cannot make that happen, and a panic if something else
// ever did would be a poor way of finding out.
func (s *state) chosenFormat() pictureFormat {
	if s.tools.picture < 0 || s.tools.picture >= len(pictureFormats) {
		return pictureFormats[0]
	}
	return pictureFormats[s.tools.picture]
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
