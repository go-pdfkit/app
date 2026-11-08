package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gfx/gfx/codec"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/convert"
	"github.com/go-pdfkit/ops"
)

// vectorSVG is a drawing every operator of which becomes a path.
const vectorSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60" viewBox="0 0 120 60">
  <rect x="10" y="10" width="100" height="40" fill="#204080"/>
  <circle cx="60" cy="30" r="12" fill="#ffffff"/>
</svg>`

// gradientSVG is a drawing that cannot be paths: convert names a gradient, a
// clip path and an embedded picture as the three it cannot hand over, and this
// is the first of them.
const gradientSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="40">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="#000000"/>
      <stop offset="1" stop-color="#ffffff"/>
    </linearGradient>
  </defs>
  <rect width="80" height="40" fill="url(#g)"/>
</svg>`

// comicArchive is a zip of pictures named the way a comic archive names them —
// including the 2-before-10 pair that a byte-wise sort gets wrong, so that what
// comes back can be checked against the order a reader expects rather than the
// order a naive implementation produces.
func comicArchive(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for i, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		// A different size per entry, so a page can be told from its neighbour
		// by measuring it.
		img := raster.New(20+i*10, 10)
		if err := codec.Encode(w, img, codec.PNG); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestADrawingOpensAsPathsAndSaysSo(t *testing.T) {
	// ⛔ The POINT of the arm, not a smoke test: the status line has to say
	// which of the two shapes came back, because a page of paths scales and
	// keeps its text while a page that was drawn is pixels.
	h := &fakeHost{name: "logo.svg", file: []byte(vectorSVG)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("a drawing did not open at all")
	}
	if s.doc.PageCount() != 1 {
		t.Fatalf("it became %d pages", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "as paths") {
		t.Errorf("the status line says %q", s.note)
	}
	if strings.Contains(s.note, "⚠") {
		t.Errorf("a drawing that kept its paths was warned about: %q", s.note)
	}
	if s.name != "logo.pdf" {
		t.Errorf("it is called %q", s.name)
	}
	// The page is the drawing's own size in points: convert documents that one
	// SVG user unit is one point on this path and that the DPI has nothing to
	// say about it.
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	page, err := src.Page(1)
	if err != nil {
		t.Fatal(err)
	}
	if w, hgt := pageSize(src, page); w != 120 || hgt != 60 {
		t.Errorf("the page is %g by %g points, not the drawing's 120 by 60", w, hgt)
	}
}

func TestADrawingThatCannotBePathsIsDrawnAndWarnedAbout(t *testing.T) {
	h := &fakeHost{name: "shaded.svg", file: []byte(gradientSVG)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("a gradient drawing did not open at all")
	}
	if !strings.Contains(s.note, "⚠") || !strings.Contains(s.note, "picture of it") {
		t.Errorf("the status line says %q, which does not warn", s.note)
	}
	if strings.Contains(s.note, "stays sharp") {
		t.Errorf("a drawn page was called sharp: %q", s.note)
	}
}

func TestAnArchiveOpensAsItsPagesAndCountsThem(t *testing.T) {
	h := &fakeHost{
		name: "book.cbz",
		file: comicArchive(t, "page1.png", "page2.png", "page10.png"),
	}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("an archive did not open at all")
	}
	if s.doc.PageCount() != 3 {
		t.Fatalf("it became %d pages, not 3", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "3 pictures") {
		t.Errorf("the status line says %q", s.note)
	}
	if s.name != "book.pdf" {
		t.Errorf("it is called %q", s.name)
	}
	// ⛔ AND IN THE ORDER THE NAMES PUT THEM, which is the whole job of the
	// format: page10 follows page2, which a byte-wise sort does not do. Each
	// entry was made a different width, so the pages can be told apart by
	// measuring them rather than by trusting the count.
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	var widths []float64
	for at := 1; at <= 3; at++ {
		page, err := src.Page(at)
		if err != nil {
			t.Fatal(err)
		}
		w, _ := pageSize(src, page)
		widths = append(widths, w)
	}
	want := []float64{20, 30, 40}
	for i := range want {
		if widths[i] != want[i] {
			t.Fatalf("the pages are %v points wide, not %v — the order is wrong",
				widths, want)
		}
	}
}

func TestAnArchiveOfNoPicturesSaysSoRatherThanOpeningEmpty(t *testing.T) {
	// A .docx and an .odt are ZIPs too. The useful thing to say to somebody who
	// opened one is convert's own sentence about what it found, not a guess at
	// what they meant.
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
	h := &fakeHost{name: "report.docx", file: buf.Bytes()}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc != nil {
		t.Fatal("an archive with no pictures opened as a document")
	}
	if !strings.Contains(s.note, "no pictures in the archive") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestAnEmptyArchiveIsNotTakenForAComic(t *testing.T) {
	// An empty ZIP begins PK\x05\x06, which looksZipped deliberately does not
	// match: there is nothing in it to make pages from, so the error worth
	// showing is the PDF reader's.
	var buf bytes.Buffer
	if err := zip.NewWriter(&buf).Close(); err != nil {
		t.Fatal(err)
	}
	if looksZipped(buf.Bytes()) {
		t.Fatal("an empty archive was taken for one with pages in it")
	}
	h := &fakeHost{name: "nothing.zip", file: buf.Bytes()}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc != nil {
		t.Fatal("an empty archive opened as a document")
	}
	if !strings.Contains(s.note, "cannot open nothing.zip") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestWhenAConversionFailsNothingIsOpenedAndItSaysWhy(t *testing.T) {
	// The three arms that go through convert each have a failure the file
	// itself cannot reach, because convert is what fails. Every one of them
	// must leave the workbench as it was and say what went wrong.
	for _, c := range []struct {
		name string
		host *fakeHost
		fail func(t *testing.T)
		says string
	}{
		{
			name: "a drawing that will not become paths or pixels",
			host: &fakeHost{name: "logo.svg", file: []byte(vectorSVG)},
			fail: func(t *testing.T) {
				wasPaths, wasDrawn := svgAsPaths, svgAsDrawn
				t.Cleanup(func() { svgAsPaths, svgAsDrawn = wasPaths, wasDrawn })
				svgAsPaths = func([]byte, convert.Options) ([]byte, error) {
					return nil, errors.New("no paths here")
				}
				svgAsDrawn = func([]byte, convert.Options) ([]byte, error) {
					return nil, errors.New("the renderer gave up")
				}
			},
			says: "this drawing cannot be put on a page",
		},
		{
			name: "a drawing that becomes a PDF nothing can read",
			host: &fakeHost{name: "logo.svg", file: []byte(vectorSVG)},
			fail: func(t *testing.T) {
				was := svgAsPaths
				t.Cleanup(func() { svgAsPaths = was })
				svgAsPaths = func([]byte, convert.Options) ([]byte, error) {
					return []byte("not a PDF at all"), nil
				}
			},
			says: "what was drawn cannot be read back",
		},
		{
			name: "an archive convert refuses",
			host: &fakeHost{name: "book.cbz", file: comicArchive(t, "page1.png")},
			fail: func(t *testing.T) {
				was := archiveAsPages
				t.Cleanup(func() { archiveAsPages = was })
				archiveAsPages = func([]byte, convert.Options) ([]byte, error) {
					return nil, errors.New("the archive holds 9000 entries, past the 4096 allowed")
				}
			},
			says: "this archive cannot be made into a document",
		},
		{
			name: "an archive that becomes a PDF nothing can read",
			host: &fakeHost{name: "book.cbz", file: comicArchive(t, "page1.png")},
			fail: func(t *testing.T) {
				was := archiveAsPages
				t.Cleanup(func() { archiveAsPages = was })
				archiveAsPages = func([]byte, convert.Options) ([]byte, error) {
					return []byte("not a PDF at all"), nil
				}
			},
			says: "what came out of the archive cannot be read back",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newState(surfaceW, surfaceH, c.host)
			c.fail(t)
			s.open()
			if s.doc != nil {
				t.Error("something opened anyway")
			}
			if !strings.Contains(s.note, c.says) {
				t.Errorf("the status line says %q, not %q", s.note, c.says)
			}
		})
	}
}

func TestAnArchiveThatPassesABoundIsRefusedAndTheReasonReachesTheStatusLine(t *testing.T) {
	// ⛔ A REAL REFUSAL, from a real archive, through the real code — not a seam
	// standing in for one. convert makes its ceilings VARIABLES on purpose: "a
	// ceiling of half a gigabyte cannot be reached by a test that has to run in
	// a second, so a constant here would be a guard nobody could exercise". So
	// the ceiling comes down to where a three-entry archive is over it, and what
	// is checked is that the refusal arrives at the workbench naming the bound
	// rather than being swallowed into an empty document.
	for _, c := range []struct {
		name  string
		lower func(t *testing.T)
		says  string
		after bool // the refused entry must not be the first one
	}{
		{
			name: "too many entries",
			lower: func(t *testing.T) {
				was := convert.MaxArchiveEntries
				t.Cleanup(func() { convert.MaxArchiveEntries = was })
				convert.MaxArchiveEntries = 2
			},
			says: "past the 2 allowed",
		},
		{
			name: "one entry too large",
			lower: func(t *testing.T) {
				was := convert.MaxArchiveEntryBytes
				t.Cleanup(func() { convert.MaxArchiveEntryBytes = was })
				convert.MaxArchiveEntryBytes = 64
			},
			// Not `after`: this bound refuses page1.png, and a control that
			// marked it `after` fails with exactly that message — which is how
			// the cumulative case below is known to be measuring its own bound.
			says: "past the 64 allowed",
		},
		{
			// ⛔ This case has to be told apart from the one above it, and the
			// message cannot do it: convert computes the room left as the
			// SMALLER of the per-entry ceiling and what remains of the whole,
			// and prints that one number under the same sentence. So what
			// distinguishes the cumulative bound is WHICH entry it refuses —
			// never the first, because the first is what filled the budget.
			// Asserting only "past the N allowed" here would have passed on the
			// per-entry ceiling and measured nothing.
			name: "the whole archive too large",
			lower: func(t *testing.T) {
				was := convert.MaxArchiveBytes
				t.Cleanup(func() { convert.MaxArchiveBytes = was })
				convert.MaxArchiveBytes = 200
			},
			says:  "allowed",
			after: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := &fakeHost{
				name: "bomb.cbz",
				file: comicArchive(t, "page1.png", "page2.png", "page3.png"),
			}
			s := newState(surfaceW, surfaceH, h)
			c.lower(t)
			s.open()
			if s.doc != nil {
				t.Fatalf("an archive over the bound opened anyway, as %d pages",
					s.doc.PageCount())
			}
			if !strings.Contains(s.note, "this archive cannot be made into a document") {
				t.Errorf("the status line says %q", s.note)
			}
			if !strings.Contains(s.note, c.says) {
				t.Errorf("the status line does not name the bound: %q", s.note)
			}
			if c.after && strings.Contains(s.note, "page1.png") {
				t.Errorf("the cumulative bound refused the FIRST entry, so this "+
					"is the per-entry ceiling under another name: %q", s.note)
			}
		})
	}
}

func TestTheFileDialogOffersEveryShapeOpenAcceptsAndNothingElse(t *testing.T) {
	// ⛔⛔ THE GATE FOR THE OTHER HALF OF EVERY ARM. The accept list said
	// `application/pdf,.pdf` while the picture arm had been there from the
	// start: the code was right, every test passed because tests hand bytes
	// straight to `open`, and the file dialog would not show a PNG. No test
	// here could see it, because the browser is where the filter lives.
	//
	// Compared IN BOTH DIRECTIONS. A suffix in the Go list that the dialog does
	// not offer is unreachable; one the dialog offers that the Go list does not
	// hold is a file the workbench refuses after somebody chose it.
	page, err := os.ReadFile(filepath.Join("web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	accept, ok := acceptList(string(page))
	if !ok {
		t.Fatal("web/index.html has no file input with an accept list")
	}
	offered := map[string]bool{}
	for _, item := range strings.Split(accept, ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
			t.Error("the accept list has an empty entry")
		case strings.HasPrefix(item, "."):
			offered[item] = true
		case strings.Contains(item, "/"):
			// A media type, which mobile pickers honour where they ignore a
			// suffix. Checked for shape only: the suffixes are the contract.
		default:
			t.Errorf("the accept list holds %q, which is neither a suffix nor a media type", item)
		}
	}
	for _, want := range openSuffixes {
		if !offered[want] {
			t.Errorf("Open accepts %s and the file dialog does not offer it", want)
		}
		delete(offered, want)
	}
	for left := range offered {
		t.Errorf("the file dialog offers %s and Open does not accept it", left)
	}
}

// acceptList is the value of the accept attribute on the page's file input.
func acceptList(page string) (string, bool) {
	const key = `accept="`
	at := strings.Index(page, key)
	if at < 0 {
		return "", false
	}
	rest := page[at+len(key):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func TestHowAConvertedFileIsNamed(t *testing.T) {
	// ⛔ The suffix is REPLACED. "book.cbz" used to become "book.cbz.pdf", which
	// saves as "book.cbz-edited.pdf" — a name claiming to be a comic archive, a
	// PDF and an edit of something, filed next to the archive it came from.
	for _, c := range []struct{ was, now string }{
		{"book.cbz", "book.pdf"},
		{"logo.svg", "logo.pdf"},
		{"receipt.png", "receipt.pdf"},
		{"scan", "scan.pdf"},
		{"two.dots.here.tiff", "two.dots.here.pdf"},
		{"", "document.pdf"},
	} {
		if got := pdfNamed(c.was); got != c.now {
			t.Errorf("%q became %q, not %q", c.was, got, c.now)
		}
	}
}

func TestOneOrManyPicturesAreCountedInWords(t *testing.T) {
	// A count printed with the wrong plural reads as a program that does not
	// know what it just did.
	h := &fakeHost{name: "one.cbz", file: comicArchive(t, "only.png")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if want := "1 picture from the archive"; !strings.Contains(s.note, want) {
		t.Errorf("the status line says %q, not %q", s.note, want)
	}
	if strings.Contains(s.note, "1 pictures") {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestAPictureStillOpensExactlyAsItDid(t *testing.T) {
	// The control for the whole chain: the arm that was there before this file
	// existed is tried FIRST and is unchanged, so nothing that opened yesterday
	// takes a different path today.
	var buf bytes.Buffer
	if err := codec.Encode(&buf, raster.New(24, 12), codec.PNG); err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{name: "receipt.png", file: buf.Bytes()}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("a picture did not open")
	}
	if s.note != "this was a picture, and is now a page" {
		t.Errorf("the status line says %q", s.note)
	}
	if s.raw == nil {
		t.Error("a document begun from a picture was left with no file behind it")
	}
}

func TestADrawingCarriesTheBytesItWasMadeFromRatherThanBeingWrittenTwice(t *testing.T) {
	// ⛔ An arm that went through convert already holds a written PDF. If `open`
	// wrote the document again to have a file to read back, a vector page would
	// pay a second write — and for the SVG arm that is a second rasterisation
	// decision nobody asked for. The seam that would be used for that write is
	// made to fail: the drawing must still open.
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) {
		return nil, errors.New("this write was not supposed to happen")
	}
	h := &fakeHost{name: "logo.svg", file: []byte(vectorSVG)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the drawing was written a second time and failed: %q", s.note)
	}
	if !bytes.HasPrefix(s.raw, []byte("%PDF")) {
		t.Errorf("the bytes behind it begin %q",
			s.raw[:min(8, len(s.raw))])
	}
}

func TestWhatLooksZippedAndWhatDoesNot(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		want bool
	}{
		{"a local file header", []byte("PK\x03\x04rest"), true},
		{"an empty archive", []byte("PK\x05\x06rest"), false},
		{"a spanned archive", []byte("PK\x07\x08rest"), false},
		{"plain words", []byte("just some words"), false},
		{"nothing at all", nil, false},
		{"two bytes", []byte("PK"), false},
	} {
		if got := looksZipped(c.in); got != c.want {
			t.Errorf("%s: looksZipped = %v", c.name, got)
		}
	}
}
