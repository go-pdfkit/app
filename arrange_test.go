package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// countedPDF is a document whose pages can be told apart: each draws its own
// number as text, so the ORDER of the pages is readable out of the written
// file rather than inferred from their count.
func countedPDF(t *testing.T, n int) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
	font := w.Add(reader.Dict{"Type": reader.Name("Font"),
		"Subtype": reader.Name("Type1"), "BaseFont": reader.Name("Helvetica")})
	kids := make(reader.Array, 0, n)
	for i := 1; i <= n; i++ {
		body := w.Add(&reader.Stream{Dict: reader.Dict{},
			Raw: []byte("BT /F1 12 Tf 20 100 Td (page " + strconv.Itoa(i) + ") Tj ET")})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pages, "Contents": body,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}))
	}
	w.Put(pages, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count": reader.Integer(n),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(200), reader.Integer(300)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pages})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// pageOrder is which page of the ORIGINAL document each page of the document as
// it would be saved is, read out of what each one draws.
func pageOrder(t *testing.T, s *state) []int {
	t.Helper()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatalf("the document could not be written and read back: %s", msg)
	}
	var out []int
	for i := 1; i <= src.PageCount(); i++ {
		data, err := src.PageContent(i)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		was := 0
		for n := 1; n <= 20; n++ {
			if strings.Contains(string(data), "(page "+strconv.Itoa(n)+")") {
				was = n
				break
			}
		}
		if was == 0 {
			t.Fatalf("page %d says which page it was nowhere: %s", i, data)
		}
		out = append(out, was)
	}
	return out
}

func orderOf(pages []int) string {
	var b strings.Builder
	for i, p := range pages {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Itoa(p))
	}
	return b.String()
}

