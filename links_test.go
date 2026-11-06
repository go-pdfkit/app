package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// linkedPDF is a three-page document whose first page carries three links over
// real words: one to page 3, one to an address, and one over no text at all.
//
// The text is set in a font of even width so a test can say where every letter
// lands, which is what makes "the words under this rectangle" a thing a test
// can assert rather than hope for.
func linkedPDF(t *testing.T) []byte {
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

	refs := make([]reader.Ref, 3)
	for i := range refs {
		refs[i] = w.Reserve()
	}
	for i, ref := range refs {
		// Two lines of text, at 10 points: "the first chapter" on the upper one
		// and "elsewhere entirely" below it. At 500/1000 em each character is
		// 5 points wide.
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
			"BT /F1 10 Tf 20 300 Td (the first chapter) Tj ET\n" +
				"BT /F1 10 Tf 20 280 Td (elsewhere entirely) Tj ET\n" +
				fmt.Sprintf("BT /F1 10 Tf 20 260 Td (page %d) Tj ET", i+1))})
		page := reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}
		if i == 0 {
			page["Annots"] = reader.Array{
				// Over "the first chapter": 17 characters from x=20, so
				// 20..105, with the line at y=300 and 10 points tall.
				w.Add(reader.Dict{"Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(20), reader.Integer(298),
						reader.Integer(105), reader.Integer(312)},
					"Dest": reader.Array{refs[2], reader.Name("Fit")}}),
				// Over "elsewhere entirely", on the line below.
				w.Add(reader.Dict{"Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(20), reader.Integer(278),
						reader.Integer(110), reader.Integer(292)},
					"A": reader.Dict{"S": reader.Name("URI"),
						"URI": reader.String("https://example.org/elsewhere")}}),
				// Over nothing at all, at the foot of the page.
				w.Add(reader.Dict{"Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(20), reader.Integer(40),
						reader.Integer(120), reader.Integer(60)},
					"Dest": reader.Array{refs[1], reader.Name("Fit")}}),
			}
		}
		w.Put(ref, page)
	}
	kids := make(reader.Array, len(refs))
	for i, r := range refs {
		kids[i] = r
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count":    reader.Integer(len(refs)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// openedWith builds a workbench holding the bytes given.
func openedWith(t *testing.T, b []byte) *state {
	t.Helper()
	s := newState(surfaceW, surfaceH, &fakeHost{name: "linked.pdf", file: b})
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	return s
}

// ⛔⛔ THE CORRECTION THIS TEST CARRIES. The contents view shipped falling back
// to a LIST OF PAGES, and the person who asked for it said, on seeing it, that
// what they meant by a table of contents was a display of the document's LINKS.
// A link is structure the author wrote — this phrase leads there. A list of
// pages is arithmetic: the same list for every file of the same length.
func TestADocumentWithNoOutlineListsItsLinks(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	if got := s.bookmarks(s.src); len(got) != 0 {
		t.Fatalf("the fixture carries an outline after all: %+v", got)
	}

	rows := s.contentsOf(s.src, s.thumbs.stamp)
	if len(rows) != 3 {
		t.Fatalf("%d rows for a document carrying three links: %+v", len(rows), rows)
	}
	// ⛔ And they are NOT the page list, which is what this used to be. A
	// three-page document would have given exactly three rows either way, so
	// the count alone proves nothing: what the rows SAY is the whole test.
	if rows[0].title == "Page 1" {
		t.Fatal("the fallback is still listing pages")
	}
}

// A row says the words the link sits on, because that is the only name a link
// has: a PDF link is a rectangle and a destination, and nothing in it is a
// title.
func TestALinkRowSaysTheWordsItSitsOn(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	rows := s.linkList(s.src)
	if len(rows) != 3 {
		t.Fatalf("%d rows: %+v", len(rows), rows)
	}
	if rows[0].title != "the first chapter" || rows[0].page != 3 {
		t.Errorf("the first link reads %q and goes to page %d", rows[0].title, rows[0].page)
	}
	if rows[1].title != "elsewhere entirely" {
		t.Errorf("the second link reads %q", rows[1].title)
	}
	// ⛔ The control that the words come from UNDER THE RECTANGLE and not from
	// the page as a whole: the two links sit on two different lines of the same
	// page, and each must find only its own.
	if strings.Contains(rows[0].title, "elsewhere") || strings.Contains(rows[1].title, "chapter") {
		t.Errorf("the rows read %q and %q; each link has taken the other's line", rows[0].title, rows[1].title)
	}
}

// ⛔ A link with no words under it is still a row. An image that is a link, a
// link over a blank area of a scan, a link whose text cannot be read: a row
// naming the destination is more use than no row, because the reader at least
// learns the document points there.
func TestALinkOverNoTextNamesWhereItGoes(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	rows := s.linkList(s.src)
	if len(rows) != 3 {
		t.Fatalf("%d rows", len(rows))
	}
	if rows[2].title != "Page 2" || rows[2].page != 2 {
		t.Errorf("the link over nothing reads %q and goes to page %d", rows[2].title, rows[2].page)
	}
}

// ⛔ A LINK OUT OF THE DOCUMENT IS SHOWN AND NOT FOLLOWED. The promise of this
// workbench is that nothing leaves this tab, and navigating the tab to
// somebody else's URL is the one gesture that would break it — silently, and in
// the one direction a reader cannot undo.
func TestPressingALinkThatLeavesTheDocumentSaysSoAndStays(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	rows := s.linkList(s.src)
	if rows[1].uri != "https://example.org/elsewhere" {
		t.Fatalf("the second row carries the address %q", rows[1].uri)
	}

	was := s.at
	// ⛔ Pressed through the PANE at its row's index, not by building a row on
	// its own: the pane maps an index back to a bookmark, and that mapping is
	// the part a reader's press actually goes through.
	lb := listBoxOf(t, s.contentsPane(rows))
	lb.OnActivate(1)
	if s.at != was {
		t.Errorf("pressing it moved the reader to page %d", s.at)
	}
	if !strings.Contains(s.note, "https://example.org/elsewhere") {
		t.Errorf("pressing it said %q, which does not say where it would go", s.note)
	}
}

// A row that goes to a page goes there when it is pressed, the same as an
// outline row does.
func TestPressingALinkRowGoesToItsPage(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	s.goTo(1)
	lb := listBoxOf(t, s.contentsPane(s.linkList(s.src)))
	lb.OnActivate(0)
	if s.at != 3 {
		t.Errorf("pressing a link to page 3 left the reader on page %d", s.at)
	}
}

// ⛔ A document chooses how many links it carries, and the last row SAYS the
// list was cut. A list that silently stops lies about what the document holds,
// which is the one thing a listing must not do.
func TestMoreLinksThanTheBoundSaysSo(t *testing.T) {
	s := openedWith(t, manyLinksPDF(t, maxLinkRows+25))
	rows := s.linkList(s.src)
	if len(rows) != maxLinkRows+1 {
		t.Fatalf("%d rows for %d links with a bound of %d", len(rows), maxLinkRows+25, maxLinkRows)
	}
	last := rows[len(rows)-1]
	if !strings.Contains(last.title, "25 more") {
		t.Errorf("the last row reads %q, and 25 links were left out", last.title)
	}
	// The row that says the list was cut is not a place to go.
	if last.page != 0 || last.uri != "" {
		t.Errorf("the row saying the list was cut leads somewhere: %+v", last)
	}
}

// manyLinksPDF is a one-page document carrying n links, all to itself.
func manyLinksPDF(t *testing.T, n int) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Reserve()
	annots := make(reader.Array, 0, n)
	for i := 0; i < n; i++ {
		annots = append(annots, w.Add(reader.Dict{"Subtype": reader.Name("Link"),
			"Rect": reader.Array{reader.Integer(0), reader.Integer(0),
				reader.Integer(1), reader.Integer(1)},
			"Dest": reader.Array{pageRef, reader.Name("Fit")}}))
	}
	w.Put(pageRef, reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 20 20 200 200 re f")}),
		"Annots":   annots})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": reader.Array{pageRef},
		"Count":    reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔ The page list is the LAST resort now, not the first. A document with
