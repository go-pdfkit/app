// Turning a printed form into one somebody can fill in.
//
// `Fill in` has filled the fields a document already carries since the form
// panel existed. ops.AddFields has been able to PUT fields into a document for
// as long — `pdfops addfields -fields <description.json>` is the command for it
// — and nothing in the workbench called it, which is the gap inventory's
// "Create Fillable Forms" row: a capability written, tested, released and
// unreachable from the only interface most people will ever use.
//
// ⛔ THE GESTURE IS A SELECTION, because that is the question being asked. A
// blank printed form is a page of QUESTIONS — "Surname", "Date of birth" — and
// what somebody wants is a box to answer each in. Selecting the question and
// saying "a field for this" names the field and places it in one go, where a
// rectangle typed in points would be four numbers somebody has to measure.

package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/go-pdfkit/forms"
	"github.com/go-pdfkit/ops"
)

const (
	// fieldHeight is how tall a field is drawn, in points: a line of type with
	// room to type into it.
	fieldHeight = 16.0
	// fieldGap is the room left between the words and the field that answers
	// them, so the two do not read as one.
	fieldGap = 3.0
	// fieldNameMax bounds the name taken from a passage, in runes. A field's
	// name is what a form says about itself to everything outside it.
	fieldNameMax = 40
)

// fieldKinds are the kinds offered, in the words somebody would use, and in the
// order the chooser lists them.
var fieldKinds = []struct {
	name string
	kind forms.Kind
}{
	{"words", forms.Text},
	{"a tick", forms.Checkbox},
}

// fieldKindNames is the list a person chooses from.
func fieldKindNames() []string {
	out := make([]string, 0, len(fieldKinds))
	for _, k := range fieldKinds {
		out = append(out, k.name)
	}
	return out
}

// makeField puts a fillable field under the selected words.
func (s *state) makeField() {
	quads := s.selectionQuads()
	if len(quads) == 0 {
		s.fail("select the words the field is for: it goes under them")
		return
	}
	// ⛔ The FIRST page the selection reaches, and only that one. A field is one
	// place to write; a selection dragged across two pages is two questions, and
	// guessing which of them was meant is worse than putting the field under the
	// first and saying which page it went on.
	g := quads[0]
	// ⛔ THE TYPED NAME IS CLEANED TOO, not only the one taken from the words.
	// The reason for cleaning is what the name is FOR — it travels outside the
	// document, into exported data and into whatever script fills the form — and
	// that is as true of a name somebody typed as of one read off the page.
	typed := strings.TrimSpace(s.tools.fieldName.Get())
	name := fieldNameFrom(typed)
	// ⛔ The words are the fallback for an EMPTY box, not for one holding
	// something unusable. A name of nothing but punctuation cleans down to
	// nothing, and quietly using the words instead would be answering a question
	// nobody asked — somebody typed that on purpose and is owed the refusal.
	if name == "" && typed == "" {
		name = fieldNameFrom(s.sel.SelectedText())
	}
	if name == "" {
		s.fail("a field needs a name: type one with letters or digits in it, " +
			"or select words it can take one from")
		return
	}
	at := s.tools.fieldKind.Get()
	if at < 0 || at >= len(fieldKinds) {
		at = 0
	}
	kind := fieldKinds[at]
	rect, ok := fieldPlace(g, kind.kind)
	if !ok {
		s.fail(fmt.Sprintf("there is no room for a field beside those words on page %d", g.page))
		return
	}
	if s.changeSaying(fmt.Sprintf("made a field called %q for %s on page %d", name, kind.name, g.page),
		func(d *ops.Doc) error {
			// ⛔ A TICK CARRIES THE NAME OF ITS "ON" STATE, and without one the
			// library refuses the field outright: every checkbox and radio
			// button has a state name of its own, which is how a set of them
			// sharing a value says which was pressed. "Yes" is what nearly every
			// form writes and what `pdfops fields` prints for them.
			on := ""
			if kind.kind == forms.Checkbox {
				on = "Yes"
			}
			return d.AddFields(forms.Field{
				Name: name, Kind: kind.kind,
				Widgets: []forms.Widget{{Page: g.page, Rect: rect, On: on}},
			})
		}) {
		s.sel.Clear()
	}
}

// fieldPlace is where the field goes: under the words, or above them when the
// passage is at the foot of the page.
//
// ⛔ EACH CANDIDATE IS CHECKED against the paper rather than clamped onto it. A
// field clamped to the bottom edge of a page whose last line is 4 points from it
// would be drawn across the words it is meant to answer — the same defect a
// sticky note had, and the reason notePlace chooses rather than clamps.
//
// A tick is a SQUARE: a checkbox as wide as the question it follows is a box
// nobody reads as a checkbox.
// ⛔ A VARIABLE so a test can watch what the VERB does when the placement
// refuses, the way extractImages and openForm are variables for the same
// reason. The geometry is driven directly by the tests below; the refusal it
// can reach — paper smaller than a stamp, where a field fits neither under the
// words nor above them — cannot be reached through a drag, because a selection
// cannot be made on a page drawn that small.
var fieldPlace = func(g pageQuads, kind forms.Kind) ([4]float64, bool) {
	x0, y0, x1, y1 := quadsBox(g.quads)
	wide := x1 - x0
	if kind == forms.Checkbox {
		wide = fieldHeight
	}
	if x0+wide > g.pw {
		x0 = g.pw - wide
	}
	if x0 < 0 {
		return [4]float64{}, false
	}
	// Under the words first, which is where a form puts its answers.
	if under := y0 - fieldGap - fieldHeight; under >= 0 {
		return [4]float64{x0, under, x0 + wide, y0 - fieldGap}, true
	}
	// Then above them, for a question on the last line of the page.
	if above := y1 + fieldGap; above+fieldHeight <= g.ph {
		return [4]float64{x0, above, x0 + wide, above + fieldHeight}, true
	}
	return [4]float64{}, false
}

// fieldNameFrom is a field's name taken from the words it answers: their
// letters and digits, with a space where anything else was.
//
// ⛔ Not the passage as it stands. A field's name travels outside the document
// — into the data somebody exports, into a script that fills it — and a name
// holding a bracket, a colon or a newline is a name every one of those has to
// quote. The words are what a reader recognises; the punctuation is not.
func fieldNameFrom(said string) string {
	var b strings.Builder
	space := false
	for _, r := range said {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	// ⛔ CUT AFTERWARDS, not inside the loop. Breaking out of it when the name
	// had reached its length let one more iteration write a space AND a letter
	// first, so a long passage came back one rune over — found by the test that
	// counted them rather than by reading the loop.
	r := []rune(b.String())
	if len(r) > fieldNameMax {
		return strings.TrimRight(string(r[:fieldNameMax]), " ")
	}
	return b.String()
}
