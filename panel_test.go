package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// The heights of the rows of each group, in the order they are built, so that
// a test can press a control where it was drawn rather than call its handler.
// Pressing is the whole point: it is what says the thing on the screen is
// wired to the thing it claims to be.
//
// ⛔ FUNCTIONS, not vars. A row's height follows the installed face, and
// package-level vars are initialised BEFORE any init() runs -- so a table
// written as a var captured the bitmap heights, the panel was then laid out
// with the vector face installed by init(), and every press after the second
// row landed in the gap between two controls. Eleven tests failed on an
// off-by-a-font.

// ⛔ Every row of every panel is now a SETTING. The verbs moved to the ribbon,
// so these tables hold what the verbs are told and nothing else -- which is why
// they are mostly labelled rows: a setting has a caption, a button had none.

// ⛔⛔ THIS IS THE PAGES PANEL'S LAYOUT, WRITTEN DOWN TWICE. One entry per row,
// and the tests below press rows by their POSITION — so inserting a row in
// pagesGroup moves every row after it and the presses land on the wrong
// control. It has already happened: adding "Set the paper to" and "Paint the
// text" above the split row moved splitting from row 5 to row 7, and the
// failure read "row 5 holds 0 spin buttons" rather than naming the cause.
//
// The rows, in order, so the indices below can be read against something:
//
//	0 Which pages   1 Turn them by + or this angle   2 Move this page to
//	3 Crop to, in points   4 Put a blank page before   5 Set the paper to
//	6 Paint the text   7 Split into files of
//
// Finding a row by the label of the control on it would remove this copy
// altogether, and is worth doing the next time one of these presses lands
// somewhere unexpected. ⛔ It nearly happened again on 2026-11-09: the free
// angle was first given a row of its own, which moved four presses — and then
// the panel turned out to be FULL, so it went beside the cycle button instead
// and the indices never moved. The refactor is filed as app#108.
func pagesRows() []int {
	return []int{
		labelledH(), labelledH(), labelledH(), labelledH(),
		labelledH(), labelledH(), labelledH(), labelledH(),
	}
}

func sheetRows() []int {
	return []int{labelledH()}
}

// openGroup shows a group and lays it out, which is what gives its controls
// the bounds a press is tested against.
//
// It chooses the group's TAB, because that is the only way a panel opens now:
// there is no switch left to turn one on with, so a test that set the field
// directly would be testing an arrangement the workbench cannot reach.
func openGroup(t *testing.T, s *state, name string) {
	t.Helper()
	s.ribbon.ActiveTab().Set(tabIndexOf(t, name))
	s.draw(buffer())
	if s.tools.open != name {
		t.Fatalf("the %s group did not open", name)
	}
}

// tabIndexOf is where a group's tab is on the ribbon.
func tabIndexOf(t *testing.T, name string) int {
	t.Helper()
	for i, tab := range ribbonTabs {
		if tab.label == name {
			return i
		}
	}
	t.Fatalf("no tab is named %q", name)
	return 0
}

// pressVerb presses the ribbon control that runs the verb, on the tab that
// holds it.
//
// ⛔ PRESSED WHERE IT IS DRAWN, like every other control in this suite, and
// not by calling the handler: the point of the test is that the thing on the
// screen is wired to the thing it claims to be. ControlRect is the ribbon's own
// answer to where a control ended up, so the press follows the layout instead
// of a second copy of it.
func pressVerb(t *testing.T, s *state, id string) {
	t.Helper()
	v, ok := verbs[id]
	if !ok {
		t.Fatalf("there is no verb %q", id)
	}
	// A verb on the quick access bar is reached from wherever the ribbon
	// already is: that is what the bar is for, so the test does not change
	// tabs to press one.
	if tab, onATab := tabHolding(id); onATab {
		s.ribbon.ActiveTab().Set(tab)
	}
	s.draw(buffer())
	r, shown := s.ribbon.ControlRect(v.label)
	if !shown {
		if r, shown = s.ribbon.QuickRect(v.label); !shown {
			t.Fatalf("%q is neither on the ribbon nor on its quick access bar", v.label)
		}
	}
	press(s, r.X+r.W/2, r.Y+r.H/2)
}