// neither an outline nor a link still gets a pane rather than an empty one.
func TestADocumentWithNeitherOutlineNorLinksStillListsItsPages(t *testing.T) {
	s, _ := opened(t, 3) // samplePDF carries no outline and no links
	if got := s.doc.Links(); len(got) != 0 {
		t.Fatalf("the sample carries %d links after all", len(got))
	}
	rows := s.contentsOf(s.src, [2]uint64{})
	if len(rows) != 3 || rows[2].title != "Page 3" {
		t.Errorf("the last resort reads %+v", rows)
	}
}

// An outline still wins over the links: it is the author saying which parts
// matter, which is more than a link says.
func TestAnOutlineIsPreferredToTheLinks(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	// The outline is injected through the memo contentsOf reads, which is the
	// only way in now that contentsOf takes the document rather than a list.
	stamp := [2]uint64{7, 7}
	s.marksDoc, s.marks = stamp, []bookmark{{title: "a chapter", page: 2}}
	rows := s.contentsOf(s.src, stamp)
	if len(rows) != 1 || rows[0].title != "a chapter" {
		t.Errorf("with an outline the pane reads %+v", rows)
	}
}

func TestATitleLongerThanALineIsClipped(t *testing.T) {
	long := strings.Repeat("word ", 200)
	got := clipTitle(long)
	if n := len([]rune(got)); n != linkTitleMax {
		t.Errorf("a long title came back %d runes, want %d", n, linkTitleMax)
	}
	if !strings.HasSuffix(got, outlineEllipsis) {
		t.Errorf("a clipped title does not say it was clipped: %q", got)
	}
	// ⛔ Counted in RUNES. Clipping by bytes cuts a character in half and the
	// row comes back with a replacement glyph in it.
	wide := strings.Repeat("é", 400)
	if n := len([]rune(clipTitle(wide))); n != linkTitleMax {
		t.Errorf("a title of two-byte characters came back %d runes", n)
	}
}

