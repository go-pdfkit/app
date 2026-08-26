package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-pdfkit/forms"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// formPDF writes a document with a form on it: a box to type in, one to tick,
// a list to choose from, one the document says may not be changed, and a push
// button, which is not a thing anybody fills in.
func formPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Reserve()
	blank := w.Add(&reader.Stream{Dict: reader.Dict{"BBox": nums(0, 0, 12, 12)},
		Raw: []byte("")})
	text := w.Add(reader.Dict{
		"FT": reader.Name("Tx"), "T": reader.String("name"),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 150, 180, 175), "MaxLen": reader.Integer(20),
	})
	comb := w.Add(reader.Dict{
		"FT": reader.Name("Tx"), "T": reader.String("code"),
		"Ff": reader.Integer(1 << 24), "MaxLen": reader.Integer(5),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 130, 180, 145),
	})
	many := w.Add(reader.Dict{
		"FT": reader.Name("Tx"), "T": reader.String("story"),
		"Ff":   reader.Integer(1 << 12),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 100, 180, 125),
	})
	tick := w.Add(reader.Dict{
		"FT": reader.Name("Btn"), "T": reader.String("agree"),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 80, 32, 92),
		"AP":   reader.Dict{"N": reader.Dict{"Off": blank, "Yes": blank}},
	})
	list := w.Add(reader.Dict{
		"FT": reader.Name("Ch"), "T": reader.String("where"),
		"Ff":   reader.Integer(1 << 17),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 60, 180, 75),
		"Opt": reader.Array{
			reader.Array{reader.String("FR"), reader.String("France")},
			reader.Array{reader.String("BE"), reader.String("Belgique")},
		},
		"V": reader.String("BE"),
	})
	empty := w.Add(reader.Dict{
		"FT": reader.Name("Ch"), "T": reader.String("nothing"),
		"Ff":   reader.Integer(1 << 17),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 40, 180, 55),
	})
	locked := w.Add(reader.Dict{
		"FT": reader.Name("Tx"), "T": reader.String("serial"),
		"Ff": reader.Integer(1), "V": reader.String("A-1756"),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(20, 20, 180, 35),
	})
	press := w.Add(reader.Dict{
		"FT": reader.Name("Btn"), "T": reader.String("print"),
		"Ff":   reader.Integer(1 << 16),
		"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"), "P": pageRef,
		"Rect": nums(150, 20, 180, 35),
	})
	fields := reader.Array{text, comb, many, tick, list, empty, locked, press}
	w.Put(pageRef, reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200), "Annots": fields,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef}, "Count": reader.Integer(1)})
	out, err := w.Finish(reader.Dict{"Root": w.Add(reader.Dict{
		"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"AcroForm": w.Add(reader.Dict{"Fields": fields,
			"DA": reader.String("/Helv 0 Tf 0 g"),
			"DR": reader.Dict{"Font": reader.Dict{"Helv": w.Add(reader.Dict{
				"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
				"BaseFont": reader.Name("Helvetica"),
				"Encoding": reader.Name("WinAnsiEncoding")})}}})})})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func nums(vs ...float64) reader.Array {
	out := make(reader.Array, len(vs))
	for i, v := range vs {
		out[i] = reader.Real(v)
	}
	return out
}

// openedForm opens the workbench on a document with a form in it.
func openedForm(t *testing.T) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "form.pdf", file: formPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	if s.form == nil {
		t.Fatalf("the document has a form and the workbench did not find it: %q", s.note)
	}
	return s, h
}

func TestADocumentWithAFormSaysSoOnOpening(t *testing.T) {
	s, _ := openedForm(t)
	if !strings.Contains(s.note, "form") {
		t.Errorf("the workbench said %q", s.note)
	}
	if got := len(s.form.what.Form().Fields()); got != 8 {
		t.Errorf("found %d fields", got)
	}
}

func TestADocumentWithNoFormOffersNothingToFillIn(t *testing.T) {
	s, _ := opened(t, 2)
	if s.form != nil {
		t.Error("a document with no form was given one")
	}
	s.showForm()
	if !strings.Contains(s.note, "no form") {
		t.Errorf("the workbench said %q", s.note)
	}
	if s.showingForm {
		t.Error("the panel was shown for a document with no form")
	}
}

