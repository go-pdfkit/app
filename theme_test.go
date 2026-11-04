package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// paintsDark says whether a workbench is currently painting in the dark
// palette. ⛔ It compares against the toolkit's own two themes rather than
// asking the workbench what mode it is in: a mode field that says "dark" while
// the surface is painted light is exactly the defect these tests exist to
// catch, and a test that reads the field back would agree with it.
func paintsDark(s *state) bool {
	return s.theme.Background == toolkit.DefaultDark().Background
}

func TestTheThemeFollowsTheSystemUntilSomebodyChoosesOtherwise(t *testing.T) {
	h := &fakeHost{sysDark: true}
	s := newState(surfaceW, surfaceH, h)
	if !paintsDark(s) {
		t.Fatal("a workbench opened on a dark system painted light")
	}
	// ⛔ The case a value read once at startup cannot serve: the system changes
	// under a workbench that is already open.
	h.flipSystem(false)
	if paintsDark(s) {
		t.Error("the system went light and the workbench stayed dark")
	}
	h.flipSystem(true)
	if !paintsDark(s) {
		t.Error("the system went dark again and the workbench stayed light")
	}
}

func TestChoosingLightOrDarkTakesItOutOfTheSystemsHands(t *testing.T) {
	h := &fakeHost{sysDark: true}
	s := newState(surfaceW, surfaceH, h)

	s.cycleTheme() // system -> light
	if s.themeMode != themeLight || paintsDark(s) {
		t.Fatalf("after one press: mode %q, dark %v — want light", s.themeMode.name(), paintsDark(s))
	}
	// And the system no longer decides. This is the half a cycle test that only
	// checks the mode would miss: the mode can be right while the palette is
	// still being re-derived from the system behind it.
	h.flipSystem(true)
	if paintsDark(s) {
		t.Error("the system went dark and overrode a reader who had chosen light")
	}

	s.cycleTheme() // light -> dark
	if s.themeMode != themeDark || !paintsDark(s) {
		t.Fatalf("after two presses: mode %q, dark %v — want dark", s.themeMode.name(), paintsDark(s))
	}
	h.flipSystem(false)
	if !paintsDark(s) {
		t.Error("the system went light and overrode a reader who had chosen dark")
	}

	s.cycleTheme() // dark -> system, which is now light
	if s.themeMode != themeSystem {
		t.Fatalf("after three presses: mode %q, want system", s.themeMode.name())
	}
	if paintsDark(s) {
		t.Error("back on system over a light system, and still dark")
	}
}

func TestTheChoiceIsRememberedAndRestored(t *testing.T) {
	h := &fakeHost{sysDark: false}
	s := newState(surfaceW, surfaceH, h)
	s.cycleTheme() // -> light
	s.cycleTheme() // -> dark
	want := []string{"light", "dark"}
	if len(h.storedAs) != len(want) || h.storedAs[0] != want[0] || h.storedAs[1] != want[1] {
		t.Fatalf("stored %v, want %v", h.storedAs, want)
	}

	// A later visit, same browser: the remembered mode is in force before the
	// first frame, not after it.
	//
	// ⛔ Each is restored AGAINST the opposite system, which is the only way the
	// restore can be told from the system deciding. A remembered "dark" on a
	// dark system passes whether it was read or ignored.
	back := newState(surfaceW, surfaceH, &fakeHost{sysDark: false, mode: "dark"})
	if back.themeMode != themeDark || !paintsDark(back) {
		t.Errorf("a remembered dark came back as %q (dark %v) on a LIGHT system",
			back.themeMode.name(), paintsDark(back))
	}
	lit := newState(surfaceW, surfaceH, &fakeHost{sysDark: true, mode: "light"})
	if lit.themeMode != themeLight || paintsDark(lit) {
		t.Errorf("a remembered light came back as %q (dark %v) on a DARK system",
			lit.themeMode.name(), paintsDark(lit))
	}

	// ⛔ And restoring must NOT write back. A write on every visit would
	// overwrite a choice made in another tab between this tab's read and its
	// write — two tabs open is the ordinary case, not an exotic one.
	fresh := &fakeHost{mode: "dark"}
	newState(surfaceW, surfaceH, fresh)
	if len(fresh.storedAs) != 0 {
		t.Errorf("opening a workbench wrote %v to storage without anybody pressing anything",
			fresh.storedAs)
	}
}

func TestAnUnreadableRememberedModeIsNotATrap(t *testing.T) {
	// The value comes from browser storage, which a reader can edit by hand and
	// another tab can overwrite with a newer app's spelling. The worst it may do
	// is leave the workbench where it would have been without it.
	for _, bad := range []string{"", "DARK", "sombre", "true", "system "} {
		s := newState(surfaceW, surfaceH, &fakeHost{sysDark: true, mode: bad})
		if s.themeMode != themeSystem {
			t.Errorf("a remembered %q became mode %q, want system", bad, s.themeMode.name())
		}
		if !paintsDark(s) {
			t.Errorf("a remembered %q stopped the workbench following a dark system", bad)
		}
	}
}

func TestTheSwitchSaysWhichModeIsOnAndWhatTheNextPressDoes(t *testing.T) {
	// ⛔ The label is deliberately fixed — Ribbon.ControlRect finds a control by
	// it, so a label that moved with the state would move the control out from
	// under the press that moved it. That makes the MARK and the TOOLTIP the
	// only things carrying the mode, so both are asserted here: if they ever
	// stop distinguishing the three, the switch becomes a button a reader
	// cannot tell they have already pressed.
	s := newState(surfaceW, surfaceH, &fakeHost{})
	have := map[string]bool{}
	for _, n := range toolkit.IconoirNames() {
		have[n] = true
	}
	seen := map[string]bool{}
	tips := map[string]bool{}
	for range 3 {
		icon, tip := s.themeMode.icon(), s.themeCtl.Tip
		if seen[icon] {
			t.Errorf("mode %q reuses the mark %q", s.themeMode.name(), icon)
		}
		if tips[tip] {
			t.Errorf("mode %q reuses the tooltip %q", s.themeMode.name(), tip)
		}
		seen[icon], tips[tip] = true, true
		if !have[icon] {
			t.Errorf("mode %q carries the mark %q, which the pack does not have: it would "+
				"draw NOTHING, silently", s.themeMode.name(), icon)
		}
		s.cycleTheme()
	}
	if s.themeMode != themeSystem {
		t.Errorf("three presses did not come back to system, got %q", s.themeMode.name())
	}
}

func TestTheSwitchWorksWithNoDocumentOpen(t *testing.T) {
	// ⛔ Everything else on this ribbon greys when no document is open, because
	// everything else acts on a document. A colour scheme does not, and the
	// moment a reader most wants dark is the empty workbench they just opened
	// at night.
	s := newState(surfaceW, surfaceH, &fakeHost{})
	if s.opened() {
		t.Fatal("precondition: this workbench should have no document")
	}
	if !s.acts["theme"].IsEnabled() {
		t.Fatal("the theme switch is greyed with no document open")
	}
	s.cycleTheme()
	if s.themeMode != themeLight {
		t.Errorf("pressing it with no document open did nothing: mode %q", s.themeMode.name())
	}
	s.reconsider() // what runs after every change, and greys the verbs
	if !s.acts["theme"].IsEnabled() {
		t.Error("reconsider greyed the theme switch along with the verbs")
	}
}
