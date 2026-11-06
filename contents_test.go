// What the contents pane COSTS and where its text LANDS.
//
// ⛔⛔ THREE DEFECTS REPORTED AT ONCE, BY SOMEBODY RUNNING THE BUILD THAT ADDED
// THE VIEW: "resizing the bars is very slow, I have the impression the whole
// tab lags; the table of contents is unusable as it is; it looks right-aligned
// instead of left". All three were in the view I had just shipped, and all
// three are asserted here as PROPERTIES rather than as the shapes of the
// widgets that happen to hold them — a test that named toolkit.Button would
// have passed on the centred rows that caused the third report.

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// paintPane lays the contents pane out at the rail's real width, paints it at
// the origin and hands back the pixels.
//
// ⛔ PIXELS, because the rows are a ListBox and a ListBox reports no text runs:
// it draws its own rows rather than holding a widget per row, which is exactly
// why it is used here. A test that asked the widget tree where its text was
// would find nothing and pass. Where the ink LANDS is the thing the person who
// reported this was looking at.
func paintPane(t *testing.T, s *state, rows []bookmark) ([]byte, int, int) {
	t.Helper()
	w, h := s.railPx(), toolkit.Scaled(contentsLogicalH)
	pane := s.contentsPane(rows)
	pane.SetBounds(painter.Rect{W: w, H: h})
	buf := make([]byte, s.w*s.h*4)
	fillBG(buf, s.theme.Background)
	pane.Draw(painter.NewPixelPainter(buf, s.w, s.h), s.theme)
	return buf, w, h
}

// inkSpan is the first and last column of row i that is not the colour of that
// row's own far right end — its text, in other words, measured against the
// background the row itself is painted on.
//
// ⛔ Against the ROW's own background and not a constant: a selected row is
// filled with the accent colour, and the theme may be light or dark. A test
// that knew one background passed in CI and was wrong on the display the
// person was actually looking at — the lesson a pixel test in this repo has
// already cost once.
func inkSpan(buf []byte, surfW, rowH, paneW, row int) (first, last int) {
	first, last = -1, -1
	y := row*rowH + rowH/2
	at := func(x int) [4]byte {
		i := (y*surfW + x) * 4
		return [4]byte{buf[i], buf[i+1], buf[i+2], buf[i+3]}
	}
	bg := at(paneW - 2)
	for x := range paneW - 2 {
		if at(x) != bg {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	return first, last
}

// listBoxOf is the pane as the list it is, or a failure saying what it became.
func listBoxOf(t *testing.T, w toolkit.Widget) *toolkit.ListBox {
	t.Helper()
	lb, ok := w.(*toolkit.ListBox)
	if !ok {
		t.Fatalf("the contents pane is a %T, so its rows are not a list", w)
	}
	return lb
}

// ⛔ A row that leads nowhere keeps its place and does nothing. Dropping it
// would renumber every row under it, and the index a press arrives with is an
// index into the list the reader is looking at.
func TestARowThatLeadsNowhereDoesNothing(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	was := s.at
	rows := []bookmark{{title: "… and 25 more links, not shown"}}
	listBoxOf(t, s.contentsPane(rows)).OnActivate(0)
	if s.at != was {
		t.Errorf("pressing a row that leads nowhere moved the reader to page %d", s.at)
	}
	// And an index no row answers to is refused rather than panicking: the list
	// is rebuilt as the reader moves, and a press can arrive for the list that
	// was there a frame ago.
	listBoxOf(t, s.contentsPane(rows)).OnActivate(7)
}

// ⛔⛔ THE LAG, COUNTED. marksFor walks the outline once per DOCUMENT, with a
// comment saying why; the links fallback it sits beside had no such memo, so
// every frame that rebuilt the rail read the text of every page carrying a
// link — and the rail is rebuilt on every page change and on every frame while
// the minipages are still filling.
//
// Counted rather than timed, for the reason cost_test.go gives: this machine
// runs two dozen other things, so a count is exact where a duration is not.
func TestTheLinksAreReadOncePerDocumentNotOncePerFrame(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())

	calls := 0
	was := extractRuns
	extractRuns = func(src *reader.Document, page int) ([]extract.Run, error) {
		calls++
		return was(src, page)
	}
	t.Cleanup(func() { extractRuns = was })

	const frames = 30
	for range frames {
		s.rail(s.src, s.thumbs.stamp)
	}
	// The fixture carries its three links on one page, so the whole list costs
	// ONE read of one page's text — once.
	if calls > 1 {
		t.Errorf("%d frames read the pages' text %d times; the links are re-read on every frame",
			frames, calls)
	}
}

// ⛔ THE READER'S OWN GESTURE, on a document that actually carries links. The
// drag was already guarded against writing the document — that was measured
// and is held by TestDraggingTheDividerCostsNothingPerFrame — but that sample
// carries no links at all, so it exercised the page-list fallback and could
// not see this cost. Every width change throws the rail away, and putting it
// back read the text of every linked page again.
func TestDraggingTheDividerDoesNotReReadTheLinks(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("there is no divider beside the page")
	}
	r, ok := sp.HandleRect(0)
	if !ok {
		t.Fatalf("the divider has no rectangle to grab: %+v", r)
	}

	calls := 0
	was := extractRuns
	extractRuns = func(src *reader.Document, page int) ([]extract.Run, error) {
		calls++
		return was(src, page)
	}
	t.Cleanup(func() { extractRuns = was })

	x, y := r.X+r.W/2, r.Y+r.H/2
	s.handleClick(x, y)
	for i := 1; i <= 10; i++ {
		s.handleMove(x+2*i, y)
		s.tick(0.016)
		s.draw(buffer())
	}
	s.handleRelease(x+20, y)
	// The frames AFTER the gesture matter as much: letting go is what puts the
	// rail back, and that is where the cost landed.
	for range 5 {
		s.tick(0.016)
		s.draw(buffer())
	}
	if calls != 0 {
		t.Errorf("dragging the divider re-read the document's text %d times", calls)
	}
}

