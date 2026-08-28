package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// The heights of the rows of each group, in the order they are built, so that
// a test can press a control where it was drawn rather than call its handler.
// Pressing is the whole point: it is what says the thing on the screen is
// wired to the thing it claims to be.
var (
	pagesRows = []int{labelledH, bareH, bareH, bareH, labelledH, bareH,
		labelledH, bareH, labelledH, bareH, labelledH, bareH}
	sheetRows = []int{labelledH, bareH, bareH, bareH, bareH}
)

// openGroup shows a group and lays it out, which is what gives its controls
// the bounds a press is tested against.
func openGroup(t *testing.T, s *state, name string) {
	t.Helper()
	s.showGroup(name)
	s.draw(buffer())
	if s.tools.open != name {
		t.Fatalf("the %s group did not open", name)
	}
}

// rowAt is a point inside the nth row of the panel now open, across is where
// along it: 0 is the left quarter and 1 the middle.
func rowAt(t *testing.T, s *state, rows []int, n int, across int) (int, int) {
	t.Helper()
	b := s.tools.built[s.tools.open].Bounds()
	if b.W == 0 {
		t.Fatal("the panel has no bounds, so nothing can be pressed on it")
	}
	y := b.Y
	for i := 0; i < n; i++ {
		y += rows[i] + rowGap
	}
	x := b.X + b.W/2
	if across == 0 {
		x = b.X + b.W/4
	}
	return x, y + rows[n]/2
}

// plusAt is the + of the spin button in the nth row, which is at the right
// hand end of the control, under the row's name.
func plusAt(t *testing.T, s *state, rows []int, n int) (int, int) {
	t.Helper()
	x, y := rowAt(t, s, rows, n, 1)
	return x + 120, y - 8
}

// press puts a press on the panel and redraws, the way a frame follows an
// event in the browser.
func press(s *state, x, y int) {
	s.handleClick(x, y)
	s.draw(buffer())
}

func TestTheStripOpensAndClosesEachGroup(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	// Every group is reachable by pressing its name on the strip, and
	// pressing it again puts it away.
	for _, name := range groupNames {
		var at int
		for x := margin; x < surfaceW-margin; x += 2 {
			s.handleClick(x, margin+toolbarH/2)
			if s.tools.open == name {
				at = x
				break
			}
		}
		if at == 0 {
			t.Fatalf("no press on the strip opened %q", name)
		}
		s.draw(buffer())
		if s.pageW() >= viewW {
			t.Error("the page kept the whole width with a panel beside it")
		}
		s.handleClick(at, margin+toolbarH/2)
		if s.tools.open != "" {
			t.Errorf("pressing %q again left it open", name)
		}
	}
}

