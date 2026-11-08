package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// shapesOn is the shape annotations the written document carries on page one,
// as their subtypes.
func shapesOn(t *testing.T, s *state) []reader.Name {
	t.Helper()
	var out []reader.Name
	for _, d := range annotsOf(t, s, 1) {
		name, _ := d.Get("Subtype").(reader.Name)
		switch name {
		case "Square", "Circle", "Line":
			out = append(out, name)
		}
		_ = d
	}
	return out
}

// ⛔⛔ EACH SHAPE GOES ROUND THE WORDS AND NOT OVER THEM. A box or a ring drawn
// on the glyphs would hide what it is drawing attention to, which is the same
// failure the sticky note had — so the rectangle is grown beyond the passage,
// and this measures that rather than trusting it.
func TestABoxAndARingGoRoundTheWords(t *testing.T) {
	for _, c := range []struct {
		name    string
		run     func(*state)
		subtype reader.Name
		says    string
	}{
		{"a box", (*state).boxSelection, "Square", "drew a box round"},
		{"a ring", (*state).ringSelection, "Circle", "drew a ring round"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, perGlyphPDF(t))
			selectFirstLine(t, s)
			groups := s.selectionQuads()
			if len(groups) == 0 {
				t.Fatal("the selection reaches no page")
			}
			px0, py0, px1, py1 := quadsBox(groups[0].quads)
			c.run(s)
			if !strings.Contains(s.note, c.says) {
				t.Fatalf("the status line says %q", s.note)
			}
			if got := shapesOn(t, s); len(got) != 1 || got[0] != c.subtype {
				t.Fatalf("the page carries %v, not one /%s", got, c.subtype)
			}
			d := onlyAnnotOfSubtype(t, s, c.subtype)
			rect := rectOf(t, s, d)
			// ⛔ The shape's rectangle CONTAINS the passage, with room: a border
			// exactly on the glyphs is a border through them.
			if rect[0] >= px0 || rect[1] >= py0 || rect[2] <= px1 || rect[3] <= py1 {
				t.Errorf("the shape's rectangle is %v and the words are "+
					"%.1f %.1f %.1f %.1f: it does not enclose them",
					rect, px0, py0, px1, py1)
			}
			// And it stays on the paper: perGlyphPDF is 300 by 400.
			if rect[0] < 0 || rect[1] < 0 || rect[2] > 300 || rect[3] > 400 {
				t.Errorf("the shape runs off the paper, at %v", rect)
			}
			if !s.sel.IsEmpty() {
				t.Error("the words are still selected")
			}
		})
	}
}

// ⛔⛔ AN ARROW STOPS SHORT OF THE WORDS AND COMES FROM THE MARGIN. A line drawn
// THROUGH a passage would be a strike-out, and there is already a verb for that
// two controls along — so this asserts the head is outside the passage and the
// tail further out still.
func TestAnArrowPointsAtTheWordsFromOutside(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	selectFirstLine(t, s)
	groups := s.selectionQuads()
	if len(groups) == 0 {
		t.Fatal("the selection reaches no page")
	}
	px0, _, px1, _ := quadsBox(groups[0].quads)
	s.arrowToSelection()
	if !strings.Contains(s.note, "drew an arrow at the words") {
		t.Fatalf("the status line says %q", s.note)
	}
	d := onlyAnnotOfSubtype(t, s, "Line")
	// The L array holds the two ends: the tail first, the head second.
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	obj, err := src.Resolve(d.Get("L"))
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := reader.ToArray(obj)
	if !ok || len(arr) != 4 {
		t.Fatalf("the /L is %v", obj)
	}
	var l [4]float64
	for i, v := range arr {
		n, ok := reader.ToFloat(v)
		if !ok {
			t.Fatalf("the /L holds %v", v)
		}
		l[i] = n
	}
	tailX, headX := l[0], l[2]
	// The passage starts 19.7 points from the left edge on this fixture, so
	// there is no room in the left margin: the arrow must come from the RIGHT,
	// which is the branch worth pinning down because the obvious one is left.
	if headX <= px1 {
		t.Errorf("the arrow's head is at x=%.1f and the words end at x=%.1f: "+
			"it points into them", headX, px1)
	}
	if tailX <= headX {
		t.Errorf("the arrow runs from x=%.1f to x=%.1f, so it points away from "+
			"the words", tailX, headX)
	}
	if headX < px0 {
		t.Errorf("the arrow's head is at x=%.1f, left of the words at %.1f",
			headX, px0)
	}
	// It asks for a filled head, which Table 176 makes conditional on there
	// being an interior colour.
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "/ClosedArrow") {
		t.Error("the arrow's /LE does not name ClosedArrow")
	}
	if !strings.Contains(string(out), "/IC") {
		t.Error("the arrow wrote no interior colour, so its head is an outline")
	}
}