// ⛔ And a document whose bytes CHANGE is read again: a memo that never lets go
// is a memo that shows the previous document's contents.
func TestTheLinksAreReadAgainWhenTheDocumentChanges(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())

	calls := 0
	was := extractRuns
	extractRuns = func(src *reader.Document, page int) ([]extract.Run, error) {
		calls++
		return was(src, page)
	}
	t.Cleanup(func() { extractRuns = was })

	// Two stamps neither pass has seen. The first frame drawn above has already
	// filled the memo for the document's own stamp — which is the memo doing
	// its work, and would make a measurement from here read zero.
	rows := s.contentsOf(s.src, [2]uint64{11, 11})
	first := calls
	if first == 0 || len(rows) == 0 {
		t.Fatalf("the first pass read nothing: %d call(s), %d row(s)", calls, len(rows))
	}
	// A different stamp is a different document.
	if s.contentsOf(s.src, [2]uint64{12, 12}); calls == first {
		t.Error("a different document got the previous one's contents")
	}
}

// ⛔⛔ A THOUSAND ROWS ARE NOT A THOUSAND WIDGETS, and this is the half of the
// lag that hits the COMMONEST documents. Most files carry no outline and no
// links, so the contents fall back to a row per page — and the rows were a
// VBox of toolkit.Buttons inside a ScrollView, built and laid out in full on
// every rebuild, which happens on every page change and on every frame while
// the minipages are still filling. A four-hundred-page scan was four hundred
// widgets, sixty times a second.
func TestALongContentsDoesNotBuildAWidgetPerRow(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	rows := make([]bookmark, 1000)
	for i := range rows {
		rows[i] = bookmark{title: fmt.Sprintf("Page %d", i+1), page: i%3 + 1}
	}
	// ⛔ THE CONTROL FOR THE INSTRUMENT. A walker that could not see children
	// would report one widget for anything at all and this test would pass
	// however the pane were built. So it is first shown counting the shape it
	// is here to rule out: a column with a row per entry.
	col := toolkit.NewVBox()
	for range 3 {
		col.AddFixed(toolkit.NewButton("a row", func() {}), lineH)
	}
	if n := widgetsIn(col); n != 4 {
		t.Fatalf("the walker counts %d widgets in a column of three rows, so it cannot "+
			"tell a column of a thousand from a list", n)
	}

	pane := s.contentsPane(rows)
	// The list draws its own rows, so its subtree does not grow with them. The
	// bound is generous on purpose: what it rules out is PROPORTIONAL, and a
	// handful either way is not the defect.
	if n := widgetsIn(pane); n > 8 {
		t.Errorf("a contents of %d rows is built from %d widgets", len(rows), n)
	}
	// And it still SAYS all of them: cheap must not mean truncated.
	if got := len(listBoxOf(t, pane).Items); got != len(rows) {
		t.Errorf("the list holds %d of %d rows", got, len(rows))
	}
}

// widgetsIn counts a widget and everything under it.
func widgetsIn(w toolkit.Widget) int {
	n := 1
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, c := range p.Children() {
			n += widgetsIn(c)
		}
	}
	return n
}