func TestWordsUnderJoinsWhatTheGapSays(t *testing.T) {
	// Two runs on one line, far enough apart to be two words; and a third on
	// another line, which is never joined to them.
	runs := []extract.Run{
		{Text: "alpha", X: 10, Y: 100, Width: 25, Size: 10, Space: 5},
		// A run that says nothing, which a page can draw and which must not
		// become a stray space between two words.
		{Text: "", X: 36, Y: 100, Width: 3, Size: 10, Space: 5},
		{Text: "beta", X: 40, Y: 100, Width: 20, Size: 10, Space: 5},
		{Text: "gamma", X: 10, Y: 80, Width: 25, Size: 10, Space: 5},
	}
	if got := wordsUnder(runs, [4]float64{0, 95, 200, 115}); got != "alpha beta" {
		t.Errorf("one line read %q", got)
	}
	// Kerning, not a word break: the gap is under half a space.
	tight := []extract.Run{
		{Text: "al", X: 10, Y: 100, Width: 10, Size: 10, Space: 5},
		{Text: "pha", X: 21, Y: 100, Width: 15, Size: 10, Space: 5},
	}
	if got := wordsUnder(tight, [4]float64{0, 95, 200, 115}); got != "alpha" {
		t.Errorf("a kerned pair read %q", got)
	}
	// ⛔ And a rectangle over nothing finds nothing, rather than the whole page.
	if got := wordsUnder(runs, [4]float64{0, 0, 5, 5}); got != "" {
		t.Errorf("a rectangle over an empty corner read %q", got)
	}
}

// ⛔ Runs arrive in the order the page DRAWS them, and a page is free to draw
// its last word first. A row that read them in that order would say "chapter
// first the".
func TestWordsUnderReadsDownAndAcrossNotInDrawingOrder(t *testing.T) {
	runs := []extract.Run{
		{Text: "third", X: 50, Y: 80, Width: 25, Size: 10, Space: 5},
		{Text: "second", X: 50, Y: 100, Width: 30, Size: 10, Space: 5},
		{Text: "first", X: 10, Y: 100, Width: 25, Size: 10, Space: 5},
	}
	if got := wordsUnder(runs, [4]float64{0, 70, 200, 115}); got != "first second third" {
		t.Errorf("read in drawing order: %q", got)
	}
}

func TestLinkTitleNamesWhatItCan(t *testing.T) {
	for _, c := range []struct {
		name string
		link ops.Link
		want string
	}{
		{"an address", ops.Link{URI: "https://example.org/"}, "https://example.org/"},
		{"a page", ops.Link{Target: 7}, "Page 7"},
		{"neither", ops.Link{}, "(a link that leads nowhere)"},
	} {
		if got := linkTitle(nil, c.link); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestNoDocumentHasNoLinks(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	if got := s.linkList(nil); got != nil {
		t.Errorf("an empty workbench listed %d links", len(got))
	}
}
