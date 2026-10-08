package main

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// TestTheControlFollowsTheModelAndNotOnlyTheOtherWayRound is the control over
// the whole MVVM conversion.
//
// ⛔ It reads the SPIN BUTTON, not the field behind it. Asserting that
// s.tools.before holds 2 would pass with the old one-way wiring just as well,
// because the old wiring could set the field perfectly happily -- what it
// could not do is get the number back onto the screen. The panel is built once
// and kept, so the only thing that ever showed a number was the widget, and the
// widget never heard about a change it had not made itself.
//
// Someone who sets "Put a blank page before 7" and then drops pages 3 to 9 is
// left looking at a 7 that refers to nothing.
func TestTheControlFollowsTheModelAndNotOnlyTheOtherWayRound(t *testing.T) {
	s, _ := opened(t, 9)
	openGroup(t, s, "Pages")

	s.tools.before.Set(7)
	s.tools.moveTo.Set(8)
	s.draw(buffer())

	spins := spinsIn(s.tools.built["Pages"])
	if len(spins) < 3 {
		t.Fatalf("the Pages panel has %d spin buttons, which is not the panel this is about", len(spins))
	}
	shown := func() []int {
		var out []int
		for _, sp := range spins {
			out = append(out, sp.Value().Get())
		}
		return out
	}
	before := shown()
	if before[0] != 8 || before[1] != 7 {
		t.Fatalf("the panel shows %v, and it was set to move to 8 and insert before 7", before)
	}

	// The document shrinks under both numbers.
	s.tools.spec.Set("3-9")
	s.deleteRange()
	if n := s.doc.PageCount(); n != 2 {
		t.Fatalf("dropping 3-9 of nine pages left %d", n)
	}

	after := shown()
	for i, got := range after[:2] {
		if got > 2 {
			t.Errorf("spin button %d still offers page %d of a document with 2 pages: the control did not "+
				"hear the model change, which is the direction a one-way Subscribe does not have", i, got)
		}
	}
}