// ⛔⛔ "it looks right-aligned instead of left". It was CENTRED: a contents row
// was a toolkit.Button, and a Button is documented as "a clickable rectangle
// with a centred label". A column of centred rows of different lengths has no
// edge to read down, and a row wider than the rail overflows both ways, which
// is what reads as right-aligned.
//
// The property is where the TEXT lands, not what the row is made of.
func TestAContentsRowReadsFromTheLeftEdge(t *testing.T) {
	s := openedWith(t, linkedPDF(t))
	s.draw(buffer())
	// ⛔ A DIFFERENCE, not an absolute position. Two rows of very different
	// lengths: left-aligned they start at the same column, centred they do not
	// — and the gap between them grows with the difference in length, which is
	// what made a column of real headings look pushed to the right. Neither
	// page is the one being read, so neither row is the lit one.
	rows := []bookmark{
		{title: "A", page: 2},
		{title: "A heading of the length headings are really written at", page: 3},
	}
	buf, w, h := paintPane(t, s, rows)
	if h < 2*lineH {
		t.Fatalf("the pane is %d tall and a row is %d", h, lineH)
	}
	short, _ := inkSpan(buf, s.w, lineH, w, 0)
	long, _ := inkSpan(buf, s.w, lineH, w, 1)
	if short < 0 || long < 0 {
		t.Fatalf("a row drew no text at all: short %d, long %d", short, long)
	}
	if short != long {
		t.Errorf("a one-letter row starts at column %d and a long one at %d, "+
			"so the rows are not aligned on their left edge", short, long)
	}
	if short > contentsGutter {
		t.Errorf("the rows start %d pixels into a pane %d wide, past the %d-pixel gutter",
			short, w, contentsGutter)
	}
}

// ⛔⛔ "the table of contents is unusable as it is". A row was clipped to 200
// RUNES in a rail 132 logical pixels wide — something like fifteen times what
// fits. Every row overflowed, and a column of overflowing centred rows is a
// column of middles of sentences.
//
// The bound belongs to the PANE, not to a number chosen beside it.
func TestAContentsRowFitsThePaneItIsIn(t *testing.T) {
	s := openedWith(t, longLinkPDF(t))
	s.draw(buffer())
	rows := s.contentsOf(s.src, s.thumbs.stamp)
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	// The row's stored title is far longer than the rail: that is the case of
	// every real heading, and the one the reader saw.
	if n := len([]rune(rows[0].title)); n < 40 {
		t.Fatalf("the fixture's row is only %d characters, so it proves nothing", n)
	}
	buf, w, _ := paintPane(t, s, rows)
	first, last := inkSpan(buf, s.w, lineH, w, 0)
	if first < 0 {
		t.Fatal("the row drew no text at all")
	}
	if last >= w-scrollbarW {
		t.Errorf("a row's text runs to column %d in a pane %d wide (less a %d-pixel bar), "+
			"so it is cut off rather than fitted", last, w, scrollbarW)
	}
	// ⛔ And it SAYS it was cut, rather than stopping mid-word as if the heading
	// ended there.
	if !strings.HasSuffix(s.contentsLabel(rows[0]), outlineEllipsis) {
		t.Errorf("a row that did not fit reads %q, which does not say it was cut",
			s.contentsLabel(rows[0]))
	}
	// ⛔ And a WIDER rail shows MORE of the row. The bound belongs to the pane,
	// so dragging the divider must change what the row says — which a bound
	// counted in runes beside the code could never do.
	narrow := s.contentsLabel(rows[0])
	s.railWidth.Set(railMaxW)
	if wide := s.contentsLabel(rows[0]); len(wide) <= len(narrow) {
		t.Errorf("widening the rail from %d to %d left the row reading %q",
			railLogicalW, railMaxW, wide)
	}
}

// ⛔ A PANE WITH NO ROOM SAYS NOTHING, rather than something wrong. Both ends
// are reachable: the rail can be asked for a width below its own floor, and a
// pane can be narrower than a single character and its ellipsis. The answer in
// both cases is an empty row — a row holding half a letter, or an ellipsis
// alone pretending to be a cut title, would be worse than a blank one.
func TestARowWithNoRoomSaysNothing(t *testing.T) {
	for _, c := range []struct {
		what string
		px   int
	}{
		{"a rail dragged below its own floor", -40},
		{"a pane of no width at all", 0},
		{"a pane narrower than one character", 1},
	} {
		if got := clipToWidth("a heading", c.px); got != "" {
			t.Errorf("%s gave the row %q", c.what, got)
		}
	}
	// ⛔ The control: the same string in a pane that HAS room comes back whole.
	// Without it this test would pass on a clipToWidth that always said nothing.
	whole := "a heading"
	if got := clipToWidth(whole, toolkit.TextWidth(whole)); got != whole {
		t.Errorf("a row that fits exactly came back %q", got)
	}
}

// longLinkPDF is one page carrying one link over a line of text far longer than
// any rail: the case every row of a real document's contents is in.
func longLinkPDF(t *testing.T) []byte {
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
	refs := []reader.Ref{w.Reserve(), w.Reserve()}
	line := "A section heading that somebody wrote at the length headings are " +
		"really written at, which is longer than any rail is wide"
	for i, ref := range refs {
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
			fmt.Sprintf("BT /F1 10 Tf 20 300 Td (%s) Tj ET", line))})
		page := reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}
		if i == 0 {
			page["Annots"] = reader.Array{
				w.Add(reader.Dict{"Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(20), reader.Integer(298),
						reader.Integer(20 + 5*len(line)), reader.Integer(312)},
					"Dest": reader.Array{refs[1], reader.Name("Fit")}}),
			}
		}
		w.Put(ref, page)
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids":  reader.Array{refs[0], refs[1]},
		"Count": reader.Integer(len(refs)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(20 + 5*len(line) + 20), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