// ⛔ The arrow comes from the LEFT when there is room there, which is the other
// half of the choice above. Measured on paper wide enough to have a left
// margin: without this the right-hand branch could be the only one that works.
func TestAnArrowComesFromTheLeftWhenThereIsRoom(t *testing.T) {
	// 300 points wide, with the line starting at x 20 — so the left margin is
	// 20 points and too narrow; a wider page moves the words right instead.
	s := shown(t, indentedPagePDF(t))
	selectFirstLine(t, s)
	groups := s.selectionQuads()
	if len(groups) == 0 {
		t.Fatal("the selection reaches no page")
	}
	px0, _, _, _ := quadsBox(groups[0].quads)
	if px0 < shapeRoom+arrowReach+shapeInk {
		t.Fatalf("the fixture's words start at x=%.1f, so there is no left "+
			"margin to measure", px0)
	}
	s.arrowToSelection()
	d := onlyAnnotOfSubtype(t, s, "Line")
	src, msg := s.reopen()
	if msg != "" {
		t.Fatal(msg)
	}
	obj, err := src.Resolve(d.Get("L"))
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := reader.ToArray(obj)
	tail, _ := reader.ToFloat(arr[0])
	head, _ := reader.ToFloat(arr[2])
	if head >= px0 {
		t.Errorf("the arrow's head is at x=%.1f and the words start at x=%.1f",
			head, px0)
	}
	if tail >= head {
		t.Errorf("the arrow runs from x=%.1f to x=%.1f, so it points away",
			tail, head)
	}
}

// indentedPagePDF sets perGlyphPDF's line well in from the left edge, so there
// is a margin an arrow can come from.
func indentedPagePDF(t *testing.T) []byte {
	t.Helper()
	return pagePDFOfSizeAt(t, 400, 400, 120)
}

// ⛔ Neither margin wide enough is a refusal with the number and a way forward,
// not a line drawn through the words.
func TestAnArrowWithNoMarginIsRefused(t *testing.T) {
	// Paper only as wide as the words: the line runs x 20..96 on a 110-point
	// page, so neither side has the 40 points an arrow needs.
	s := shown(t, pagePDFOfSize(t, 110, 400))
	selectFirstLine(t, s)
	s.arrowToSelection()
	if !strings.Contains(s.note, "neither margin") {
		t.Fatalf("the status line says %q", s.note)
	}
	if !strings.Contains(s.note, "box round them instead") {
		t.Errorf("the refusal offers no way forward: %q", s.note)
	}
	if got := shapesOn(t, s); len(got) != 0 {
		t.Errorf("%v was drawn anyway", got)
	}
}

func TestDrawingWithNothingSelected(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"a box", (*state).boxSelection},
		{"a ring", (*state).ringSelection},
		{"an arrow", (*state).arrowToSelection},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := shown(t, perGlyphPDF(t))
			c.run(s)
			if !strings.Contains(s.note, "nothing is selected") {
				t.Errorf("the status line says %q", s.note)
			}
			if got := shapesOn(t, s); len(got) != 0 {
				t.Errorf("%v was drawn anyway", got)
			}
		})
	}
}

// ⛔⛔ A PASSAGE AT THE EDGE OF ITS PAPER STILL GETS A BOX, clamped to the
// page rather than refused. The first version of this test accepted EITHER a
// refusal or a success — an or-assertion, which passes whatever happens and
// measures nothing; it passed through the success arm while the refusal it was
// supposedly about could not fire at all. What is true is this: the box is
// drawn, and it stays on the paper.
func TestWordsAtTheEdgeOfThePaperStillGetABox(t *testing.T) {
	// A page barely wider than its line: x 20..96 on a 99-point page, so the
	// box's left and right edges both want to be outside the paper.
	const pw = 99
	s := shown(t, pagePDFOfSize(t, pw, 400))
	selectFirstLine(t, s)
	s.boxSelection()
	if !strings.Contains(s.note, "drew a box round") {
		t.Fatalf("the status line says %q", s.note)
	}
	d := onlyAnnotOfSubtype(t, s, "Square")
	rect := rectOf(t, s, d)
	// ops grows the Rect by half the line width, so the annotation's own
	// rectangle may reach that far past the clamped box and no further.
	const slack = shapeInk
	if rect[0] < -slack || rect[2] > pw+slack {
		t.Errorf("the box runs from x=%.1f to x=%.1f on a %d-point page",
			rect[0], rect[2], pw)
	}
}

// ⛔ A shape is signed by whoever the file says is writing, as the comments are.
func TestAShapeIsSignedLikeAComment(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	s.tools.author.Set("a careful reader")
	selectFirstLine(t, s)
	s.boxSelection()
	d := onlyAnnotOfSubtype(t, s, "Square")
	if got, _ := d.Get("T").(reader.String); string(got) != "a careful reader" {
		t.Errorf("the shape is signed %q", got)
	}
}
