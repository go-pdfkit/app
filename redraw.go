// Drawing the pages again with the pixels changed: inverted, grey, on a
// colour, brighter, or scuffed the way a flatbed scuffs them.
//
// ⛔⛔ EVERY ONE OF THESE REPLACES THE PAGE WITH A PICTURE OF IT, and that has
// to be said out loud every time. `convert.Redraw`'s own documentation says the
// zero value "redraws the pages unchanged, which is rasterize-pdf": the text
// stops being text. After it, nothing can select a word, search the document or
// read it out — and the file gets bigger, not smaller.
//
// That is not a defect of the verbs; turning light into dark is a pixel
// operation and there is no other way to do it. It is a defect of a workbench
// that offers them without saying so, because the reader who inverts a contract
// to read it at night and then saves has quietly thrown away everything that
// made it a document.

package main

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"github.com/go-pdfkit/convert"
	"github.com/go-pdfkit/ops"
)

// grounds are the colours a page can be drawn on, by name. nil is the
// renderer's own white.
//
// ⛔ The colour is handed to the RENDERER rather than painted under the
// finished page, and convert says why: "A first version painted the colour
// under the finished raster — which does nothing at all, because the renderer
// has already filled the page with opaque white". The option was documented,
// shipped, and had no effect. Nothing here needs to repeat that mistake, but it
// is why this passes Background rather than compositing.
var grounds = []struct {
	name string
	rgba *color.RGBA
}{
	{"leave it white", nil},
	{"cream, for reading", &color.RGBA{R: 253, G: 246, B: 227, A: 255}},
	{"grey, for proofing", &color.RGBA{R: 222, G: 222, B: 222, A: 255}},
	{"black, for the night", &color.RGBA{R: 16, G: 16, B: 16, A: 255}},
}

// groundNames is the list a person chooses from.
func groundNames() []string {
	out := make([]string, 0, len(grounds))
	for _, g := range grounds {
		out = append(out, g.name)
	}
	return out
}

// redrawn applies one redrawing to the document and says what it did, with the
// warning that the pages are pictures now.
//
// ⛔ IT GOES THROUGH ops.Doc LIKE EVERY OTHER CHANGE, by writing the document
// out, redrawing it and reading the result back in. convert.Redraw takes and
// returns a whole PDF, so there is no way to hand it a Doc — and doing it any
// other way would put the redrawn pages beside the document rather than in it,
// so Save would hand over the one that was not redrawn.
func (s *state) redrawn(said string, opt convert.RedrawOptions) {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	in, msg := s.reopenBytes()
	if msg != "" {
		s.fail(msg)
		return
	}
	out, err := redrawPDF(in, opt)
	if err != nil {
		s.fail("the pages cannot be redrawn: " + err.Error())
		return
	}
	next, err := ops.Open(out)
	if err != nil {
		s.fail("what was redrawn cannot be read back: " + err.Error())
		return
	}
	s.doc = next
	s.note = said + " — ⚠ the pages are pictures now: their text cannot be " +
		"selected, searched or read out any more"
	s.at = 1
	s.refresh()
}

// redrawPDF is a variable so a test can see what the workbench does when a
// page refuses to be drawn, which is the branch that decides whether somebody
// is handed a document of blank pages or told.
var redrawPDF = convert.Redraw

// invert turns light into dark, which is what a reader wants at night.
func (s *state) invert() {
	s.redrawn("turned the pages inside out", convert.RedrawOptions{Invert: true})
}

// greyscale drops the colour.
func (s *state) greyscale() {
	s.redrawn("dropped the colour", convert.RedrawOptions{Greyscale: true})
}

// onGround draws the pages on the colour chosen beside it.
func (s *state) onGround() {
	at := s.tools.ground.Get()
	if at < 0 || at >= len(grounds) {
		at = 0
	}
	g := grounds[at]
	if g.rgba == nil {
		s.fail("choose a colour to draw the pages on first")
		return
	}
	s.redrawn("drew the pages on "+g.name,
		convert.RedrawOptions{Background: g.rgba})
}

// scanned makes the pages look as though they went through a flatbed.
func (s *state) scanned() {
	s.redrawn("put the pages through a scanner", convert.RedrawOptions{
		// ⛔ A fixed seed, which is convert's own default and its own reason:
		// "two runs over the same file must produce the same bytes, or nothing
		// downstream can be compared, cached or checksummed".
		Scanner: &convert.ScannerEffect{Skew: 0.4, Noise: 0.08, Fade: 0.1},
	})
}

// adjust shifts the brightness and the contrast.
//
// ⛔ The panel holds whole numbers because a spin button does, and they are
// read as PERCENTAGES: brightness −100 to +100 maps to convert's −1 to +1, and
// contrast 100 means "leave it alone" because convert's 1 does. A contrast of
// zero there "would turn every page into a flat grey rectangle", which is why
// convert treats 0 as 1 — and why this never sends it one.
func (s *state) adjust() {
	bright := float64(s.tools.bright.Get()) / 100
	contrast := float64(s.tools.contrast.Get()) / 100
	if contrast <= 0 {
		contrast = 1
	}
	if bright == 0 && contrast == 1 {
		s.fail("set a brightness or a contrast beside it first: both are at none")
		return
	}
	s.redrawn(fmt.Sprintf("redrew the pages at %d%% brightness and %d%% contrast",
		s.tools.bright.Get(), s.tools.contrast.Get()),
		convert.RedrawOptions{Brightness: bright, Contrast: contrast})
}

// asArchive hands the document over as a comic archive: one picture per page
// in a zip a reader opens as a book.
//
// ⛔ A REAL .cbz, which the zip beside it is not. `Hand over every page,
// zipped` writes the pictures under .zip, and a comic reader opens a .cbz —
// the same bytes under the name the format is known by, which is the whole
// difference between a file somebody can use and one they have to rename.
func (s *state) asArchive() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	in, msg := s.reopenBytes()
	if msg != "" {
		s.fail(msg)
		return
	}
	title := saveName(s.name)
	var out bytes.Buffer
	if err := archivePDF(&out, in, convert.RasterOptions{}, title); err != nil {
		s.fail("the pages cannot be written as an archive: " + err.Error())
		return
	}
	s.handOver(strings.TrimSuffix(title, ".pdf")+".cbz", out.Bytes())
}

// archivePDF is a variable for the reason redrawPDF is one.
var archivePDF = convert.ToArchive
