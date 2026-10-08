package main

import (
	"fmt"

	"github.com/go-pdfkit/forms"
	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/mvvm/tkbind"
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

	// One Observable per field, which the control and the DOCUMENT both speak
	// through. The control is bound to it two ways; a subscription writes it
	// into the file and puts back what the file kept.
	//
	// ⛔ That last part is a defect being fixed, not a refinement. SetText
	// TRUNCATES to MaxLen and strips newlines, and says nothing about it; an
	// uneditable ComboBox refuses a value outright. With the control wired one
	// way -- Subscribe, box to document -- the box went on showing twelve
	// characters while the file held eight, and the only place that knew was
	// the file.
	text map[*forms.Field]*mvvm.Observable[string]
	tick map[*forms.Field]*mvvm.Observable[bool]
	pick map[*forms.Field]*mvvm.Observable[int]
}

// textOf is the field's value as something the control can be bound to. Made
// once per field and kept, so that rebuilding a row does not stack another
// subscription on the same field.
func (f *filling) textOf(s *state, field *forms.Field) *mvvm.Observable[string] {
	if v, ok := f.text[field]; ok {
		return v
	}
	v := mvvm.NewObservable(field.Value)
	v.Subscribe(func(x string) {
		f.after(s, field.SetText(x))
		// What the document KEPT. Set is re-entrant by design -- a Set from
		// inside a notification re-notifies with the newer value until it
		// settles -- so this lands on the control and stops.
		v.Set(field.Value)
	})
	f.text[field] = v
	return v
}

// tickOf is the same for a box that is ticked or not.
//
// ⛔ No write-back here, and none in pickOf, though the symmetry is tempting.
// SetChecked cannot keep anything other than what it is handed on a field a
// control was built for -- a read-only one is shown as a label and never
// reaches here -- and Choose is only ever given one of the field's own
// options. Both write-backs were written, and both SURVIVED the mutation run:
// no test could tell them from nothing, because there is nothing for them to
// do. Keeping unreachable symmetry is how a file fills with code that looks
// like it is protecting something.
func (f *filling) tickOf(s *state, field *forms.Field) *mvvm.Observable[bool] {
	if v, ok := f.tick[field]; ok {
		return v
	}
	v := mvvm.NewObservable(field.Checked())
	v.Subscribe(func(on bool) { f.after(s, field.SetChecked(on)) })
	f.tick[field] = v
	return v
}

// pickOf is the same for a row chosen out of a list.
func (f *filling) pickOf(s *state, field *forms.Field) *mvvm.Observable[int] {
	if v, ok := f.pick[field]; ok {
		return v
	}
	v := mvvm.NewObservable(chosenRow(field))
	v.Subscribe(func(row int) {
		if row < 0 || row >= len(field.Options) {
			return
		}
		f.after(s, field.Choose(field.Options[row].Value))
	})
	f.pick[field] = v
	return v
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
	s.form = &filling{
		what: what,
		text: map[*forms.Field]*mvvm.Observable[string]{},
		tick: map[*forms.Field]*mvvm.Observable[bool]{},
		pick: map[*forms.Field]*mvvm.Observable[int]{},
	}
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
		v := f.textOf(s, field)
		entry := toolkit.NewEntry(v.Get())
		entry.Placeholder = placeholderFor(field)
		tkbind.BindEntry(v, entry, s.repaint)
		return toolkit.NewFormField(label, entry)

	case forms.Checkbox, forms.Radio:
		v := f.tickOf(s, field)
		box := toolkit.NewCheckButton(buttonLabel(field), v.Get())
		tkbind.BindCheck(v, box, s.repaint)
		return toolkit.NewFormField(label, box)

	case forms.ComboBox, forms.ListBox:
		options := make([]string, 0, len(field.Options))
		for _, o := range field.Options {
			options = append(options, o.Label)
		}
		if len(options) == 0 {
			return nil
		}
		v := f.pickOf(s, field)
		drop := toolkit.NewDropDown(options, v.Get())
		tkbind.BindChoice(v, drop, s.repaint)
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
