// The paper the pages are on, and the colour their text is in. Two more verbs
// `ops` already carried and the ribbon did not.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
)

// papers are the sheet sizes a person asks for by name, in points.
//
// ⛔ IN POINTS, AND THE NUMBERS ARE THE STANDARD'S. A4 is 595.276 × 841.89
// points — 210 × 297 mm at 72 points to the inch — and rounding it to 595 × 842
// is a tenth of a millimetre of error that shows up as a hairline when the page
// is printed against a trimmed edge. ISO 216 for the A series, ANSI/ASME Y14.1
// for Letter and Legal.
var papers = []struct {
	name string
	w, h float64
}{
	{"A4 — 210 × 297 mm", 595.276, 841.89},
	{"A3 — 297 × 420 mm", 841.89, 1190.551},
	{"A5 — 148 × 210 mm", 419.528, 595.276},
	{"Letter — 8.5 × 11 in", 612, 792},
	{"Legal — 8.5 × 14 in", 612, 1008},
	{"Tabloid — 11 × 17 in", 792, 1224},
}

// paperNames is the list a person chooses from.
func paperNames() []string {
	out := make([]string, 0, len(papers))
	for _, p := range papers {
		out = append(out, p.name)
	}
	return out
}

// inks are the colours the text of a page can be painted, by name.
//
// ⛔ A NAMED LIST RATHER THAN A COLOUR WHEEL, for now and on purpose: the verb
// underneath takes three numbers from zero to one, and a wheel is a widget with
// its own state, its own binding and its own tests. Six names reach the verb
// today; the wheel can replace the list without the verb changing.
var inks = []struct {
	name    string
	r, g, b float64
}{
	{"black", 0, 0, 0},
	{"dark grey", 0.35, 0.35, 0.35},
	{"red", 0.8, 0.1, 0.1},
	{"blue", 0.1, 0.2, 0.75},
	{"green", 0.05, 0.45, 0.15},
	{"white", 1, 1, 1},
}

// inkNames is the list a person chooses from.
func inkNames() []string {
	out := make([]string, 0, len(inks))
	for _, i := range inks {
		out = append(out, i.name)
	}
	return out
}

// setPaper sets the paper of the pages the range names.
//
// ⛔ Not `resize`: that name is taken, by the verb that resizes the WINDOW.
// Two different things called the same thing in one type is how a surface
// resize ends up setting A4.
func (s *state) setPaper() {
	at := s.tools.paper.Get()
	if at < 0 || at >= len(papers) {
		at = 0
	}
	p := papers[at]
	spec := s.where()
	s.changeSaying(fmt.Sprintf("set %s to %s", spec, p.name), func(d *ops.Doc) error {
		return d.Resize(spec, [4]float64{0, 0, p.w, p.h})
	})
}

// recolourText paints the text of the pages the range names in one colour.
//
// ⛔⛔ AND IT SAYS WHAT IT COULD NOT REACH. ops.Doc.RecolourText only reaches
// the text in each PAGE'S OWN content stream: text drawn through a Form XObject
// is shared with every page that draws it, so recolouring one would change
// pages the caller did not name. The library counts what it stepped over and
// UnreachedText reports it — and passing that count on is the whole reason the
// library bothers to keep it. A document whose text is all inside XObjects
// would otherwise come back looking unchanged with nothing said, which is the
// one outcome worse than refusing.
func (s *state) recolourText() {
	at := s.tools.ink.Get()
	if at < 0 || at >= len(inks) {
		at = 0
	}
	ink := inks[at]
	spec := s.where()
	if !s.changeSaying(fmt.Sprintf("painted the text of %s %s", spec, ink.name),
		func(d *ops.Doc) error { return d.RecolourText(spec, ink.r, ink.g, ink.b) }) {
		return
	}
	if left := s.doc.UnreachedText(); left > 0 {
		s.note += fmt.Sprintf(" — %d text operation(s) were left as they were, "+
			"being inside a form shared with other pages", left)
		s.refresh()
	}
}
