package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// spots reads the published answer back the way a harness outside this process
// does: as text, parsed.
func spots(t *testing.T, s *state) map[string]spot {
	t.Helper()
	var got []spot
	if err := json.Unmarshal([]byte(s.controlsJSON()), &got); err != nil {
		t.Fatalf("what was published does not parse: %v", err)
	}
	out := map[string]spot{}
	for _, sp := range got {
		out[sp.Label] = sp
	}
	return out
}

func TestWhereTheControlsAreIsWhereTheyAreDrawn(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	got := spots(t, s)

	// Every tab, and every verb of the tab now showing.
	for _, tab := range ribbonTabs {
		if _, ok := got[tab.label]; !ok {
			t.Errorf("the %s tab is not published, so nothing outside can choose it", tab.label)
		}
	}
	for _, g := range ribbonTabs[0].groups {
		for _, id := range g.ids {
			sp, ok := got[id]
			if !ok {
				t.Fatalf("%q is on the tab being shown and is not published", id)
			}
			// ⛔ Pressed where it says, and the verb runs. A published
			// coordinate that is not where the control is would be worse than
			// none: a harness pressing it would report the verb as broken.
			r, drawn := s.ribbon.ControlRect(verbs[id].label)
			if !drawn || r.X != sp.X || r.Y != sp.Y || r.W != sp.W || r.H != sp.H {
				t.Errorf("%q is published at %d,%d %dx%d and drawn at %d,%d %dx%d",
					id, sp.X, sp.Y, sp.W, sp.H, r.X, r.Y, r.W, r.H)
			}
		}
	}
	// A verb on another tab is not published as being on this one.
	if _, ok := got["watermark"]; ok {
		t.Error("a verb on the Marks tab is published while the Home tab is showing")
	}

	// Pressing where it says runs that verb and nothing else.
	sp := got["rotate"]
	press(s, sp.X+sp.W/2, sp.Y+sp.H/2)
	if got, _ := s.doc.Rotation(1); got != 90 {
		t.Errorf("pressing where Rotate is published left the page turned %d degrees", got)
	}
}

func TestAVerbThatCannotRunIsPublishedAsOff(t *testing.T) {
	// ⛔ A harness that presses a greyed verb and waits for the document to
	// change waits for ever, and reports it as the verb being broken.
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.draw(buffer())
	got := spots(t, s)
	if !got["open"].On {
		t.Error("Open is published as unusable, though it needs no document")
	}
	if got["save"].On {
		t.Error("Save is published as usable with nothing open")
	}
}

func TestChoosingAPublishedTabOpensItsPanel(t *testing.T) {
	s, _ := opened(t, 3)
	s.draw(buffer())
	if _, ok := spots(t, s)[bandPanel]; ok {
		t.Error("a panel is published with the Home tab showing, where there is none")
	}
	tab := spots(t, s)[groupMarks]
	press(s, tab.X+tab.W/2, tab.Y+tab.H/2)
	if s.tools.open != groupMarks {
		t.Fatalf("pressing where the %s tab is published opened %q", groupMarks, s.tools.open)
	}
	// And its verbs, and the panel of settings they read, are published now
	// that they are what is being shown.
	got := spots(t, s)
	if _, ok := got["watermark"]; !ok {
		t.Error("the verbs of the tab now showing are not published")
	}
	panel, ok := got[bandPanel]
	if !ok {
		t.Fatal("the panel the tab opened is not published, so nothing outside can read it")
	}
	if panel.X+panel.W > s.w || panel.Y+panel.H > s.h {
		t.Errorf("the panel is published at %d,%d %dx%d, off a %dx%d canvas",
			panel.X, panel.Y, panel.W, panel.H, s.w, s.h)
	}
}

