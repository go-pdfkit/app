package main

import (
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// aTile is the surface point in the middle of the nth minipage.
func aTile(t *testing.T, s *state, n int) (int, int) {
	t.Helper()
	got := railTiles(s.railView)
	if len(got) < n {
		t.Fatalf("the rail has %d minipages, wanted at least %d", len(got), n)
	}
	b := got[n-1].Bounds()
	return b.X + b.W/2, b.Y + b.H/2
}

// menuLabels is what the open menu offers, in order.
func menuLabels(s *state) []string {
	if s.menu == nil {
		return nil
	}
	var out []string
	for _, it := range s.menu.Menu.Items {
		if it.Separator {
			continue
		}
		out = append(out, it.Label)
	}
	return out
}

func TestASecondaryPressOnAMinipageOpensAMenuForThatPage(t *testing.T) {
	s, _ := opened(t, 4)
	s.draw(buffer())
	x, y := aTile(t, s, 3)

	if !s.handleContext(x, y) {
		t.Fatal("a secondary press on a minipage was not answered")
	}
	if s.menu == nil {
		t.Fatal("no menu opened over a minipage")
	}
	// ⛔ It names the page, and it is the page that was pointed at — not the
	// one the reader happened to be on. A menu that acted on the wrong page
	// would look identical until it was used.
	got := menuLabels(s)
	if len(got) == 0 || got[0] != "Page 3" {
		t.Fatalf("the menu offers %v; its first row should name page 3", got)
	}
	if s.at != 3 {
		t.Errorf("the menu says page 3 and the workbench is on %d: the verbs would act "+
			"on the wrong page", s.at)
	}
}

func TestASecondaryPressOnThePageOpensAMenuForIt(t *testing.T) {
	s, _ := opened(t, 2)
	s.draw(buffer())
	b := s.pv.Bounds()
	if !s.handleContext(b.X+b.W/2, b.Y+b.H/2) {
		t.Fatal("a secondary press on the page was not answered")
	}
	if s.menu == nil {
		t.Fatal("no menu opened over the page")
	}
	if got := menuLabels(s); len(got) == 0 || !strings.HasPrefix(got[0], "Page ") {
		t.Fatalf("the menu offers %v; its first row should name a page", got)
	}
}

func TestTheMenuOffersTheRibbonsOwnVerbs(t *testing.T) {
	// ⛔ By id out of the verbs table, never restated. A menu with its own copy
	// of a label keeps the old one after a rename — which is exactly what the
	// strip's icon map did, two hundred lines from the button it belonged to.
	s, _ := opened(t, 3)
	s.draw(buffer())
	x, y := aTile(t, s, 2)
	s.handleContext(x, y)

	got := menuLabels(s)
	for _, id := range pageVerbs {
		want := verbs[id].label
		found := false
		for _, label := range got {
			if label == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the verb %q is called %q on the ribbon and the menu offers %v",
				id, want, got)
		}
	}
}

func TestAVerbThatMayNotRunKeepsItsRowAndLosesItsAction(t *testing.T) {
	// ⛔ Keeps its ROW. A row that vanished would move every row under it
	// between one opening of the menu and the next, so the press a reader
	// aimed by muscle memory would land on a different verb.
	s, _ := opened(t, 1) // one sheet: "delete" may not run
	s.draw(buffer())
	b := s.pv.Bounds()
	s.handleContext(b.X+b.W/2, b.Y+b.H/2)
	if s.menu == nil {
		t.Fatal("no menu")
	}
	var del *toolkit.MenuItem
	for i := range s.menu.Menu.Items {
		if s.menu.Menu.Items[i].Label == verbs["delete"].label {
			del = &s.menu.Menu.Items[i]
		}
	}
	if del == nil {
		t.Fatalf("the only verb that cannot run on a single sheet lost its row: %v", menuLabels(s))
	}
	if del.Action != nil {
		t.Error("deleting the only page is offered as something that can be done")
	}
}

func TestChoosingFromTheMenuActsAndClosesIt(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	x, y := aTile(t, s, 2)
	s.handleContext(x, y)

	var rotate func()
	for _, it := range s.menu.Menu.Items {
		if it.Label == verbs["rotate"].label {
			rotate = it.Action
		}
	}
	if rotate == nil {
		t.Fatal("the menu does not offer a turn that can be run")
	}
	before := s.pvDoc
	rotate()
	if s.menu != nil {
		t.Error("the menu stayed up after a row was chosen")
	}
	if s.pvDoc == before {
		t.Error("choosing the row changed no document")
	}
}

func TestAPressAnywhereClosesTheMenu(t *testing.T) {
	s, _ := opened(t, 4)
	s.draw(buffer())
	x, y := aTile(t, s, 3)

	s.handleContext(x, y)
	if s.menu == nil {
		t.Fatal("no menu")
	}
	mb := s.menu.MenuBounds()
	s.handleClick(mb.X+mb.W/2, mb.Y+2)
	if s.menu != nil {
		t.Error("a press inside the menu left it up")
	}

	s.handleContext(x, y)
	if s.menu == nil {
		t.Fatal("the menu did not reopen")
	}
	s.handleClick(2, s.h-2) // the status line, far from everything
	if s.menu != nil {
		t.Error("a press outside the menu left it up")
	}
}