func TestTheFieldsAreShownInsteadOfThePage(t *testing.T) {
	s, _ := openedForm(t)
	page := buffer()
	s.draw(page)
	s.showForm()
	if !s.showingForm {
		t.Fatal("the panel was not shown")
	}
	panel := buffer()
	s.draw(panel)
	if inked(panel, s.theme.Background) == 0 {
		t.Error("the panel drew nothing at all")
	}
	same := 0
	for i := range page {
		if page[i] == panel[i] {
			same++
		}
	}
	if same == len(page) {
		t.Error("the panel is the same picture as the page")
	}
	if got := s.statusLine()[1]; !strings.Contains(got, "8 fields") {
		t.Errorf("the status line says %q", got)
	}
	// And back to the page.
	s.showForm()
	if s.showingForm {
		t.Error("pressing again did not go back to the page")
	}
}

// inputOf digs the actual input out of a labelled row, since that is what a
// person touches and therefore what a test has to touch. Calling the setters
// behind the panel would prove the setters work and say nothing about whether
// anything is wired to them.
func inputOf(t *testing.T, row toolkit.Widget) toolkit.Widget {
	t.Helper()
	field, ok := row.(*toolkit.FormField)
	if !ok {
		t.Fatalf("the row is a %T, not a labelled field", row)
	}
	if field.Child == nil {
		t.Fatal("the row has nothing to type into")
	}
	return field.Child
}

func TestTypingIntoTheBoxFillsTheFieldIn(t *testing.T) {
	s, _ := openedForm(t)
	field, ok := s.form.what.Form().Field("name")
	if !ok {
		t.Fatal("no such field")
	}
	entry, ok := inputOf(t, s.form.row(s, field)).(*toolkit.Entry)
	if !ok {
		t.Fatal("a text field is not offered a box to type in")
	}
	entry.SetText("Mozart")
	if field.Value != "Mozart" {
		t.Errorf("the field holds %q", field.Value)
	}
	if s.form.changed != 1 {
		t.Errorf("%d fields counted as changed", s.form.changed)
	}
	if !strings.Contains(s.note, "1 field") {
		t.Errorf("the workbench said %q", s.note)
	}
}

func TestTickingTheBoxTicksTheField(t *testing.T) {
	s, _ := openedForm(t)
	field, _ := s.form.what.Form().Field("agree")
	box, ok := inputOf(t, s.form.row(s, field)).(*toolkit.CheckButton)
	if !ok {
		t.Fatal("a checkbox is not offered a square to tick")
	}
	box.Checked().Set(true)
	if !field.Checked() {
		t.Errorf("the field holds %q", field.Value)
	}
	box.Checked().Set(false)
	if field.Checked() {
		t.Errorf("unticked, the field holds %q", field.Value)
	}
}

func TestChoosingFromTheListChoosesInTheField(t *testing.T) {
	s, _ := openedForm(t)
	field, _ := s.form.what.Form().Field("where")
	drop, ok := inputOf(t, s.form.row(s, field)).(*toolkit.DropDown)
	if !ok {
		t.Fatal("a choice field is not offered a list")
	}
	drop.Select(0)
	if field.Value != "FR" {
		t.Errorf("the field holds %q", field.Value)
	}
	// A row that is not there changes nothing rather than breaking.
	before := field.Value
	s.form.choose(s, field, 9)
	s.form.choose(s, field, -1)
	if field.Value != before {
		t.Errorf("a row that does not exist changed it to %q", field.Value)
	}
}
func TestAFieldTheDocumentWillNotAllowToBeChanged(t *testing.T) {
	s, _ := openedForm(t)
	locked, _ := s.form.what.Form().Field("serial")
	s.form.set(s, locked, "something else")
	if locked.Value != "A-1756" {
		t.Errorf("a read-only field was changed to %q", locked.Value)
	}
	if !strings.Contains(s.note, "read-only") {
		t.Errorf("the workbench said %q", s.note)
	}
}

func TestSavingAFilledFormKeepsItAForm(t *testing.T) {
	// Every other verb here rebuilds the document, and a form does not
	// survive that. A filled form is saved as the file it came from with the
	// answers appended.
	s, h := openedForm(t)
	field, _ := s.form.what.Form().Field("name")
	s.form.set(s, field, "Mozart")
	s.save()
	if len(h.saved) == 0 {
		t.Fatalf("nothing was saved: %q", s.note)
	}
	d, err := reader.Open(h.saved)
	if err != nil {
		t.Fatalf("what was saved cannot be read: %v", err)
	}
	back, ok := forms.Read(d)
	if !ok {
		t.Fatal("what was saved has no form in it")
	}
	got, _ := back.Field("name")
	if got.Value != "Mozart" {
		t.Errorf("the saved form holds %q", got.Value)
	}
}

func TestSavingAFormNobodyFilledInGoesTheUsualWay(t *testing.T) {
	s, h := openedForm(t)
	s.save()
	if len(h.saved) == 0 {
		t.Fatalf("nothing was saved: %q", s.note)
	}
	if _, err := reader.Open(h.saved); err != nil {
		t.Errorf("what was saved cannot be read: %v", err)
	}
}

