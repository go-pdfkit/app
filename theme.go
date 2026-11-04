// The reader's choice of light or dark, and the system's.
//
// ⛔ THREE STATES AND NOT TWO. A switch that only says light-or-dark has no way
// to say "whatever this machine is set to", so the first press takes the
// workbench out of the system's hands for good — including when the system
// flips at sunset and the workbench does not. The third state is the DEFAULT,
// which is what makes a reader who never touches the button still get the right
// one.

package main

import "github.com/go-widgets/toolkit"

// A themeMode is which of the three the reader has chosen.
type themeMode int

const (
	// themeSystem follows the colour scheme the page is being shown in, and
	// keeps following it: the system can change while the tab is open.
	themeSystem themeMode = iota
	themeLight
	themeDark
)

// next is the order the button cycles in: system, light, dark, back to system.
//
// System comes FIRST rather than between the two, so the way back to "stop
// deciding for me" is one more press of the same button and not a hunt through
// a menu. It is also the state the workbench starts in, which means the cycle
// reads as a loop from where it began.
func (m themeMode) next() themeMode {
	if m == themeDark {
		return themeSystem
	}
	return m + 1
}

// name is how a mode is written down, which is what the host remembers between
// visits.
func (m themeMode) name() string {
	switch m {
	case themeLight:
		return "light"
	case themeDark:
		return "dark"
	default:
		return "system"
	}
}

// parseThemeMode reads back what name wrote. ⛔ Anything else is "system",
// not an error and not a crash: the value comes from browser storage, which a
// reader can edit, another tab can overwrite with a newer app's spelling, and
// a browser may hand back empty. The worst an unreadable one may do is leave
// the workbench where it would have been without it.
func parseThemeMode(s string) themeMode {
	switch s {
	case "light":
		return themeLight
	case "dark":
		return themeDark
	default:
		return themeSystem
	}
}

// dark is whether to paint dark, given what the system reports. Only
// themeSystem reads the argument, which is the whole difference between the
// three.
func (m themeMode) dark(systemDark bool) bool {
	switch m {
	case themeLight:
		return false
	case themeDark:
		return true
	default:
		return systemDark
	}
}

// icon is the mark the button carries, which is the only thing that says which
// mode is on: the control keeps one label so a test and a reader can both find
// it by name, and a button whose state is invisible is a button nobody can
// tell they have already pressed.
func (m themeMode) icon() string {
	switch m {
	case themeLight:
		return "sun-light"
	case themeDark:
		return "half-moon"
	default:
		return "brightness"
	}
}

// tip is the sentence under the name, and it says what the NEXT press will do
// as well as what is on now — a cycle of three is the one control where
// "what does this do" cannot be answered by its name alone.
func (m themeMode) tip() string {
	switch m {
	case themeLight:
		return "Light. Press for dark."
	case themeDark:
		return "Dark. Press to follow the system again."
	default:
		return "Following the system. Press for light."
	}
}

// themeFor is the palette a mode resolves to.
func themeFor(m themeMode, systemDark bool) *toolkit.Theme {
	if m.dark(systemDark) {
		return toolkit.DefaultDark()
	}
	return toolkit.DefaultLight()
}

// cycleTheme is the button: the next mode, painted, and remembered.
func (s *state) cycleTheme() {
	s.setThemeMode(s.themeMode.next())
	s.host.StoreThemeMode(s.themeMode.name())
}

// setThemeMode puts the workbench in a mode without remembering it, which is
// what restoring a remembered one needs: writing it back on the way in would be
// a write for every visit and would overwrite a newer choice made in another
// tab between the read and the write.
func (s *state) setThemeMode(m themeMode) {
	s.themeMode = m
	s.applyTheme()
}

// setSystemDark takes the page's current colour scheme. It is called once on
// the way in and again whenever the system changes, so a workbench left open
// across sunset follows.
//
// ⛔ It repaints even when the mode is not themeSystem. The mode can be changed
// back at any time, and the workbench would then be painted from a value last
// read before the reader chose light.
func (s *state) setSystemDark(dark bool) {
	s.systemDark = dark
	s.applyTheme()
}

// applyTheme resolves the palette and asks for a repaint. The page picture
// itself is NOT recoloured: a PDF page is paper, and paper does not turn black
// because the window did.
func (s *state) applyTheme() {
	s.theme = themeFor(s.themeMode, s.systemDark)
	if s.themeCtl != nil {
		s.themeCtl.Tip = s.themeMode.tip()
	}
	s.repaint()
}
