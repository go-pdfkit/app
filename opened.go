// What Open accepts that is not a PDF.
//
// Somebody who hands a file to a PDF workbench wants a PDF of it. Telling them
// it is not a PDF is true and useless: they knew. A picture has opened as a
// page since the workbench could open anything; this adds the two other shapes
// the fleet can already read — an SVG drawing and a comic archive — and makes
// each of them say WHICH of those it was, because the three keep different
// amounts of the original.
//
// ⛔ THE ORDER OF THE ARMS IS THE ORDER THAT KEEPS THE MOST. A picture is tried
// first, so nothing that opens today changes path. The SVG arm asks for paths
// before it accepts pixels. The archive arm is last, because a ZIP is also what
// a .docx and an .odt are, and the useful thing to tell somebody who opened one
// is convert's own sentence about finding no pictures in it rather than a guess
// about what they meant.

package main

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/go-pdfkit/convert"
	"github.com/go-pdfkit/ops"
)

// made is what a file that was not a PDF became: the document, the bytes it
// was made from, and the sentence that says what happened to it.
//
// ⛔ The BYTES are carried, not re-derived. An arm that goes through convert
// already holds a written PDF, and `open` would otherwise write the document a
// second time to have a file to read back — a second write of a document that
// came from a picture is harmless, but of one that came from a vector drawing
// it is a second rasterisation decision nobody asked for.
type made struct {
	doc  *ops.Doc
	pdf  []byte
	said string
}

// documentFrom turns bytes that are not a PDF into a document, and says what
// they were. The second result is false when nothing here could read them, and
// then the caller says what the PDF reader said — which is the useful error for
// a file that was meant to be a PDF and is damaged.
func (s *state) documentFrom(data []byte) (made, bool) {
	if d, ok := s.pictureDocument(data); ok {
		return made{doc: d, said: "this was a picture, and is now a page"}, true
	}
	if convert.LooksLikeSVG(data) {
		return s.svgDocument(data)
	}
	if looksZipped(data) {
		return s.archiveDocument(data)
	}
	return made{}, false
}

// svgDocument lays an SVG on a page, as paths where it can and as a drawing
// where it cannot — and says which, because the difference is everything a
// reader of the result can do with it.
//
// ⛔ IT ASKS FOR THE VECTOR FORM FIRST, although SVGToPDF would have fallen
// back by itself. The fallback is in convert and is the right behaviour; what
// convert cannot do is tell the person at the workbench which of the two
// happened. A page of paths scales, prints at any size and keeps its text; a
// page that was drawn is pixels, with exactly the loss the Redraw group warns
// about. Calling SVGToPDF alone would have been one line shorter and would have
// left the workbench unable to say which file it just handed back.
func (s *state) svgDocument(doc []byte) (made, bool) {
	out, err := svgAsPaths(doc, convert.Options{})
	said := "the drawing is on the page as paths, so it stays sharp at any size"
	if err != nil {
		// Not a failure: convert reports the semantics it could not hand over —
		// a gradient, a clip path, an embedded picture — and then this asks for
		// the drawn form on purpose.
		out, err = svgAsDrawn(doc, convert.Options{})
		if err != nil {
			s.fail("this drawing cannot be put on a page: " + err.Error())
			return made{}, true
		}
		said = "⚠ this drawing uses something that cannot be turned into paths, " +
			"so the page is a picture of it rather than the drawing itself"
	}
	d, err := ops.Open(out)
	if err != nil {
		s.fail("what was drawn cannot be read back: " + err.Error())
		return made{}, true
	}
	return made{doc: d, pdf: out, said: said}, true
}

// archiveDocument lays the pictures in an archive onto pages.
//
// ⛔ The bounds are convert's and are not restated here: MaxArchiveEntries
// bounds an allocation made before anything is read, MaxArchiveEntryBytes and
// MaxArchiveBytes bound one entry and the whole, "because one entry under the
// ceiling says nothing about a thousand of them". A refusal from any of them
// reaches the status line as it is written, which is what a reader needs: the
// name of the entry and the ceiling it passed.
func (s *state) archiveDocument(src []byte) (made, bool) {
	out, err := archiveAsPages(src, convert.Options{})
	if err != nil {
		s.fail("this archive cannot be made into a document: " + err.Error())
		return made{}, true
	}
	d, err := ops.Open(out)
	if err != nil {
		s.fail("what came out of the archive cannot be read back: " + err.Error())
		return made{}, true
	}
	return made{
		doc: d,
		pdf: out,
		said: fmt.Sprintf("%d picture%s from the archive, in the order their names put them",
			d.PageCount(), plural(d.PageCount())),
	}, true
}

// looksZipped says whether these bytes begin a ZIP's first local file header.
//
// ⛔ By CONTENT and not by the name the host gave, for the reason convert gives
// for LooksLikeSVG: a comic archive arrives as .cbz, .zip, .cbr-that-is-really-
// a-zip, or with no suffix at all. An EMPTY archive begins PK\x05\x06 instead
// and is deliberately not matched — there is nothing in it to make pages from,
// so the PDF reader's own error is the better thing to say.
func looksZipped(b []byte) bool {
	return bytes.HasPrefix(b, []byte("PK\x03\x04"))
}

// plural is the s on a count.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// pdfNamed is what the converted file is called: the same name, carrying .pdf
// instead of whatever it carried.
//
// ⛔ REPLACING the suffix rather than appending one. "book.cbz" became
// "book.cbz.pdf", which saves as "book.cbz-edited.pdf" — a name that says the
// file is a comic archive and a PDF and an edit of something, and which a
// desktop sorts next to the archive it came from rather than next to the other
// documents.
func pdfNamed(name string) string {
	if name == "" {
		return "document.pdf"
	}
	return strings.TrimSuffix(name, path.Ext(name)) + ".pdf"
}

// openSuffixes is every file `Open` accepts, and it is the ONE list.
//
// ⛔⛔ THE FILE DIALOG'S FILTER IS HALF OF EVERY ARM ABOVE. web/index.html said
// `accept="application/pdf,.pdf"` while the workbench had read a picture as a
// page since it could open anything: the code was right, the tests passed
// because they hand bytes straight to `open`, and a person standing at the
// dialog could not choose a PNG. Six rows counted as delivered were behind a
// picker that would not show their files.
//
// So the list lives here, in Go, and a test reads the HTML and compares the two
// IN BOTH DIRECTIONS — a suffix here that the dialog does not offer is a verb
// nobody can reach; a suffix the dialog offers that is not here is a file the
// workbench will refuse after somebody chose it.
//
// ⛔ It is wider than the rows that asked for it, on purpose. codec reads ICO,
// ICNS, PNM, QOI, JP2 and JBIG2 as well, `ops.Picture` makes a page of any of
// them, and a dialog that hides a file the program would have opened is the
// same defect at a smaller size.
var openSuffixes = []string{
	".pdf",
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".tif", ".tiff", ".bmp",
	".ico", ".icns", ".pnm", ".pbm", ".pgm", ".ppm", ".qoi", ".jp2",
	".jbig2", ".jb2",
	".svg",
	".cbz", ".zip",
}

// svgAsPaths, svgAsDrawn and archiveAsPages are variables for the reason
// redrawPDF is one: the branch where a conversion fails decides whether
// somebody is handed an empty document or told what went wrong, and a file that
// fails inside convert rather than at its door is not one a test can keep.
var (
	svgAsPaths     = convert.SVGToPDFVector
	svgAsDrawn     = convert.SVGToPDF
	archiveAsPages = convert.ArchiveToPDF
)
