package main

import (
	"fmt"

	"github.com/go-pdfkit/forms"
	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// A form is the one thing in a PDF that is meant to be changed by whoever
// receives it, and the one thing every other verb in this workbench would
// destroy: rotating a page or laying two on a sheet rebuilds the document, and
// a form is tied into a document by object number in a dozen places at once.
//
// So a form is filled in on the file itself. What was opened is kept, the
// values are put into it, and what is saved is that file with the changes
// appended after it — which is how everything that saves a form saves one.
//
// The panel below is built out of the toolkit's own widgets: a box to type in
// for a text field, a square to tick for a checkbox or a button, and a list to
// choose from for a choice field. Each is bound to the field it stands for
// through the observable it already publishes, so nothing is copied back and
// forth every frame; a change arrives when it happens.

// filling is the form of the document now open, when it has one.
type filling struct {
	// what holds the document, its fields, and the means of writing it back.
	what *ops.Filling
	// rows is the panel, kept so that it is not built again on every frame.
	rows toolkit.Widget
	// changed counts the fields somebody has altered, which is what the
	// status line says and what decides whether saving means anything.
	changed int
}

// readForm looks for a form in what was just opened. A document without one —
// which is nearly every document — simply has none, and the button that shows
// the panel is not offered.
func (s *state) readForm(data []byte) {
	s.form = nil
	what, ok, err := openForm(data)
	if err != nil || !ok {
		return
	}
	s.form = &filling{what: what}
}

// openForm is a variable so that a test can watch what happens when a
// document has a form that cannot be written back to.
var openForm = ops.OpenForm

// showForm puts the panel where the page was, or the page back when it is
// already there. A person filling a form wants to see the fields; a person
// checking their work wants to see the page.
func (s *state) showForm() {
	if s.form == nil {
		s.fail("this document has no form in it")
		return
	}
	s.showingForm = !s.showingForm
	// The form takes the whole view: a document with a hundred fields in it
	// has no room to spare for a panel of verbs beside them, and every one of
	// those verbs would destroy the form anyway.
	s.tools.open = ""
	s.settle()
	s.refresh()
}

// panel builds the rows, once.
func (f *filling) panel(s *state) toolkit.Widget {
	if f.rows != nil {
		return f.rows
	}
	box := toolkit.NewVBox()
	box.Spacing = 6
	for _, field := range f.what.Form().Fields() {
		row := f.row(s, field)
		if row == nil {
			continue
		}
		box.AddFixed(row, formRowH)
	}
	f.rows = toolkit.NewScrollView(box)
	return f.rows
}

// formRowH is how tall one labelled field is: enough for its name, the thing
// that holds it, and a line underneath for what is wrong with it.
const formRowH = 56

// row is one field: what it is called, and the widget that holds it.
func (f *filling) row(s *state, field *forms.Field) toolkit.Widget {
	label := field.Name
	if field.ReadOnly {
		// A field the document says may not be changed is still worth showing,
		// so that somebody can see what it holds and why they cannot type in
		// it.
		return toolkit.NewFormField(label+"  (the document does not allow this to be changed)",
			toolkit.NewLabel(field.Value))
	}
	switch field.Kind {
	case forms.Text:
		entry := toolkit.NewEntry(field.Value)
		entry.Placeholder = placeholderFor(field)
		entry.Text().Subscribe(func(v string) { f.set(s, field, v) })
		return toolkit.NewFormField(label, entry)

	case forms.Checkbox, forms.Radio:
		box := toolkit.NewCheckButton(buttonLabel(field), field.Checked())
		box.Checked().Subscribe(func(on bool) { f.tick(s, field, on) })
		return toolkit.NewFormField(label, box)

	case forms.ComboBox, forms.ListBox:
		options := make([]string, 0, len(field.Options))
		for _, o := range field.Options {
			options = append(options, o.Label)
		}
		if len(options) == 0 {
			return nil
		}
		drop := toolkit.NewDropDown(options, chosenRow(field))
		drop.Selected().Subscribe(func(i int) { f.choose(s, field, i) })
		return toolkit.NewFormField(label, drop)
	}
	// A push button does nothing here and a signature is not a thing this
	// pretends to make.
	return nil
}

// placeholderFor is the hint shown in an empty box: what the document says it
// will take, when it says anything.
func placeholderFor(field *forms.Field) string {
	switch {
	case field.Comb && field.MaxLen > 0:
		return fmt.Sprintf("%d characters, one to a cell", field.MaxLen)
	case field.MaxLen > 0:
		return fmt.Sprintf("up to %d characters", field.MaxLen)
	case field.Multiline:
		return "several lines"
	}
	return ""
}

// buttonLabel says which button of a group this is, when the group has more
// than the usual two.
func buttonLabel(field *forms.Field) string {
	states := field.States()
	if len(states) == 1 {
		return states[0]
	}
	return ""
}

// chosenRow is which row of a choice field is chosen now, or the first when
// none is.
func chosenRow(field *forms.Field) int {
	for i, o := range field.Options {
		if o.Value == field.Value {
			return i
		}
	}
	return 0
}

// set, tick and choose put a value in a field and say so.
func (f *filling) set(s *state, field *forms.Field, v string) {
	f.after(s, field.SetText(v))
}

func (f *filling) tick(s *state, field *forms.Field, on bool) {
	f.after(s, field.SetChecked(on))
}

func (f *filling) choose(s *state, field *forms.Field, row int) {
	if row < 0 || row >= len(field.Options) {
		return
	}
	f.after(s, field.Choose(field.Options[row].Value))
}

// after counts what was changed and says what went wrong, if anything.
func (f *filling) after(s *state, err error) {
	if err != nil {
		s.note = err.Error()
		s.dirty = true
		return
	}
	f.changed = len(f.what.Form().Changed())
	s.note = fmt.Sprintf("%d field(s) filled in", f.changed)
	s.dirty = true
}

// bytes is the file to save: what was opened with the answers appended.
func (f *filling) bytes() ([]byte, string) {
	out, err := f.what.Bytes()
	if err != nil {
		return nil, "this form cannot be saved: " + err.Error()
	}
	return out, ""
}
