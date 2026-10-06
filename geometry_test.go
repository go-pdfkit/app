package main

import (
	"testing"

	"github.com/go-opentype/fonts/inter"
	"github.com/go-widgets/toolkit"
)

// bottomOf is the lowest edge of anything laid out under w, itself included.
func bottomOf(w toolkit.Widget) int {
	low := 0
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if w == nil {
			return
		}
		if r := w.Bounds(); r.H > 0 && r.Y+r.H > low {
			low = r.Y + r.H
		}
		if c, ok := w.(kids); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(w)
	return low
}

// TestNoPanelOverflowsTheRoomItIsGiven is the control over the band and row
// geometry, which follows the installed typeface.
//
// ⛔ It is what the suite did not have when the workbench was changed from the
// toolkit's 5x7 bitmap to a real face. The row heights were constants chosen
// for a 7-pixel glyph; a 16-pixel one made every row taller, five panels grew
// past the bottom of their box, and the Split button went under the edge. The
// panel is a ScrollView, so nothing crashed and no test said anything about
// the geometry: eleven tests failed instead on values they could no longer
// reach, which reads as eleven broken verbs rather than one wrong constant.
//
// A panel whose last control has to be scrolled to is a control people do not
// find, so this asserts the thing worth having -- it all fits -- rather than a
// number, and it is therefore true for any face that fits and false for any
// that does not.
func TestNoPanelOverflowsTheRoomItIsGiven(t *testing.T) {
	s, _ := opened(t, 5)
	for _, name := range groupNames {
		openGroup(t, s, name)
		box := s.tools.built[name]
		room := box.Bounds()
		if low := bottomOf(box); low > room.Y+room.H {
			t.Errorf("the %s panel is laid out down to %d but its box ends at %d, "+
				"so its last %d pixels can only be scrolled to (text is %d pixels tall)",
				name, low, room.Y+room.H, low-(room.Y+room.H), textH())
		}
	}
}

// TestTheBandsFollowTheFace says the three bands are sized from the text they
// hold, which is what lets the workbench change typeface at all.
func TestTheBandsFollowTheFace(t *testing.T) {
	was := toolkit.CurrentFont()
	t.Cleanup(func() { toolkit.SetFont(was) })

	small := map[string]int{"toolbar": toolbarH(), "status": statusH(), "bare row": bareH(), "labelled row": labelledH()}

	big, err := toolkit.NewTrueTypeFont(inter.TTF, uiFontPx*2)
	if err != nil {
		t.Fatalf("a face at twice the size: %v", err)
	}
	toolkit.SetFont(big)
	grew := map[string]int{"toolbar": toolbarH(), "status": statusH(), "bare row": bareH(), "labelled row": labelledH()}

	for band, before := range small {
		after := grew[band]
		if after <= before {
			t.Errorf("the %s band is %d pixels at %d-pixel text and %d at twice that: it does not follow the face",
				band, before, was.Height(), after)
		}
	}
	// A labelled row holds two lines, so it must grow by more than a bare one.
	if grew["labelled row"]-small["labelled row"] <= grew["bare row"]-small["bare row"] {
		t.Error("a labelled row did not grow by more than a bare one, though it holds a caption as well as a control")
	}
}