// arranged opens a document of n tellable-apart pages with the grid showing.
func arranged(t *testing.T, n int) *state {
	t.Helper()
	h := &fakeHost{name: "counted.pdf", file: countedPDF(t, n)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	s.arrangePages()
	if s.tools.reading.Get() != readingArrange {
		t.Fatalf("the grid did not open: the reading is %q", s.tools.reading.Get())
	}
	return s
}

func TestTheGridShowsEveryPageAndSaysWhatToDo(t *testing.T) {
	s := arranged(t, 6)
	s.draw(buffer())

	// ⛔ The tiles of the GRID, which is what the band holds — the rail has its
	// own and this walk would find both, so it walks the band alone.
	got := tilesIn(s.shown)
	if len(got) != 6 {
		t.Fatalf("the grid shows %d tiles for six pages", len(got))
	}
	for i, th := range got {
		if th.Label != strconv.Itoa(i+1) {
			t.Errorf("tile %d is labelled %q", i+1, th.Label)
		}
		if th.Selected().Get() {
			t.Errorf("tile %d is lit with nothing picked up", i+1)
		}
	}
	if said := joined(s.shown); !strings.Contains(said, "Press one to pick it up") {
		t.Errorf("the grid does not say what to do; it says:\n%s", said)
	}
}

func TestPressingTheGridTwiceOnOnePagePutsItBack(t *testing.T) {
	s := arranged(t, 4)

	s.pressArranged(3)
	if s.held != 3 {
		t.Fatalf("page 3 was pressed and %d is held", s.held)
	}
	if !strings.Contains(s.note, "page 3 picked up") {
		t.Errorf("the status line says %q", s.note)
	}
	s.draw(buffer())
	if got := tilesIn(s.shown); !got[2].Selected().Get() {
		t.Error("the page being held is not lit")
	}
	if said := joined(s.shown); !strings.Contains(said, "Page 3 is held") {
		t.Errorf("the grid does not say what is held; it says:\n%s", said)
	}

	s.pressArranged(3)
	if s.held != 0 {
		t.Errorf("pressing it again left %d held", s.held)
	}
	if !strings.Contains(s.note, "page 3 put back") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := orderOf(pageOrder(t, s)); got != "1 2 3 4" {
		t.Errorf("picking a page up and putting it back reordered the document: %s", got)
	}
}

// TestAPageMovedForwardsLandsInFRONTOfTheOnePressed is the off-by-one this verb
// exists to get right. ops.Move takes the page OUT and then puts it back at the
// number given, so asking for the number pressed would leave the page AFTER it
// — and a reader who pressed the fifth tile meant "before this one".
func TestAPageMovedForwardsLandsInFRONTOfTheOnePressed(t *testing.T) {
	s := arranged(t, 6)
	if got := orderOf(pageOrder(t, s)); got != "1 2 3 4 5 6" {
		t.Fatalf("precondition: the document starts as %s", got)
	}

	s.pressArranged(2)
	s.pressArranged(5)

	if got := orderOf(pageOrder(t, s)); got != "1 3 4 2 5 6" {
		t.Errorf("moving page 2 in front of page 5 gave %s, want 1 3 4 2 5 6", got)
	}
	if s.held != 0 {
		t.Errorf("%d is still held after the move", s.held)
	}
	// It says where the page landed, not only where it was asked to go.
	if !strings.Contains(s.note, "where it is now page 4") {
		t.Errorf("the status line says %q", s.note)
	}
	// And the view follows the page it moved, as `Move it there` does.
	if s.at != 4 {
		t.Errorf("the workbench is on page %d", s.at)
	}
}

// TestAPageMovedBackwardsLandsWhereItWasPressed drives the whole gesture
// through PRESSES ON THE TILES, at the pixels the grid drew them at.
//
// ⛔ Not through pressArranged. Everything else here calls that directly, which
// says what the gesture does and nothing about whether the thing on the screen
// is wired to it: a tile whose OnClick was never set would pass every test
// above. This one presses where the tile is.
func TestAPageMovedBackwardsLandsWhereItWasPressed(t *testing.T) {
	s := arranged(t, 6)
	s.draw(buffer())
	tiles := tilesIn(s.shown)
	if len(tiles) != 6 {
		t.Fatalf("the grid shows %d tiles", len(tiles))
	}

	pressTile(t, s, tiles[4]) // page 5
	if s.held != 5 {
		t.Fatalf("pressing the fifth tile held %d", s.held)
	}
	// ⛔ The tiles are rebuilt by the refresh that press ended in, so the second
	// press has to be aimed at the NEW ones: the widgets above are no longer the
	// ones on the screen, and pressing a stale rectangle is how a test passes
	// against a view that has moved.
	s.draw(buffer())
	pressTile(t, s, tilesIn(s.shown)[1]) // page 2

	if got := orderOf(pageOrder(t, s)); got != "1 5 2 3 4 6" {
		t.Errorf("moving page 5 in front of page 2 gave %s, want 1 5 2 3 4 6", got)
	}
	if s.at != 2 {
		t.Errorf("the workbench is on page %d", s.at)
	}
}

// pressTile presses the middle of a tile, where the grid drew it.
func pressTile(t *testing.T, s *state, th *toolkit.Thumbnail) {
	t.Helper()
	r := th.Bounds()
	if r.W <= 0 || r.H <= 0 {
		t.Fatalf("the tile labelled %q has no rectangle", th.Label)
	}
	press(s, r.X+r.W/2, r.Y+r.H/2)
}

// TestAPageAlreadyInFrontOfTheOnePressedIsLeftAlone: moving page 3 in front of
// page 4 is where it already is, and a verb that reported a move would be
// telling somebody their document changed when it did not.
func TestAPageAlreadyInFrontOfTheOnePressedIsLeftAlone(t *testing.T) {
	s := arranged(t, 5)

	s.pressArranged(3)
	s.pressArranged(4)

	if !strings.Contains(s.note, "already in front of page 4") {
		t.Errorf("the status line says %q", s.note)
	}
	if s.held != 0 {
		t.Errorf("%d is still held", s.held)
	}
	if got := orderOf(pageOrder(t, s)); got != "1 2 3 4 5" {
		t.Errorf("the document came back as %s", got)
	}
}

// TestLeavingTheGridLetsGoOfWhatWasHeld: a page picked up in a view that is no
// longer on the screen is a move nobody can finish, and a held page that
// survived would be dropped by the next press on a grid opened much later.
func TestLeavingTheGridLetsGoOfWhatWasHeld(t *testing.T) {
	s := arranged(t, 4)
	s.pressArranged(2)
	if s.held != 2 {
		t.Fatalf("%d is held", s.held)
	}

	s.arrangePages() // the same verb again puts the page back on the screen

	if s.tools.reading.Get() == readingArrange {
		t.Error("pressing the verb again did not put the page back")
	}
	if s.held != 0 {
		t.Errorf("leaving the grid left page %d held", s.held)
	}
}

// TestPickingAPageUpMakesItThePageTheVerbsActOn is what makes the grid a place
// to work rather than a place to look: Rotate, Delete and the rest act on "the
// page", and without this a reader who could SEE the upside-down page in the
// grid had to find it again in the rail to turn it.
func TestPickingAPageUpMakesItThePageTheVerbsActOn(t *testing.T) {
	s := arranged(t, 5)
	if s.at != 1 {
		t.Fatalf("precondition: the workbench is on page %d", s.at)
	}

	s.pressArranged(4)

	if s.at != 4 {
		t.Errorf("picking page 4 up left the workbench on page %d", s.at)
	}
	// ⛔ AND A VERB PUTS IT DOWN, because a change to the document can mean the
	// page that number named is not that page any more: the press after a
	// deletion would otherwise move something nobody pointed at.
	s.rotate()
	if s.held != 0 {
		t.Errorf("turning the page left %d held", s.held)
	}
	if rot, err := s.doc.Rotation(4); err != nil || rot != 90 {
		t.Errorf("page 4 is turned %d degrees (%v); the verb acted on another page", rot, err)
	}
}

// TestThePageMenuOpensOverAGridTile is the rest of what a multi-tool grid is
// for: turning or dropping the page you can SEE, without leaving the view that
// shows you which it is.
//
// ⛔ The grid REPLACES the pages with tiles, so without this the one view that
// shows every page at once was the one view where the page menu did not open.
func TestThePageMenuOpensOverAGridTile(t *testing.T) {
	s := arranged(t, 5)
	s.draw(buffer())
	tiles := tilesIn(s.shown)
	if len(tiles) != 5 {
		t.Fatalf("the grid shows %d tiles", len(tiles))
	}
	r := tiles[3].Bounds() // page 4

	if !s.handleContext(r.X+r.W/2, r.Y+r.H/2) {
		t.Fatal("a secondary press on a grid tile was not answered")
	}
	if s.menu == nil {
		t.Fatal("no menu opened over a grid tile")
	}
	got := menuLabels(s)
	if len(got) == 0 || got[0] != "Page 4" {
		t.Fatalf("the menu offers %v; its first row should name page 4", got)
	}
	// ⛔ And the menu went to that page first, so its verbs — and the ribbon's —
	// act on the tile that was pressed rather than on whatever was being read.
	if s.at != 4 {
		t.Errorf("the menu says page 4 and the workbench is on %d", s.at)
	}
	// The grid is still the view: a menu is not a reason to put it away.
	if s.tools.reading.Get() != readingArrange {
		t.Errorf("opening the menu left the reading as %q", s.tools.reading.Get())
	}
}

// TestASecondaryPressAwayFromEveryTileOpensNothing: a menu about a page has to
// be about a page.
func TestASecondaryPressAwayFromEveryTileOpensNothing(t *testing.T) {
	s := arranged(t, 3)
	s.draw(buffer())

	// The strip of ribbon at the very top is over no tile and no minipage.
	if !s.handleContext(surfaceW/2, 2) {
		t.Fatal("a secondary press must be answered wherever it lands")
	}
	if s.menu != nil {
		t.Errorf("a menu opened over the ribbon, offering %v", menuLabels(s))
	}
}

// TestTheGridIsGreyedForADocumentOfOnePage: there is nothing to arrange.
func TestTheGridIsGreyedForADocumentOfOnePage(t *testing.T) {
	s, _ := opened(t, 1)
	if !greyed(t, s, "arrange") {
		t.Error("a document of one page offers the arranging grid")
	}
	s2, _ := opened(t, 2)
	if greyed(t, s2, "arrange") {
		t.Error("a document of two pages does not offer the arranging grid")
	}
}
