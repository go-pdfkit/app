package main

import "testing"

func TestTheAdapterForwardsEveryEvent(t *testing.T) {
	h := &fakeHost{name: "sample.pdf", file: samplePDF(t, 2)}
	a := newWorkbench(h)
	if w, hgt := a.Size(); w != surfaceW || hgt != surfaceH {
		t.Errorf("Size() = %dx%d", w, hgt)
	}
	buf := buffer()
	a.Draw(buf)
	if inked(buf, a.s.theme.Background) == 0 {
		t.Error("Draw painted nothing")
	}
	// Every event method reaches the scene and says whether it mattered.
	if !a.Click(10, 10) || !a.Move(10, 10) || !a.Release(10, 10) || !a.Context(10, 10) {
		t.Error("a pointer event was not taken")
	}
	if a.Char("x") {
		t.Error("the workbench claimed a character it has nowhere to put")
	}
	a.s.open()
	if !a.KeyDown("ArrowRight") {
		t.Error("the right arrow was not taken")
	}
	if a.s.at != 2 {
		t.Errorf("the arrow took it to page %d", a.s.at)
	}
	if a.KeyDown("KeyQ") {
		t.Error("a key with nothing to do was claimed")
	}
}

func TestAFileThatArrivesOnItsOwnIsShown(t *testing.T) {
	// The browser hands a file over well after the press that asked for it,
	// with no event of its own. The harness asks for a frame instead, and
	// the workbench has to say that something changed — once, and not again
	// while nothing does.
	h := &fakeHost{name: "sample.pdf", file: samplePDF(t, 2)}
	a := newWorkbench(h)
	a.AnimationStep(0.016) // whatever building it changed
	if a.AnimationStep(0.016) {
		t.Error("a still workbench asked to be repainted")
	}
	a.s.open()
	if !a.AnimationStep(0.016) {
		t.Error("a document that arrived on its own was not shown")
	}
	if a.AnimationStep(0.016) {
		t.Error("the same document asked to be shown twice")
	}
}