func TestAFormThatCannotBeSaved(t *testing.T) {
	s, _ := openedForm(t)
	field, _ := s.form.what.Form().Field("name")
	s.form.set(s, field, "Mozart")
	// A file the reader had to repair has no cross-reference section worth
	// pointing back at, which is what the writing refuses.
	s.form.what = brokenFilling(t)
	if _, msg := s.form.bytes(); msg == "" {
		t.Error("a form that cannot be written said nothing")
	}
	s.save()
	if !strings.Contains(s.note, "cannot") {
		t.Errorf("the workbench said %q", s.note)
	}
}

// brokenFilling is a form whose fields cannot be written back, because each is
// written inside the field list rather than as an object of its own.
func brokenFilling(t *testing.T) *ops.Filling {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200),
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef}, "Count": reader.Integer(1)})
	out, err := w.Finish(reader.Dict{"Root": w.Add(reader.Dict{
		"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"AcroForm": w.Add(reader.Dict{"DA": reader.String("/Helv 0 Tf 0 g"),
			"Fields": reader.Array{reader.Dict{
				"FT": reader.Name("Tx"), "T": reader.String("inline"),
				"Rect": nums(0, 0, 100, 20)}}})})})
	if err != nil {
		t.Fatal(err)
	}
	f, ok, err := ops.OpenForm(out)
	if err != nil || !ok {
		t.Fatalf("ok %v err %v", ok, err)
	}
	fld, _ := f.Form().Field("inline")
	if err := fld.SetText("x"); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestADocumentWhoseFormCannotBeOpened(t *testing.T) {
	// The workbench still opens the document; there is simply nothing to fill
	// in, which is the same as any other document.
	was := openForm
	defer func() { openForm = was }()
	openForm = func(b []byte) (*ops.Filling, bool, error) {
		return nil, false, errRefused
	}
	h := &fakeHost{name: "form.pdf", file: formPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("the document did not open")
	}
	if s.form != nil {
		t.Error("a form was made out of nothing")
	}
}

func TestWhatEachSortOfFieldOffers(t *testing.T) {
	s, _ := openedForm(t)
	form := s.form.what.Form()
	for _, c := range []struct {
		name string
		want string
	}{
		{"name", "up to 20 characters"},
		{"code", "5 characters, one to a cell"},
		{"story", "several lines"},
	} {
		field, ok := form.Field(c.name)
		if !ok {
			t.Fatalf("no field called %q", c.name)
		}
		if got := placeholderFor(field); got != c.want {
			t.Errorf("%s offers %q, wanted %q", c.name, got, c.want)
		}
	}
	tick, _ := form.Field("agree")
	if got := buttonLabel(tick); got != "Yes" {
		t.Errorf("the box is labelled %q", got)
	}
	where, _ := form.Field("where")
	if got := chosenRow(where); got != 1 {
		t.Errorf("the chosen row is %d, wanted the second", got)
	}
	// A field holding something that is not one of its rows starts at the
	// first, since it has to start somewhere.
	where.Value = "Andorre"
	if got := chosenRow(where); got != 0 {
		t.Errorf("a value that is not a row chose row %d", got)
	}
	// A plain box with nothing said about it offers no hint, and something
	// with no buttons at all is labelled by nothing.
	plain, _ := form.Field("story")
	plain.Multiline = false
	if got := placeholderFor(plain); got != "" {
		t.Errorf("a plain box offers %q", got)
	}
	if got := buttonLabel(where); got != "" {
		t.Errorf("something with no buttons is labelled %q", got)
	}
	// A choice field with no rows in it has nothing to show.
	none, _ := form.Field("nothing")
	if row := s.form.row(s, none); row != nil {
		t.Error("a choice field with no rows was given a control")
	}
	// A push button is not a thing anybody fills in.
	press, _ := form.Field("print")
	if row := s.form.row(s, press); row != nil {
		t.Error("a push button was given a control")
	}
	// A field the document locks is shown, so that its value can be read.
	locked, _ := form.Field("serial")
	if row := s.form.row(s, locked); row == nil {
		t.Error("a read-only field was not shown at all")
	}
}

func TestThePanelIsBuiltOnce(t *testing.T) {
	s, _ := openedForm(t)
	first := s.form.panel(s)
	if first != s.form.panel(s) {
		t.Error("the panel was built again")
	}
	if _, ok := first.(*toolkit.ScrollView); !ok {
		t.Errorf("the panel is a %T, and a form longer than the window has to scroll", first)
	}
}

// errRefused stands for whatever the verb layer says when it will not open a
// form.
var errRefused = errors.New("refused")