func TestEveryControlInThePagesPanelIsWiredToItsVerb(t *testing.T) {
	s, _ := opened(t, 6)
	openGroup(t, s, groupPages)

	// The box takes what is typed into it, one character at a time, and an
	// arrow key then belongs to the box rather than to the pages.
	x, y := rowAt(t, s, pagesRows, 0, 1)
	press(s, x, y)
	if !s.editing() {
		t.Fatal("pressing the box did not put the caret in it")
	}
	for _, c := range []string{"2", "-", "3"} {
		if !s.handleChar(c) {
			t.Fatal("a character was refused by the box that has the caret")
		}
	}
	if s.tools.spec != "2-3" {
		t.Fatalf("the box holds %q", s.tools.spec)
	}
	at := s.at
	if !s.handleKeyDown("ArrowRight") || s.at != at {
		t.Error("an arrow key turned the page while a box was being typed into")
	}
	if !s.handleKeyDown("Backspace") || s.tools.spec != "2-" {
		t.Errorf("after a backspace the box holds %q", s.tools.spec)
	}
	s.tools.spec = "2-3"

	// Turning: the left half of that row cycles how far, the right half does
	// it. One press of the cycle takes a quarter turn to a half.
	x, y = rowAt(t, s, pagesRows, 2, 0)
	press(s, x, y)
	if s.tools.turn != 180 {
		t.Fatalf("the turn is %d degrees", s.tools.turn)
	}
	x, y = rowAt(t, s, pagesRows, 2, 1)
	press(s, x, y)
	if got, _ := s.doc.Rotation(2); got != 180 {
		t.Errorf("page two is turned %d degrees", got)
	}
	if got, _ := s.doc.Rotation(1); got != 0 {
		t.Errorf("page one was turned too, to %d", got)
	}

	// Reversing, which needs no telling.
	x, y = rowAt(t, s, pagesRows, 3, 1)
	press(s, x, y)
	if got, _ := s.doc.Rotation(5); got != 180 {
		t.Error("the order was not reversed: the turned page did not move")
	}

	// Keeping only the pages the box names.
	x, y = rowAt(t, s, pagesRows, 1, 0)
	press(s, x, y)
	if s.doc.PageCount() != 2 {
		t.Errorf("keeping 2-3 left %d pages", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "2-3") {
		t.Errorf("the status line says %q", s.note)
	}

	// And dropping them.
	s.tools.spec = "1"
	x, y = rowAt(t, s, pagesRows, 1, 1)
	press(s, x, y)
	if s.doc.PageCount() != 1 {
		t.Errorf("deleting page one left %d pages", s.doc.PageCount())
	}
}

func TestMovingCroppingBlankingAndSplittingFromThePanel(t *testing.T) {
	s, h := opened(t, 4)
	openGroup(t, s, groupPages)

	// A number is pressed up before the verb beside it is pressed.
	x, y := plusAt(t, s, pagesRows, 4)
	press(s, x, y)
	if s.tools.moveTo != 2 {
		t.Fatalf("the spin button says %d", s.tools.moveTo)
	}
	s.at = 1
	x, y = rowAt(t, s, pagesRows, 5, 1)
	press(s, x, y)
	if s.at != 2 {
		t.Errorf("the view did not follow the page it moved, and shows %d", s.at)
	}
	if !strings.Contains(s.note, "moved") {
		t.Errorf("the status line says %q", s.note)
	}

	// Cropping: what is typed is a box, and what is drawn afterwards is the
	// page as it would be saved.
	x, y = rowAt(t, s, pagesRows, 6, 1)
	press(s, x, y)
	for _, c := range strings.Split("0,0,150,200", "") {
		if !s.handleChar(c) {
			t.Fatal("a character was refused by the crop box")
		}
	}
	if s.tools.box != "0,0,150,200" {
		t.Fatalf("the crop box holds %q", s.tools.box)
	}
	before := s.fitScale(s.src)
	x, y = rowAt(t, s, pagesRows, 7, 1)
	press(s, x, y)
	if s.fitScale(s.src) <= before {
		t.Error("cropping the page did not change how it is drawn")
	}

	// A blank page goes in where the number says.
	was := s.doc.PageCount()
	x, y = plusAt(t, s, pagesRows, 8)
	press(s, x, y)
	if s.tools.before != 2 {
		t.Fatalf("the blank page is to go before %d", s.tools.before)
	}
	x, y = rowAt(t, s, pagesRows, 9, 1)
	press(s, x, y)
	if s.doc.PageCount() != was+1 {
		t.Errorf("%d pages after inserting a blank one", s.doc.PageCount())
	}

	// Splitting hands over one file per piece and changes nothing. Two pages
	// to a piece, pressed up from one.
	pages := s.doc.PageCount()
	x, y = plusAt(t, s, pagesRows, 10)
	press(s, x, y)
	if s.tools.every != 2 {
		t.Fatalf("a piece is to hold %d pages", s.tools.every)
	}
	x, y = rowAt(t, s, pagesRows, 11, 1)
	press(s, x, y)
	if s.doc.PageCount() != pages {
		t.Error("splitting changed the document it was asked about")
	}
	want := (pages + 1) / 2
	if h.savedAll != want {
		t.Errorf("%d files were handed over for %d pages, two to a piece", h.savedAll, pages)
	}
	if h.as != partName("sample.pdf", want) {
		t.Errorf("the last piece was called %q", h.as)
	}
	// The last piece holds whatever was left over, and reads back as a
	// document of its own.
	last := pages - 2*(want-1)
	if back, err := reader.Open(h.saved); err != nil || back.PageCount() != last {
		t.Errorf("the last piece does not read back as %d pages: %v", last, err)
	}
}

func TestTheSheetPanel(t *testing.T) {
	s, _ := opened(t, 4)
	openGroup(t, s, groupSheet)

	// Two to a sheet is what the number starts at; pressed up it is three,
	// which puts four pages on two sheets.
	x, y := plusAt(t, s, sheetRows, 0)
	press(s, x, y)
	if s.tools.up != 3 {
		t.Fatalf("the number says %d to a sheet", s.tools.up)
	}
	s.tools.up = 2
	x, y = rowAt(t, s, sheetRows, 1, 1)
	press(s, x, y)
	if s.doc.PageCount() != 2 || s.at != 1 {
		t.Errorf("%d sheets, showing %d", s.doc.PageCount(), s.at)
	}

	// A booklet reorders the sheets rather than adding any.
	s2, _ := opened(t, 4)
	openGroup(t, s2, groupSheet)
	x, y = rowAt(t, s2, sheetRows, 2, 1)
	press(s2, x, y)
	if s2.doc.PageCount() != 2 {
		t.Errorf("a booklet of four pages came to %d sheets", s2.doc.PageCount())
	}
}

func TestAddingAndOverlayingAnotherFile(t *testing.T) {
	s, h := opened(t, 2)
	openGroup(t, s, groupSheet)
	h.file = samplePDF(t, 3)

	x, y := rowAt(t, s, sheetRows, 3, 1)
	press(s, x, y)
	if s.doc.PageCount() != 5 {
		t.Errorf("adding a three page file to a two page one gave %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "added") {
		t.Errorf("the status line says %q", s.note)
	}

	x, y = rowAt(t, s, sheetRows, 4, 1)
	press(s, x, y)
	if s.doc.PageCount() != 5 {
		t.Errorf("laying a file over this one changed the page count to %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "laid over") {
		t.Errorf("the status line says %q", s.note)
	}

	// A second file that is not a PDF is said so, and changes nothing.
	h.file = []byte("this is not a PDF")
	press(s, x, y)
	if !strings.Contains(s.note, "cannot open") {
		t.Errorf("the status line says %q", s.note)
	}
	// And one the person did not choose leaves the document alone.
	h.file = nil
	press(s, x, y)
	if s.doc.PageCount() != 5 {
		t.Errorf("changing one's mind about a second file gave %d pages", s.doc.PageCount())
	}
}

func TestOnlyTheBoxLastPressedTakesWhatIsTyped(t *testing.T) {
	// Two boxes in one panel: the range and the crop box. A press on the
	// second has to take the caret off the first, or every letter meant for
	// the second goes into the first — which is what happened, because the
	// toolkit's own focus walk cannot see into a scroll view.
	s, _ := opened(t, 3)
	openGroup(t, s, groupPages)
	x, y := rowAt(t, s, pagesRows, 0, 1)
	press(s, x, y)
	for _, c := range []string{"1", "-", "2"} {
		s.handleChar(c)
	}
	x, y = rowAt(t, s, pagesRows, 6, 1)
	press(s, x, y)
	for _, c := range []string{"0", ",", "0", ",", "9", ",", "9"} {
		s.handleChar(c)
	}
	if s.tools.spec != "1-2" {
		t.Errorf("the range box holds %q", s.tools.spec)
	}
	if s.tools.box != "0,0,9,9" {
		t.Errorf("the crop box holds %q", s.tools.box)
	}
	// And a press that lands on neither leaves both of them alone.
	press(s, margin+2, viewTop+viewH-2)
	if s.editing() {
		t.Error("a press on nothing left a box with the caret")
	}
}

func TestAPanelPutAwayLetsGoOfTheKeys(t *testing.T) {
	s, _ := opened(t, 3)
	openGroup(t, s, groupPages)
	x, y := rowAt(t, s, pagesRows, 0, 1)
	press(s, x, y)
	if !s.editing() {
		t.Fatal("the caret is not in the box")
	}
	// Putting the panel away, or opening the form panel instead, takes the
	// caret out of a box nobody can see any more.
	s.showGroup(groupPages)
	if s.editing() {
		t.Error("a box nobody can see still has the caret")
	}
	if !s.handleKeyDown("ArrowRight") || s.at != 2 {
		t.Errorf("the arrow keys did not come back to the pages: page %d", s.at)
	}
	// The panel is built once: what was typed into it is still there when it
	// comes back.
	openGroup(t, s, groupPages)
	if s.tools.built[groupPages] == nil {
		t.Error("the panel was not kept")
	}
}

func TestTheFormPanelClosesAnyGroup(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{name: "form.pdf", file: formPDF(t)})
	s.open()
	openGroup(t, s, groupPages)
	s.showForm()
	if s.tools.open != "" || !s.showingForm {
		t.Errorf("group %q open, form %v", s.tools.open, s.showingForm)
	}
	// And opening a group puts the form away again.
	s.showGroup(groupSheet)
	if s.showingForm {
		t.Error("the form stayed up beside a group of verbs")
	}
}

func TestWhatAVerbSaysWhenItCannotRun(t *testing.T) {
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	for _, act := range []func(){
		empty.selectPages, empty.deleteRange, empty.turnRange, empty.reverse,
		empty.movePage, empty.crop, empty.insertBlank, empty.split,
		empty.nUp, empty.booklet, empty.merge, empty.overlay, empty.watermark,
	} {
		empty.note = ""
		empty.tools.box = "0,0,10,10"
		act()
		if empty.note == "" {
			t.Error("a verb with no document said nothing")
		}
	}

	s, _ := opened(t, 3)
	// A range that names every page is refused: a document needs one.
	s.tools.spec = "1-3,3"
	s.deleteRange()
	if s.doc.PageCount() != 3 {
		t.Errorf("every page was deleted, leaving %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "needs one") {
		t.Errorf("the status line says %q", s.note)
	}
	// A range that is not a range at all is the operation's to complain about.
	s.tools.spec = "nonsense"
	s.deleteRange()
	if s.note == "" || s.doc.PageCount() != 3 {
		t.Errorf("a nonsense range said %q and left %d pages", s.note, s.doc.PageCount())
	}
	// Moving a page nowhere it can go.
	s.tools.moveTo = 99
	s.at = 1
	s.movePage()
	if s.at != 1 {
		t.Errorf("the view followed a move that did not happen, to %d", s.at)
	}
	// Laying out no pages to a sheet, and folding a document that cannot be.
	s.tools.up = 0
	s.nUp()
	if s.doc.PageCount() != 3 {
		t.Errorf("a nonsense n-up left %d pages", s.doc.PageCount())
	}
	s.tools.every = 0
	s.split()
	if s.note == "" {
		t.Error("a nonsense split said nothing")
	}
}

func TestABookletThatCannotBeFolded(t *testing.T) {
	// Booklet needs a document it can pair the pages of; one that cannot be
	// folded says so rather than being folded wrongly.
	s, _ := opened(t, 1)
	s.doc, _ = ops.Open(samplePDF(t, 1))
	if err := s.doc.Delete("1"); err != nil {
		t.Fatal(err)
	}
	s.booklet()
	if s.note == "" {
		t.Error("a booklet that could not be folded said nothing")
	}
}

func TestReadingACropBox(t *testing.T) {
	if _, err := parseBox("1,2,3"); err == nil {
		t.Error("three numbers were taken for a box")
	}
	if _, err := parseBox("1,2,3,x"); err == nil {
		t.Error("a box was read out of something that is not a number")
	}
	box, err := parseBox(" 1 , 2 ,3, 4 ")
	if err != nil || box != [4]float64{1, 2, 3, 4} {
		t.Errorf("parseBox gave %v, %v", box, err)
	}
	s, _ := opened(t, 2)
	s.tools.box = "not a box"
	s.crop()
	if s.note == "" {
		t.Error("a crop box that cannot be read said nothing")
	}
}

func TestWhatAPieceOfASplitDocumentIsCalled(t *testing.T) {
	for in, want := range map[string]string{
		"report.pdf": "report-001.pdf",
		"report":     "report-001.pdf",
		"":           "document.pdf-001.pdf",
	} {
		if got := partName(in, 1); got != want {
			t.Errorf("partName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAPieceThatCannotBeWritten(t *testing.T) {
	// None of this should ever happen to a document that opened, which is
	// exactly why it is worth being able to see what the workbench does.
	s, h := opened(t, 4)
	was := docBytes
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errors.New("no") }
	defer func() { docBytes = was }()
	s.tools.every = 1
	s.split()
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("the status line says %q", s.note)
	}
	if h.savedAll != 0 {
		t.Errorf("%d pieces were handed over anyway", h.savedAll)
	}
}
