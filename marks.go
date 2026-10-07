// The Marks group: what gets written on top of what the pages already show.
//
// A watermark, a page number, a Bates number and a stamp are one verb in the
// library — text drawn on a page — offered here as the four things people
// actually ask for, because "stamp this page with {n} padded to six digits" is
// not what anybody says when they mean a Bates number.

package main

import (
	"fmt"
	"strings"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// places is where a stamp can sit, in the words somebody would use, and in the
// order the library numbers them.
var places = []struct {
	name  string
	where ops.Position
}{
	{"in the middle", ops.Center},
	{"top left", ops.TopLeft},
	{"top centre", ops.TopCenter},
	{"top right", ops.TopRight},
	{"bottom left", ops.BottomLeft},
	{"bottom centre", ops.BottomCenter},
	{"bottom right", ops.BottomRight},
	{"middle left", ops.MiddleLeft},
	{"middle right", ops.MiddleRight},
}

// placeNames is the list a person chooses from.
func placeNames() []string {
	out := make([]string, 0, len(places))
	for _, p := range places {
		out = append(out, p.name)
	}
	return out
}

// marksGroup is the panel.
func (s *state) marksGroup() *column {
	box := newColumn()
	box.add(s.entryRow("Which pages", "1-3,7 — empty means all of them", s.tools.markSpec), labelledH())

	box.add(s.entryRow("Watermark", "what it says", s.tools.mark), labelledH())
	box.add(button("Write it across them", toolkit.ButtonDefault, s.watermark), bareH())

	box.add(s.entryRow("Page numbers", "{page} and {pages} are filled in", s.tools.numbers), labelledH())
	box.add(button("Number them", toolkit.ButtonDefault, s.number), bareH())

	box.add(s.entryRow("Bates prefix", "what comes before the number", s.tools.prefix), labelledH())
	box.add(buttons(
		s.spinRow("Starting at", 1, s.tools.start),
		s.spinRow("Padded to", 1, s.tools.digits),
	), labelledH())
	box.add(button("Stamp the numbers on", toolkit.ButtonDefault, s.bates), bareH())

	box.add(s.entryRow("Stamp", "what it says", s.tools.stamp), labelledH())
	box.add(buttons(
		s.chooseRow("Where it goes", placeNames(), s.tools.at),
		s.spinRow("Points", 4, s.tools.size),
	), labelledH())
	box.add(button("Stamp them", toolkit.ButtonDefault, s.stamp), bareH())
	return box
}

// marked is the range the Marks group acts on: what was typed, or every page
// when nothing was. Every page rather than the one on the screen, because a
// watermark or a page number is nearly always wanted on all of them.
func (s *state) marked() string {
	if strings.TrimSpace(s.tools.markSpec.Get()) == "" {
		return "all"
	}
	return s.tools.markSpec.Get()
}

// watermark writes across the pages, at an angle and faintly, which is what
// the library's own watermark does.
func (s *state) watermark() {
	spec, text := s.marked(), s.tools.mark.Get()
	s.changeSaying("wrote "+text+" across "+spec,
		func(d *ops.Doc) error { return d.Watermark(spec, text) })
}

// number writes a page number at the foot of each page.
func (s *state) number() {
	spec, format := s.marked(), s.tools.numbers.Get()
	s.changeSaying("numbered "+spec,
		func(d *ops.Doc) error { return d.PageNumbers(spec, format) })
}

// bates stamps a running number, which is how a set of documents is given one
// identifier per page that nothing else in it shares.
func (s *state) bates() {
	spec := s.marked()
	prefix, start, digits := s.tools.prefix.Get(), s.tools.start.Get(), s.tools.digits.Get()
	s.changeSaying(fmt.Sprintf("stamped %s from %s%0*d", spec, prefix, digits, start),
		func(d *ops.Doc) error { return d.Bates(spec, prefix, start, digits) })
}

// stamp draws a line of text where it was asked for.
func (s *state) stamp() {
	spec := s.marked()
	at := s.tools.at.Get()
	if at < 0 || at >= len(places) {
		at = 0
	}
	mark := ops.Stamp{
		Text:     s.tools.stamp.Get(),
		Font:     ops.Helvetica,
		Size:     float64(s.tools.size.Get()),
		Position: places[at].where,
	}
	s.changeSaying(fmt.Sprintf("stamped %s %s", spec, places[at].name),
		func(d *ops.Doc) error { return d.Stamp(spec, mark) })
}