func TestThePageBandIsWhereTheDocumentIsDrawn(t *testing.T) {
	// ⛔ A band a caller works out for itself goes stale the moment the band
	// above the page changes height, and it goes stale SILENTLY: it still
	// counts pixels, of something else.
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	empty.draw(buffer())
	if spots(t, empty)[bandPage].On {
		t.Error("the page band is published as holding a document with nothing open")
	}

	s, _ := opened(t, 3)
	s.draw(buffer())
	page := spots(t, s)[bandPage]
	if !page.On {
		t.Fatal("the page band is published as empty with a document drawn in it")
	}
	if page.Y < s.toolbarH() {
		t.Errorf("the page band starts at %d and the ribbon ends at %d, so it holds part of the ribbon",
			page.Y, s.toolbarH())
	}
	if page.Y+page.H > s.h-statusH() {
		t.Errorf("the page band ends at %d and the status line starts at %d",
			page.Y+page.H, s.h-statusH())
	}
}

// ⛔⛔ THE PUBLISHED DIVIDER IS DRAGGED HERE, AT THE COORDINATES IT PUBLISHES.
// Everything else the workbench publishes is pressed, and a press is one point;
// the divider only answers to a gesture, and the browser check -- which is the
// only thing that exercises the path from a real pointer to this code -- could
// not make one. It was green before and after a divider that moved nothing,
// which is how that defect reached the person using the application.
//
// This test is the in-process half of closing that: it proves the published
// rectangle is the one a gesture has to start from. The browser check is the
// other half, and it dispatches real mouse events at this same rectangle.
func TestTheDividerIsPublishedWhereAGestureHasToGrabIt(t *testing.T) {
	s, _ := opened(t, 6)
	s.draw(buffer())
	got := spots(t, s)

	h, ok := got[ctlDivider]
	if !ok {
		t.Fatal("the divider is not published, so nothing outside this process can drag it")
	}
	rail, ok := got[bandRail]
	if !ok {
		t.Fatal("the minipages are not published, so a drag has nothing to be judged by")
	}
	if h.W <= 0 || h.H <= 0 {
		t.Fatalf("the divider is published as %dx%d, which cannot be grabbed", h.W, h.H)
	}

	// ⛔ The divider published must be the RAIL's, not the panel's. arrange puts
	// the rail first, so handle 0 is the rail's -- but that is an arrangement,
	// not a law, and a harness told to widen the minipages would otherwise widen
	// the settings. It sits at the rail's right edge.
	if h.X < rail.X+rail.W-toolkit.SplitterHandleW || h.X > rail.X+rail.W+toolkit.SplitterHandleW {
		t.Fatalf("the published divider is at x=%d and the minipages end at x=%d, "+
			"so it is some other divider", h.X, rail.X+rail.W)
	}

	// Now drag it, at the published point, through the workbench's own three
	// entry points -- the ones the browser's mouse events arrive on.
	by := toolkit.Scaled(40)
	x, y := h.X+h.W/2, h.Y+h.H/2
	s.handleClick(x, y)
	s.handleMove(x+by, y)
	s.handleRelease(x+by, y)
	s.draw(buffer())

	after := spots(t, s)
	if after[bandRail].W <= rail.W {
		t.Fatalf("grabbing the published divider and moving right by %d left the minipages "+
			"%d wide; they were %d", by, after[bandRail].W, rail.W)
	}
	// And the divider moved with it, or a second drag would grab where it was.
	if after[ctlDivider].X <= h.X {
		t.Fatalf("the minipages grew and the published divider stayed at x=%d", after[ctlDivider].X)
	}
}

func TestTheDividerIsNotPublishedWhenThereIsNoRailToResize(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	s.draw(buffer())
	got := spots(t, s)
	if _, ok := got[ctlDivider]; ok {
		t.Error("a divider is published with no document open, so a harness would drag nothing")
	}
	if _, ok := got[bandRail]; ok {
		t.Error("a rail is published with no document open")
	}
}

func TestWhatIsPublishedWhenItCannotBeWritten(t *testing.T) {
	was := marshal
	t.Cleanup(func() { marshal = was })
	marshal = func(any) ([]byte, error) { return nil, errors.New("no") }

	s, _ := opened(t, 2)
	got := s.controlsJSON()
	if !strings.Contains(got, "cannot be said") {
		t.Errorf("it published %q", got)
	}
	// ⛔ And it still parses. A caller that cannot read the answer at all
	// cannot tell "the workbench would not say" from "the workbench is not
	// there", and those want different things done about them.
	var back []string
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Errorf("what it published instead does not parse: %v", err)
	}
}