// tabHolding is the tab a verb is on, and whether it is on one at all.
func tabHolding(id string) (int, bool) {
	for i, tab := range ribbonTabs {
		for _, g := range tab.groups {
			for _, have := range g.ids {
				if have == id {
					return i, true
				}
			}
		}
	}
	return 0, false
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

// kids is the toolkit's container convention: anything that holds widgets
// reports them, which is how the accessibility walk reads a tree it did not
// build. A test can use the same door.
type kids interface{ Children() []toolkit.Widget }

// spinsIn collects the spin buttons under a widget, in the order they are laid
// out.
func spinsIn(w toolkit.Widget) []*toolkit.SpinButton {
	var out []*toolkit.SpinButton
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if w == nil {
			return
		}
		if sp, ok := w.(*toolkit.SpinButton); ok {
			out = append(out, sp)
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(w)
	return out
}

// spinFor is the spin button of the labelled row that holds it.
//
// ⛔ BY LABEL, because a position in the panel is not a name. The alternative —
// `spinsIn(panel)[0]` — reads the control that happens to come first, so a row
// added above it silently moves every assertion onto the wrong number and the
// test goes on passing or fails somewhere unrelated. It is the same mistake
// pagesRows() documents for presses, and this is the half of it that can be
// fixed in one function.
func spinFor(t *testing.T, w toolkit.Widget, label string) *toolkit.SpinButton {
	t.Helper()
	var found *toolkit.SpinButton
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if w == nil || found != nil {
			return
		}
		if f, ok := w.(*toolkit.FormField); ok && f.Label == label {
			if sp := spinsIn(f); len(sp) > 0 {
				found = sp[0]
				return
			}
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(w)
	if found == nil {
		t.Fatalf("no row labelled %q carries a spin button", label)
	}
	return found
}

// dropsIn collects the drop-down lists under a widget.
func dropsIn(w toolkit.Widget) []*toolkit.DropDown {
	var out []*toolkit.DropDown
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if w == nil {
			return
		}
		if d, ok := w.(*toolkit.DropDown); ok {
			out = append(out, d)
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(w)
	return out
}

// chooseAt opens the list in the nth row and picks its which-th option, by
// pressing both -- which is the point: a chooser nobody can press is a chooser
// that does not work, however well its handler is wired.
//
// ⛔ The second press is placed from the list's OWN PopoverBounds and the
// toolkit's row height, not from the control plus 40 pixels. 40 was one and a
// half rows of bitmap text, and with a real face it fell through the list.
func chooseAt(t *testing.T, s *state, rows []int, n, which int) {
	t.Helper()
	x, y := rowAt(t, s, rows, n, 0)
	press(s, x, y)
	for _, d := range dropsIn(s.tools.built[s.tools.open]) {
		if !d.PopoverOpen() {
			continue
		}
		pb := d.PopoverBounds()
		press(s, pb.X+pb.W/2, pb.Y+which*toolkit.PopoverRowH+toolkit.PopoverRowH/2)
		return
	}
	t.Fatalf("pressing row %d of the %s panel opened no list", n, s.tools.open)
}

// plusAt is the point that makes the spin button in the nth row count up.
//
// ⛔ It ASKS THE WIDGET where it is. It used to be `x + 120`, and then
// `right - 30`, both measured against the toolkit's built-in 5x7 bitmap; the
// moment the workbench asked for a real typeface the controls changed width
// and the press landed beside the button instead of on it. A literal offset in
// a test is a second, undeclared copy of the layout, and it goes stale without
// saying so -- whereas Bounds() is the layout itself.
//
// The + is the top half of the stepper column at the right hand end of the
// control, which is what SpinButton.OnEvent tests: x past r.W less the stepper
// width, y above the half.
func plusAt(t *testing.T, s *state, rows []int, n int) (int, int) {
	t.Helper()
	return plusAtNth(t, s, rows, n, 0)
}

// plusAtNth is the same for a row that holds more than one, counting from the
// left: the Bates row carries the padding beside the first number.
func plusAtNth(t *testing.T, s *state, rows []int, n, which int) (int, int) {
	t.Helper()
	_, mid := rowAt(t, s, rows, n, 1)
	half := rows[n] / 2
	var on []*toolkit.SpinButton
	for _, sp := range spinsIn(s.tools.built[s.tools.open]) {
		r := sp.Bounds()
		if r.Y < mid+half && r.Y+r.H > mid-half {
			on = append(on, sp)
		}
	}
	if which >= len(on) {
		t.Fatalf("row %d of the %s panel holds %d spin buttons, so there is no %s",
			n, s.tools.open, len(on), []string{"first", "second", "third"}[min(which, 2)])
	}
	r := on[which].Bounds()
	return r.X + r.W - 2, r.Y + r.H/4
}

// press puts a press on the panel and redraws, the way a frame follows an
// event in the browser.
func press(s *state, x, y int) {
	s.handleClick(x, y)
	s.draw(buffer())
}

func TestEachTabOpensItsOwnPanelAndHomePutsItAway(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	// Every group is reachable by pressing its tab where the ribbon drew it,
	// and Home puts the panel away again.
	for i, name := range groupNames {
		r, ok := s.ribbon.TabRect(i + 1)
		if !ok {
			t.Fatalf("the ribbon has no tab %d, for %q", i+1, name)
		}
		press(s, r.X+r.W/2, r.Y+r.H/2)
		if s.tools.open != name {
			t.Fatalf("pressing the %q tab opened %q", name, s.tools.open)
		}
		if s.pageW() >= s.viewW() {
			t.Error("the page kept the whole width with a panel beside it")
		}
		home, _ := s.ribbon.TabRect(0)
		press(s, home.X+home.W/2, home.Y+home.H/2)
		if s.tools.open != "" {
			t.Errorf("going back to Home left the %q panel open", name)
		}
	}
}

// TestAMinimisedRibbonStillRunsItsVerbs covers the one piece of the ribbon
// that a container cannot route a press to.
//
// ⛔ A minimised ribbon's body is drawn OVER the page, outside the ribbon's
// own rectangle, so a press in it is a press the page would otherwise take:
// without PopoverClick ahead of everything else, the verb does nothing and the
// page under it turns instead. That is the failure this is here for.
func TestAMinimisedRibbonStillRunsItsVerbs(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	full := s.toolbarH()
	page := s.viewH()

	s.ribbon.Collapsed().Set(true)
	s.refresh()
	s.draw(buffer())
	if s.toolbarH() >= full {
		t.Errorf("a minimised ribbon is %d tall and a whole one %d", s.toolbarH(), full)
	}
	if s.viewH() <= page {
		t.Error("minimising the ribbon did not give the room back to the page")
	}

	// Pressing a tab makes its body peek over the page.
	tab, ok := s.ribbon.TabRect(0)
	if !ok {
		t.Fatal("a minimised ribbon has no tabs left to press")
	}
	press(s, tab.X+tab.W/2, tab.Y+tab.H/2)
	r, shown := s.ribbon.ControlRect(verbs["rotate"].label)
	if !shown {
		t.Fatal("pressing a minimised ribbon's tab did not bring its body back")
	}
	if r.Y+r.H <= s.toolbarH() {
		t.Error("the peeking body is inside the band, so it is not over the page at all")
	}
	press(s, r.X+r.W/2, r.Y+r.H/2)
	if got, _ := s.doc.Rotation(1); got != 90 {
		t.Errorf("the page is turned %d degrees, so the press reached no verb", got)
	}
	if s.at != 1 {
		t.Errorf("the press fell through to the page, which turned to %d", s.at)
	}
}

// TestTheTabsAfterHomeAreTheGroups is what lets tabGroup index groupNames.
func TestTheTabsAfterHomeAreTheGroups(t *testing.T) {
	if ribbonTabs[0].label != tabHome {
		t.Fatalf("the first tab is %q, not the document's own", ribbonTabs[0].label)
	}
	if len(ribbonTabs) != len(groupNames)+1 {
		t.Fatalf("%d tabs for %d groups and Home", len(ribbonTabs), len(groupNames))
	}
	for i, name := range groupNames {
		if ribbonTabs[i+1].label != name {
			t.Errorf("tab %d is %q but group %d is %q", i+1, ribbonTabs[i+1].label, i, name)
		}
		if got := tabGroup(i + 1); got != name {
			t.Errorf("tab %d opens the %q panel, not %q", i+1, got, name)
		}
	}
	// Home, and anything past the end, open nothing.
	for _, i := range []int{-1, 0, len(ribbonTabs)} {
		if got := tabGroup(i); got != "" {
			t.Errorf("tab %d opens the %q panel", i, got)
		}
	}
}

// TestEveryVerbIsOnTheRibbonExactlyOnce is the control over the two tables:
// one says what the verbs are, the other where they go, and nothing keeps them
// in step but this.
func TestEveryVerbIsOnTheRibbonExactlyOnce(t *testing.T) {
	seen := map[string]int{}
	for _, tab := range ribbonTabs {
		for _, g := range tab.groups {
			for _, id := range g.ids {
				if _, ok := verbs[id]; !ok {
					t.Errorf("the %s tab names %q, which is no verb", tab.label, id)
				}
				seen[id]++
			}
		}
	}
	// The quick access bar counts as a place a verb is: its commands are on no
	// tab, which is the point of putting them there.
	for _, id := range quickIDs {
		if _, ok := verbs[id]; !ok {
			t.Errorf("the quick access bar names %q, which is no verb", id)
		}
		seen[id]++
	}
	for id := range verbs {
		switch seen[id] {
		case 1:
		case 0:
			t.Errorf("the verb %q is on no tab, so nothing can run it", id)
		default:
			t.Errorf("the verb %q is on the ribbon %d times", id, seen[id])
		}
	}
}

func TestEveryControlInThePagesPanelIsWiredToItsVerb(t *testing.T) {
	s, _ := opened(t, 6)
	openGroup(t, s, groupPages)

	// The box takes what is typed into it, one character at a time, and an
	// arrow key then belongs to the box rather than to the pages.
	x, y := rowAt(t, s, pagesRows(), 0, 1)
	press(s, x, y)
	if !s.editing() {
		t.Fatal("pressing the box did not put the caret in it")
	}
	for _, c := range []string{"2", "-", "3"} {
		if !s.handleChar(c) {
			t.Fatal("a character was refused by the box that has the caret")
		}
	}
	if s.tools.spec.Get() != "2-3" {
		t.Fatalf("the box holds %q", s.tools.spec.Get())
	}
	at := s.at
	if !s.handleKeyDown("ArrowRight") || s.at != at {
		t.Error("an arrow key turned the page while a box was being typed into")
	}
	if !s.handleKeyDown("Backspace") || s.tools.spec.Get() != "2-" {
		t.Errorf("after a backspace the box holds %q", s.tools.spec.Get())
	}
	s.tools.spec.Set("2-3")

	// Turning: the panel says how far, the ribbon does it. One press of the
	// cycle takes a quarter turn to a half.
	//
	// ⛔ The LEFT of the row, because the row holds two controls now: the cycle
	// and the free angle beside it. Its middle is the seam between them.
	x, y = rowAt(t, s, pagesRows(), 1, 0)
	press(s, x, y)
	if s.tools.turn.Get() != 180 {
		t.Fatalf("the turn is %d degrees", s.tools.turn.Get())
	}
	pressVerb(t, s, "turn")
	if got, _ := s.doc.Rotation(2); got != 180 {
		t.Errorf("page two is turned %d degrees", got)
	}
	if got, _ := s.doc.Rotation(1); got != 0 {
		t.Errorf("page one was turned too, to %d", got)
	}

	// ⛔ AND THE ANGLE BESIDE IT, which is the other verb entirely: a quarter
	// turn is a number written on the page, and any other angle rewrites the
	// page onto a bigger sheet. So this asks the SHEET, not the rotation —
	// `Rotation` would answer 0 for a page turned thirty degrees and look like
	// a verb that did nothing.
	paperOf := func(n int) [2]float64 {
		t.Helper()
		page, err := s.src.Page(n)
		if err != nil {
			t.Fatal(err)
		}
		w, h := pageSize(s.src, page)
		return [2]float64{w, h}
	}
	s.tools.spec.Set("1")
	was := paperOf(1)
	pressVerb(t, s, "turnBy")
	now := paperOf(1)
	if now[0] <= was[0] || now[1] <= was[1] {
		t.Errorf("a page turned by an angle went from %g by %g to %g by %g: the paper did not grow, "+
			"so its corners are off it", was[0], was[1], now[0], now[1])
	}
	if !strings.Contains(s.note, "the paper grew") {
		t.Errorf("the status line says %q, and does not mention the paper", s.note)
	}
	if other := paperOf(3); other != was {
		t.Errorf("page 3 is %v by %v and only page 1 was named", other[0], other[1])
	}
	s.tools.spec.Set("2-3")

	// Reversing, which needs no telling.
	pressVerb(t, s, "reverse")
	if got, _ := s.doc.Rotation(5); got != 180 {
		t.Error("the order was not reversed: the turned page did not move")
	}

	// Keeping only the pages the box names.
	pressVerb(t, s, "keep")
	if s.doc.PageCount() != 2 {
		t.Errorf("keeping 2-3 left %d pages", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "2-3") {
		t.Errorf("the status line says %q", s.note)
	}

	// And dropping them.
	s.tools.spec.Set("1")
	pressVerb(t, s, "drop")
	if s.doc.PageCount() != 1 {
		t.Errorf("deleting page one left %d pages", s.doc.PageCount())
	}
}

func TestMovingCroppingBlankingAndSplittingFromThePanel(t *testing.T) {
	s, h := opened(t, 4)
	openGroup(t, s, groupPages)

	// A number is pressed up in the panel before the verb on the ribbon runs.
	x, y := plusAt(t, s, pagesRows(), 2)
	press(s, x, y)
	if s.tools.moveTo.Get() != 2 {
		t.Fatalf("the spin button says %d", s.tools.moveTo.Get())
	}
	s.at = 1
	pressVerb(t, s, "move")
	if s.at != 2 {
		t.Errorf("the view did not follow the page it moved, and shows %d", s.at)
	}
	if !strings.Contains(s.note, "moved") {
		t.Errorf("the status line says %q", s.note)
	}

	// Cropping: what is typed is a box, and what is drawn afterwards is the
	// page as it would be saved.
	x, y = rowAt(t, s, pagesRows(), 3, 1)
	press(s, x, y)
	for _, c := range strings.Split("0,0,150,200", "") {
		if !s.handleChar(c) {
			t.Fatal("a character was refused by the crop box")
		}
	}
	if s.tools.box.Get() != "0,0,150,200" {
		t.Fatalf("the crop box holds %q", s.tools.box.Get())
	}
	before := s.fitScale(s.src)
	pressVerb(t, s, "crop")
	if s.fitScale(s.src) <= before {
		t.Error("cropping the page did not change how it is drawn")
	}

	// A blank page goes in where the number says.
	was := s.doc.PageCount()
	x, y = plusAt(t, s, pagesRows(), 4)
	press(s, x, y)
	if s.tools.before.Get() != 2 {
		t.Fatalf("the blank page is to go before %d", s.tools.before.Get())
	}
	pressVerb(t, s, "insert")
	if s.doc.PageCount() != was+1 {
		t.Errorf("%d pages after inserting a blank one", s.doc.PageCount())
	}

	// Splitting hands the pieces over in ONE zip and changes nothing. Two pages
	// to a piece, pressed up from one.
	pages := s.doc.PageCount()
	// Row 7: the paper and the ink were added above it. See pagesRows.
	x, y = plusAt(t, s, pagesRows(), 7)
	press(s, x, y)
	if s.tools.every.Get() != 2 {
		t.Fatalf("a piece is to hold %d pages", s.tools.every.Get())
	}
	pressVerb(t, s, "split")
	if s.doc.PageCount() != pages {
		t.Error("splitting changed the document it was asked about")
	}
	want := (pages + 1) / 2
	// ⛔ ONE handover, not one per piece: a browser asked about each of them
	// separately, and a person who refused the fourth prompt had no way of
	// knowing which four they had.
	if h.savedAll != 1 {
		t.Errorf("%d files were handed over for %d pages, two to a piece", h.savedAll, pages)
	}
	if h.as != "sample-pieces.zip" {
		t.Errorf("the pieces came over as %q", h.as)
	}
	z, err := zip.NewReader(bytes.NewReader(h.saved), int64(len(h.saved)))
	if err != nil {
		t.Fatalf("what was handed over is not a zip: %v", err)
	}
	if len(z.File) != want {
		t.Fatalf("the zip holds %d pieces for %d pages, two to a piece", len(z.File), pages)
	}
	if got := z.File[want-1].Name; got != partName("sample.pdf", want) {
		t.Errorf("the last piece is called %q", got)
	}
	// The last piece holds whatever was left over, and reads back as a
	// document of its own.
	last := pages - 2*(want-1)
	f, err := z.File[want-1].Open()
	if err != nil {
		t.Fatal(err)
	}
	piece, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if back, err := reader.Open(piece); err != nil || back.PageCount() != last {
		t.Errorf("the last piece does not read back as %d pages: %v", last, err)
	}
}

// TestSplittingIntoOnePieceHandsOverThePdfItself: a zip holding a single file
// is a wrapper somebody has to open for nothing.
func TestSplittingIntoOnePieceHandsOverThePdfItself(t *testing.T) {
	s, h := opened(t, 3)
	s.tools.every.Set(3)

	s.split()

	if h.savedAll != 1 {
		t.Fatalf("%d files were handed over", h.savedAll)
	}
	if h.as != partName("sample.pdf", 1) {
		t.Errorf("one piece came over as %q", h.as)
	}
	if back, err := reader.Open(h.saved); err != nil || back.PageCount() != 3 {
		t.Errorf("it does not read back as the three pages it was: %v", err)
	}
}

func TestTheSheetPanel(t *testing.T) {
	s, _ := opened(t, 4)
	openGroup(t, s, groupSheet)

	// Two to a sheet is what the number starts at; pressed up it is three,
	// which puts four pages on two sheets.
	x, y := plusAt(t, s, sheetRows(), 0)
	press(s, x, y)
	if s.tools.up.Get() != 3 {
		t.Fatalf("the number says %d to a sheet", s.tools.up.Get())
	}
	s.tools.up.Set(2)
	pressVerb(t, s, "nup")
	if s.doc.PageCount() != 2 || s.at != 1 {
		t.Errorf("%d sheets, showing %d", s.doc.PageCount(), s.at)
	}

	// A booklet reorders the sheets rather than adding any.
	s2, _ := opened(t, 4)
	openGroup(t, s2, groupSheet)
	pressVerb(t, s2, "booklet")
	if s2.doc.PageCount() != 2 {
		t.Errorf("a booklet of four pages came to %d sheets", s2.doc.PageCount())
	}
}

func TestAddingAndOverlayingAnotherFile(t *testing.T) {
	s, h := opened(t, 2)
	openGroup(t, s, groupSheet)
	h.file = samplePDF(t, 3)

	pressVerb(t, s, "merge")
	if s.doc.PageCount() != 5 {
		t.Errorf("adding a three page file to a two page one gave %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "added") {
		t.Errorf("the status line says %q", s.note)
	}

	pressVerb(t, s, "overlay")
	if s.doc.PageCount() != 5 {
		t.Errorf("laying a file over this one changed the page count to %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "laid over") {
		t.Errorf("the status line says %q", s.note)
	}

	// A second file that is not a PDF is said so, and changes nothing.
	h.file = []byte("this is not a PDF")
	pressVerb(t, s, "overlay")
	if !strings.Contains(s.note, "cannot open") {
		t.Errorf("the status line says %q", s.note)
	}
	// And one the person did not choose leaves the document alone.
	h.file = nil
	pressVerb(t, s, "overlay")
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
	x, y := rowAt(t, s, pagesRows(), 0, 1)
	press(s, x, y)
	for _, c := range []string{"1", "-", "2"} {
		s.handleChar(c)
	}
	x, y = rowAt(t, s, pagesRows(), 3, 1)
	press(s, x, y)
	for _, c := range []string{"0", ",", "0", ",", "9", ",", "9"} {
		s.handleChar(c)
	}
	if s.tools.spec.Get() != "1-2" {
		t.Errorf("the range box holds %q", s.tools.spec.Get())
	}
	if s.tools.box.Get() != "0,0,9,9" {
		t.Errorf("the crop box holds %q", s.tools.box.Get())
	}
	// And a press that lands on neither leaves both of them alone.
	press(s, margin+2, s.viewTop()+s.viewH()-2)
	if s.editing() {
		t.Error("a press on nothing left a box with the caret")
	}
}

func TestAPanelPutAwayLetsGoOfTheKeys(t *testing.T) {
	s, _ := opened(t, 3)
	openGroup(t, s, groupPages)
	x, y := rowAt(t, s, pagesRows(), 0, 1)
	press(s, x, y)
	if !s.editing() {
		t.Fatal("the caret is not in the box")
	}
	// Putting the panel away, or opening the form panel instead, takes the
	// caret out of a box nobody can see any more.
	s.ribbon.ActiveTab().Set(0)
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
	openGroup(t, s, groupSheet)
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
		empty.tools.box.Set("0,0,10,10")
		act()
		if empty.note == "" {
			t.Error("a verb with no document said nothing")
		}
	}

	s, _ := opened(t, 3)
	// A range that names every page is refused: a document needs one.
	s.tools.spec.Set("1-3,3")
	s.deleteRange()
	if s.doc.PageCount() != 3 {
		t.Errorf("every page was deleted, leaving %d", s.doc.PageCount())
	}
	if !strings.Contains(s.note, "needs one") {
		t.Errorf("the status line says %q", s.note)
	}
	// A range that is not a range at all is the operation's to complain about.
	s.tools.spec.Set("nonsense")
	s.deleteRange()
	if s.note == "" || s.doc.PageCount() != 3 {
		t.Errorf("a nonsense range said %q and left %d pages", s.note, s.doc.PageCount())
	}
	// Moving a page nowhere it can go.
	s.tools.moveTo.Set(99)
	s.at = 1
	s.movePage()
	if s.at != 1 {
		t.Errorf("the view followed a move that did not happen, to %d", s.at)
	}
	// Laying out no pages to a sheet, and folding a document that cannot be.
	s.tools.up.Set(0)
	s.nUp()
	if s.doc.PageCount() != 3 {
		t.Errorf("a nonsense n-up left %d pages", s.doc.PageCount())
	}
	s.tools.every.Set(0)
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
	s.tools.box.Set("not a box")
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
	s.tools.every.Set(1)
	s.split()
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("the status line says %q", s.note)
	}
	if h.savedAll != 0 {
		t.Errorf("%d pieces were handed over anyway", h.savedAll)
	}
}