// TestABoxAndItsDatumAreTheSameValueSeenTwice asserts both directions for a
// text box.
//
// ⛔ The first draft of this asserted only that typing reaches the model, and
// a mutation that rewired the box the OLD way -- Subscribe, control to model,
// one way -- survived it, because that direction is the one the old wiring
// already had. A test of a two-way binding that only tests the easy way is a
// test of nothing that changed.
func TestABoxAndItsDatumAreTheSameValueSeenTwice(t *testing.T) {
	s, _ := opened(t, 3)
	openGroup(t, s, "Marks")

	var box *toolkit.Entry
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if e, ok := w.(*toolkit.Entry); ok && box == nil {
			box = e
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(s.tools.built["Marks"])
	if box == nil {
		t.Fatal("the Marks panel has no box to type in")
	}
	box.Text().Set("1-2")
	if got := s.tools.markSpec.Get(); got != "1-2" {
		t.Errorf("what was typed reached the model as %q", got)
	}
	// And back: a datum set by anything other than the box is shown by it.
	s.tools.markSpec.Set("4-5")
	if got := box.Text().Get(); got != "4-5" {
		t.Errorf("the model was set to 4-5 and the box shows %q, so the panel would go on "+
			"offering a range the document no longer has", got)
	}
}

// ticksIn collects the tick boxes laid out under a widget.
func ticksIn(w toolkit.Widget) []*toolkit.CheckButton {
	var out []*toolkit.CheckButton
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if c, ok := w.(*toolkit.CheckButton); ok {
			out = append(out, c)
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

func TestATickAndAListAlsoFollowTheirDatum(t *testing.T) {
	// ⛔ One test per KIND of control, and each of them in the direction the
	// old wiring did not have. Rewiring the tick or the list the old way --
	// Subscribe, control to model -- survived the suite until this existed,
	// because every other test drives them by pressing, and pressing is the
	// direction that always worked.
	s, _ := opened(t, 3)

	openGroup(t, s, "Protect")
	ticks := ticksIn(s.tools.built["Protect"])
	if len(ticks) == 0 {
		t.Fatal("the Protect panel has no permission to tick")
	}
	name := allowed[0].name
	s.tools.allow[name].Set(false)
	s.draw(buffer())
	if ticks[0].Checked().Get() {
		t.Errorf("%q was turned off in the model and its box is still ticked", name)
	}
	s.tools.allow[name].Set(true)
	if !ticks[0].Checked().Get() {
		t.Errorf("%q was turned back on and its box is not ticked", name)
	}

	openGroup(t, s, "Marks")
	lists := dropsIn(s.tools.built["Marks"])
	if len(lists) == 0 {
		t.Fatal("the Marks panel has no list to choose from")
	}
	want := len(placeNames()) - 1
	s.tools.at.Set(want)
	s.draw(buffer())
	if got := lists[0].Selected().Get(); got != want {
		t.Errorf("the model chose place %d and the list is on %d", want, got)
	}
}

// named finds a control on the strip by what it says.
func named(s *state, label string) *toolkit.Button {
	for _, k := range s.toolbar.Children() {
		if b, ok := k.(*toolkit.Button); ok && b.Label().Get() == label {
			return b
		}
	}
	return nil
}

func TestAVerbSaysBeforehandWhetherItCanRun(t *testing.T) {
	// ⛔ Read off Disabled, which is what a person sees. Every one of these
	// rules already exists inside its handler, where it answers AFTERWARDS
	// with a sentence in the status line: press Delete on a document of one
	// page and it tells you it will not. The command says it first, and that
	// is the whole of what this buys.
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.refresh()
	for _, label := range []string{"Save", "<", ">", "Rotate", "Delete", "Fill in"} {
		b := named(s, label)
		if b == nil {
			t.Fatalf("there is no %q on the strip", label)
		}
		if !b.Disabled().Get() {
			t.Errorf("%q is pressable with no document open", label)
		}
	}
	// Getting a document is not something a document is needed for.
	for _, label := range []string{"Open", "Pages", "Marks"} {
		if b := named(s, label); b != nil && b.Disabled().Get() {
			t.Errorf("%q cannot be pressed, though it needs no document", label)
		}
	}

	// A document arrives, and the verbs follow without anybody telling them.
	s2, _ := opened(t, 3)
	for _, label := range []string{"Save", "Rotate", "Delete", ">"} {
		if named(s2, label).Disabled().Get() {
			t.Errorf("%q is still greyed with a three page document open", label)
		}
	}
	if !named(s2, "<").Disabled().Get() {
		t.Error("the back arrow is pressable on the first page")
	}
	s2.goTo(3)
	if !named(s2, ">").Disabled().Get() {
		t.Error("the forward arrow is pressable on the last page")
	}
	if named(s2, "<").Disabled().Get() {
		t.Error("the back arrow is greyed on the last page")
	}
}

func TestTheLastPageCannotBeDroppedAndTheControlSaysSo(t *testing.T) {
	s, _ := opened(t, 2)
	if named(s, "Delete").Disabled().Get() {
		t.Fatal("Delete is greyed on a document of two pages")
	}
	s.deletePage()
	if n := s.doc.PageCount(); n != 1 {
		t.Fatalf("dropping one page of two left %d", n)
	}
	if !named(s, "Delete").Disabled().Get() {
		t.Error("Delete is still pressable on the one page that is left, " +
			"so the only thing stopping it is the sentence it answers with")
	}
}

func TestTheArrowsTurnThePagesWhenTheyArePressable(t *testing.T) {
	// Pressed on the strip, not called: a command sits between the control and
	// the handler now, and pressing is the only way to find out that it lets
	// the press through.
	s, _ := opened(t, 3)
	s.draw(buffer())
	press := func(label string) {
		b := named(s, label)
		if b == nil {
			t.Fatalf("there is no %q on the strip", label)
		}
		r := b.Bounds()
		s.handleClick(r.X+r.W/2, r.Y+r.H/2)
		s.draw(buffer())
	}

	press(">")
	if s.at != 2 {
		t.Fatalf("pressing the forward arrow on page 1 of three left page %d", s.at)
	}
	press("<")
	if s.at != 1 {
		t.Errorf("pressing the back arrow on page 2 left page %d", s.at)
	}
	// And at the first page it is greyed, so pressing it does nothing at all.
	press("<")
	if s.at != 1 {
		t.Errorf("a greyed back arrow still turned the page, to %d", s.at)
	}
}
