package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/forms"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// fieldsOf is the form of the document as it would be SAVED, read back out of
// the written bytes — which is the only place a field exists: AddFields is a
// decision the writer is handed.
func fieldsOf(t *testing.T, s *state) []*forms.Field {
	t.Helper()
	out, msg := s.reopenBytes()
	if msg != "" {
		t.Fatalf("the document could not be written: %s", msg)
	}
	doc, err := reader.Open(out)
	if err != nil {
		t.Fatalf("the written document cannot be opened: %v", err)
	}
	form, ok := forms.Read(doc)
	if !ok {
		t.Fatal("the written document carries no form")
	}
	return form.Fields()
}

// selectedWordy opens the one-page wordy fixture with its line selected.
func selectedWordy(t *testing.T) *state {
	t.Helper()
	h := &fakeHost{name: "wordy.pdf", file: wordyPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	selectTheWord(t, s, "Hello there")
	return s
}

func TestAFieldIsMadeUnderTheWordsItAnswers(t *testing.T) {
	s := selectedWordy(t)

	s.makeField()

	if !strings.Contains(s.note, `made a field called "Hello there"`) {
		t.Fatalf("the status line says %q", s.note)
	}
	got := fieldsOf(t, s)
	if len(got) != 1 {
		t.Fatalf("the written document has %d fields", len(got))
	}
	f := got[0]
	if f.Name != "Hello there" {
		t.Errorf("the field is called %q", f.Name)
	}
	if f.Kind != forms.Text {
		t.Errorf("the field is a %s", f.Kind)
	}
	if len(f.Widgets) != 1 || f.Widgets[0].Page != 1 {
		t.Fatalf("its widgets are %+v", f.Widgets)
	}
	// ⛔ UNDER the words: the page draws them with their baseline at y=300, so
	// a field answering them sits below that and above the foot of the page.
	r := f.Widgets[0].Rect
	if !(r[1] < 300 && r[1] > 0) {
		t.Errorf("the field's box is %v, which is not under the words", r)
	}
	if r[3]-r[1] < fieldHeight-0.01 || r[3]-r[1] > fieldHeight+0.01 {
		t.Errorf("the field is %.1f points tall, want %.1f", r[3]-r[1], fieldHeight)
	}
	if r[2] <= r[0] {
		t.Errorf("the field has no width: %v", r)
	}
	// And the selection is let go, as every verb that acts on one does.
	if !s.sel.IsEmpty() {
		t.Error("the selection survived the verb")
	}
}

func TestATickIsASquareAndTheNameBoxWins(t *testing.T) {
	s := selectedWordy(t)
	s.tools.fieldName.Set("Agreed")
	s.tools.fieldKind.Set(1)

	s.makeField()

	got := fieldsOf(t, s)
	if len(got) != 1 {
		t.Fatalf("the written document has %d fields: %q", len(got), s.note)
	}
	if got[0].Name != "Agreed" {
		t.Errorf("the name typed beside it was not used: %q", got[0].Name)
	}
	if got[0].Kind != forms.Checkbox {
		t.Errorf("the field is a %s", got[0].Kind)
	}
	r := got[0].Widgets[0].Rect
	if w, h := r[2]-r[0], r[3]-r[1]; w != h {
		t.Errorf("a tick is %.1f by %.1f, which is not a square", w, h)
	}
}

func TestMakingAFieldNeedsASelectionAndAName(t *testing.T) {
	// Nothing selected.
	h := &fakeHost{name: "wordy.pdf", file: wordyPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	s.makeField()
	if !strings.Contains(s.note, "select the words the field is for") {
		t.Errorf("with nothing selected it says %q", s.note)
	}

}

// TestANameOfPunctuationAloneIsRefused: the typed name is cleaned like any
// other, so a name of nothing but punctuation cleans down to nothing — and a
// field with no name is one nothing outside the document can address.
func TestANameOfPunctuationAloneIsRefused(t *testing.T) {
	s := selectedWordy(t)
	s.tools.fieldName.Set("...")
	// ⛔ The words are selected, so this is about the NAME and not about the
	// selection: without the typed name it would be called "Hello there".
	s.makeField()

	if !strings.Contains(s.note, "a field needs a name") {
		t.Errorf("the status line says %q", s.note)
	}
}

// TestAKindThatIsNotOneFallsBackOnWords: the chooser holds a position, and a
// position outside the list is a reading of it that must not go past the end.
func TestAKindThatIsNotOneFallsBackOnWords(t *testing.T) {
	s := selectedWordy(t)
	s.tools.fieldKind.Set(99)

	s.makeField()

	got := fieldsOf(t, s)
	if len(got) != 1 {
		t.Fatalf("the written document has %d fields: %q", len(got), s.note)
	}
	if got[0].Kind != forms.Text {
		t.Errorf("a kind of 99 made a %s", got[0].Kind)
	}
}

// TestNoRoomForAFieldReachesTheStatusLine drives the refusal through the VERB:
// the tests below say what the placement decides, this says that what it decides
// reaches the person.
//
// ⛔ Through the seam rather than through a tiny page. A field fits neither
// under a line nor above it only on paper smaller than a stamp, and a page drawn
// that small takes no selection at all — a press on it does not begin one, which
// this test measured before it was written this way.
func TestNoRoomForAFieldReachesTheStatusLine(t *testing.T) {
	s := selectedWordy(t)
	was := fieldPlace
	fieldPlace = func(pageQuads, forms.Kind) ([4]float64, bool) { return [4]float64{}, false }
	t.Cleanup(func() { fieldPlace = was })

	s.makeField()

	if !strings.Contains(s.note, "no room for a field") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := s.doc.PageCount(); got != 1 {
		t.Errorf("the refusal changed the document: %d pages", got)
	}
}

// TestAFieldForWordsAtTheFootOfThePageGoesAboveThem: each candidate is checked
// against the paper rather than clamped onto it, which is the lesson a sticky
// note taught — a field clamped to the bottom edge would be drawn across the
// very words it answers.
func TestAFieldForWordsAtTheFootOfThePageGoesAboveThem(t *testing.T) {
	low := pageQuads{page: 1, pw: 200, ph: 300, quads: []ops.Quad{{
		TopLeftX: 20, TopLeftY: 12, TopRightX: 120, TopRightY: 12,
		BottomLeftX: 20, BottomLeftY: 2, BottomRightX: 120, BottomRightY: 2,
	}}}

	r, ok := fieldPlace(low, forms.Text)

	if !ok {
		t.Fatal("a line near the foot of the page was given no field at all")
	}
	if r[1] < 12 {
		t.Errorf("the field is at %v, which is not above words ending at y=12", r)
	}
	if r[3] > low.ph {
		t.Errorf("the field at %v runs off the top of a %.0f point page", r, low.ph)
	}
}

// TestAPageWithRoomForNeitherIsRefused is the far end of the same rule.
func TestAPageWithRoomForNeitherIsRefused(t *testing.T) {
	// A page barely taller than a field, with the words across the whole of it:
	// there is room neither under nor above.
	tight := pageQuads{page: 1, pw: 200, ph: fieldHeight + 2, quads: []ops.Quad{{
		TopLeftX: 0, TopLeftY: fieldHeight + 1, TopRightX: 100, TopRightY: fieldHeight + 1,
		BottomLeftX: 0, BottomLeftY: 1, BottomRightX: 100, BottomRightY: 1,
	}}}

	if r, ok := fieldPlace(tight, forms.Text); ok {
		t.Errorf("a page with no room gave a field at %v", r)
	}
	// A page narrower than the tick it is asked for: there is nowhere across.
	narrow := pageQuads{page: 1, pw: fieldHeight / 2, ph: 300, quads: []ops.Quad{{
		TopLeftX: 0, TopLeftY: 100, TopRightX: 5, TopRightY: 100,
		BottomLeftX: 0, BottomLeftY: 90, BottomRightX: 5, BottomRightY: 90,
	}}}
	if r, ok := fieldPlace(narrow, forms.Checkbox); ok {
		t.Errorf("a page narrower than a tick gave one at %v", r)
	}
}

// TestAFieldsNameIsTheWordsWithoutThePunctuation: a field's name travels
// outside the document, into exported data and into whatever script fills it,
// and a name holding a colon or a bracket is one every reader has to quote.
func TestAFieldsNameIsTheWordsWithoutThePunctuation(t *testing.T) {
	for _, c := range []struct{ said, want string }{
		{"Surname:", "Surname"},
		{"  Date of birth (dd/mm/yyyy)  ", "Date of birth dd mm yyyy"},
		{"Nº 7 — signed?", "Nº 7 signed"},
		{"...", ""},
	} {
		if got := fieldNameFrom(c.said); got != c.want {
			t.Errorf("%q became %q, want %q", c.said, got, c.want)
		}
	}
	// ⛔ A long passage is cut to the bound and not one rune past it, which is
	// where the first version of this was wrong: it broke out of its loop after
	// appending, and an iteration can append a space AND a letter.
	long := fieldNameFrom(strings.Repeat("word ", 50))
	if n := len([]rune(long)); n > fieldNameMax {
		t.Errorf("a long passage gave a name of %d runes, want at most %d", n, fieldNameMax)
	}
	if strings.HasSuffix(long, " ") {
		t.Errorf("a cut name ends in a space: %q", long)
	}
}

// TestASecondFieldOfTheSameNameIsRefusedByTheLibrary: two fields under one name
// is a form that has lost one of them, and the refusal is the useful answer.
func TestASecondFieldOfTheSameNameIsRefusedByTheLibrary(t *testing.T) {
	s := selectedWordy(t)
	s.makeField()
	if len(fieldsOf(t, s)) != 1 {
		t.Fatalf("the first field was not made: %q", s.note)
	}

	selectTheWord(t, s, "Hello there")
	s.makeField()

	if strings.Contains(s.note, "made a field") {
		t.Errorf("a second field of the same name was accepted: %q", s.note)
	}
	if got := len(fieldsOf(t, s)); got != 1 {
		t.Errorf("the document carries %d fields", got)
	}
}