func TestAPressInsideTheMenuDoesNotReachTheMinipageUnderIt(t *testing.T) {
	// ⛔ This test exists because the one above does NOT check it. The menu is
	// drawn over the rail, so a press in it is a press a minipage would
	// otherwise take — the row would do nothing and the page under it would
	// turn. Closing the menu happens either way, so asserting on the menu
	// being gone proved nothing: a mutant that let the press fall straight
	// through kept the whole file green.
	//
	// The press therefore lands LOW in the menu, where the rail has a
	// different tile underneath, and the thing asserted is the page the
	// workbench is on.
	s, _ := opened(t, 6)
	s.draw(buffer())
	x, y := aTile(t, s, 1)
	s.handleContext(x, y)
	if s.menu == nil {
		t.Fatal("no menu")
	}
	if s.at != 1 {
		t.Fatalf("precondition: the menu was opened over page 1 and the workbench is on %d", s.at)
	}
	// ⛔ The point is FOUND, not guessed. The first guess — the middle of the
	// menu's bottom row — was to the right of the rail entirely, because the
	// menu is wider than the rail is, and the test skipped itself. A skip is a
	// silent filter: the test would have sat there green and checking nothing.
	mb := s.menu.MenuBounds()
	px, py, under := 0, 0, 0
	for dy := 1; dy < mb.H-1 && under == 0; dy++ {
		for _, dx := range []int{2, mb.W / 4, mb.W / 2} {
			if n, ok := s.tileUnder(mb.X+dx, mb.Y+dy); ok && n != 1 {
				px, py, under = mb.X+dx, mb.Y+dy, n
				break
			}
		}
	}
	if under == 0 {
		t.Fatalf("no point inside the menu (%+v) sits over another minipage, so this test "+
			"cannot check what it says it checks", mb)
	}

	s.handleClick(px, py)
	if s.at != 1 {
		t.Errorf("a press inside the menu reached minipage %d underneath it and turned to "+
			"page %d", under, s.at)
	}
}

func TestTheMenuIsActuallyPaintedWhereItWasOpened(t *testing.T) {
	// ⛔ The test that would have caught the defect this feature shipped with
	// for twenty minutes. ContextMenu.MenuBounds clamps the menu inside its
	// OWN bounds, so a menu never given any is clamped into a surface of
	// nothing: placed at the origin, zero pixels tall, drawn nowhere. It
	// opened, s.menu was not nil, every other test here passed — and there was
	// nothing on the screen.
	//
	// So this one asks the PIXELS.
	s, _ := opened(t, 4)
	s.draw(buffer())
	x, y := aTile(t, s, 2)
	s.handleContext(x, y)

	mb := s.menu.MenuBounds()
	if mb.W <= 0 || mb.H <= 0 {
		t.Fatalf("the menu occupies %+v, which is not a rectangle anybody can see", mb)
	}
	if mb.X != x || mb.Y != y {
		// Not an error: the menu shifts to stay inside the window. But it must
		// not have been pushed to the origin, which is what no bounds looks
		// like.
		if mb.X == 0 && mb.Y == 0 {
			t.Errorf("the menu was opened at %d,%d and sits at the origin: it has been "+
				"clamped into a surface of nothing", x, y)
		}
	}

	before := buffer()
	s.menu = nil
	s.draw(before)
	after := buffer()
	s.handleContext(x, y)
	s.draw(after)
	if inkIn(after, s.w, mb.X, mb.Y, mb.W, mb.H, s.theme.Background) <=
		inkIn(before, s.w, mb.X, mb.Y, mb.W, mb.H, s.theme.Background) {
		t.Error("opening the menu put no more ink where the menu is than there was before it")
	}
}

func TestOpeningTheMenuOnThePageAlreadyBeingReadTurnsNothing(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	x, y := aTile(t, s, 1) // already on page 1
	s.handleContext(x, y)
	if s.at != 1 {
		t.Errorf("opening the menu over the page already being read moved to %d", s.at)
	}
	if got := menuLabels(s); len(got) == 0 || got[0] != "Page 1" {
		t.Errorf("the menu offers %v", got)
	}
}

func TestEveryPageVerbIsOneTheRibbonHas(t *testing.T) {
	// ⛔ pageVerbs names ribbon verbs by id. An id that no longer exists would
	// otherwise be found out by a reader, as a menu one row short — or, now
	// that the silent skip is gone, by a panic. This says it at build time
	// instead, the way TestEveryMarkOnTheRibbonIsOneThePackHas does for icons.
	for _, id := range pageVerbs {
		v, held := verbs[id]
		if !held {
			t.Errorf("the page menu offers %q, which is not a verb the ribbon has", id)
			continue
		}
		if v.run == nil {
			t.Errorf("the verb %q has no body, so choosing it from the menu would panic", id)
		}
	}
}

func TestWalkingForTilesIsSafeOnAnythingElse(t *testing.T) {
	// The rail is rebuilt on every repaint, so the walk meets whatever the
	// band happens to hold — including leaves that are not minipages.
	if got := railTiles(toolkit.NewLabel("not a minipage")); got != nil {
		t.Errorf("a label was read as %d minipages", len(got))
	}
}

func TestASecondaryPressOnNothingOpensNoMenuAndIsStillAnswered(t *testing.T) {
	// ⛔ Answered even so. Reporting false lets the host fall back on the
	// browser's own menu, over a canvas we have already drawn our own on.
	s, _ := opened(t, 2)
	s.draw(buffer())
	if !s.handleContext(2, 2) { // the ribbon
		t.Error("a secondary press on the command band was not answered")
	}
	if s.menu != nil {
		t.Errorf("a page menu opened over the ribbon: %v", menuLabels(s))
	}

	empty := newState(surfaceW, surfaceH, &fakeHost{})
	if !empty.handleContext(surfaceW/2, surfaceH/2) {
		t.Error("a secondary press with no document was not answered")
	}
	if empty.menu != nil {
		t.Error("a page menu opened with no document open")
	}
}
