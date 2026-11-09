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

// marksGroup is the panel beside the marks written across every page by a rule.
func (s *state) marksGroup() *column {
	box := newColumn()
	box.add(s.whichPages(), labelledH())

	box.add(s.entryRow("Watermark", "what it says", s.tools.mark), labelledH())
	box.add(s.entryRow("Page numbers", "{page} and {pages} are filled in", s.tools.numbers), labelledH())

	box.add(s.entryRow("Bates prefix", "what comes before the number", s.tools.prefix), labelledH())
	box.add(buttons(
		s.spinRow("Starting at", 1, s.tools.start),
		s.spinRow("Padded to", 1, s.tools.digits),
	), labelledH())
	return box
}

// stampGroup is the panel beside the marks put at a place of the reader's
// choosing: a stamp, a header and a footer, a picture.
func (s *state) stampGroup() *column {
	box := newColumn()
	box.add(s.whichPages(), labelledH())

	// ⛔ THE HINT NAMES THE TOKENS, because the stamp has always filled them in
	// and nothing said so. ops.expand runs over EVERY stamp's text, not only a
	// page number's — so a stamp at `top centre` reading "Report — {page} of
	// {pages}" is a header with a page number in it, which is the whole of what
	// a "header and footer" tool does. The capability was there and
	// undiscoverable, which for a reader is the same as absent.
	box.add(s.entryRow("Stamp", "what it says — {page} and {pages} too", s.tools.stamp), labelledH())
	box.add(buttons(
		s.chooseRow("Where it goes", placeNames(), s.tools.at),
		s.spinRow("Points", 4, s.tools.size),
	), labelledH())

	// The header and the footer have boxes of their own rather than sharing the
	// stamp's: they are written together, and a reader who has typed a header
	// should not lose it by stamping something else. The size above governs all
	// three, because it is the same question about the same page.
	box.add(s.entryRow("Header", "the line at the top — {page} too", s.tools.header), labelledH())
	box.add(s.entryRow("Footer", "the line at the foot", s.tools.footer), labelledH())

	// `Where it goes` above is shared with the picture on purpose: it is the
	// same question about the same page, and two choosers holding the same nine
	// answers would be two things to keep in step for a reader who thinks of it
	// as one. The width is the picture's own, so it has a box of its own.
	box.add(s.spinRow("Picture width", 12, s.tools.wide), labelledH())
	return box
}

// whichPages is the range both panels act on, shown on both and bound to ONE
// datum.
//
// ⛔ ONE DATUM AND TWO BOXES, which is the opposite of the Pages panel's rule
// and right for the same reason. The Pages box asks a different question from
// this one — which pages to KEEP, against which pages to MARK — so they are
// two values. Marks and Stamp ask the same question, split across two tabs for
// room; a reader who typed 1-3 before stamping must not find it gone after
// choosing the other tab, and the two-way binding keeps both boxes showing it.
func (s *state) whichPages() toolkit.Widget {
	return s.entryRow("Which pages", "1-3,7 — empty means all of them", s.tools.markSpec)
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

// headFoot puts a line at the top of the pages and a line at the foot, in one
// change.
//
// ⛔ ONE PRESS FOR BOTH, which is the whole of it: `Stamp them` could already
// write either — ops.Stamp takes nine positions and fills {page} in for any of
// them — so a reader who wanted a header AND a footer had to stamp twice,
// changing the box and the chooser in between, and the second press is the one
// people forget. The verb exists because the gesture is one gesture.
//
// ⛔ A text left empty is not stamped. ops refuses a stamp with no text, and
// the useful reading of an empty footer box is "no footer", not an error.
func (s *state) headFoot() {
	spec := s.marked()
	head, foot := s.tools.header.Get(), s.tools.footer.Get()
	size := float64(s.tools.size.Get())
	if strings.TrimSpace(head) == "" && strings.TrimSpace(foot) == "" {
		s.fail("type a header or a footer first — {page} and {pages} are filled in")
		return
	}
	said := "put a header and a footer on " + spec
	switch {
	case strings.TrimSpace(foot) == "":
		said = "put a header on " + spec
	case strings.TrimSpace(head) == "":
		said = "put a footer on " + spec
	}
	s.changeSaying(said, func(d *ops.Doc) error {
		for _, m := range []struct {
			text  string
			where ops.Position
		}{{head, ops.TopCenter}, {foot, ops.BottomCenter}} {
			if strings.TrimSpace(m.text) == "" {
				continue
			}
			if err := d.Stamp(spec, ops.Stamp{
				Text:     m.text,
				Font:     ops.Helvetica,
				Size:     size,
				Position: m.where,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// placePicture draws a picture at a place on the pages: a signature on a
// contract, a logo on a letterhead, a seal on a certificate.
//
// ⛔ ops.Place HAS BEEN THERE AND THE WORKBENCH NEVER CALLED IT. Its own
// documentation says what it is for — "a signature, a logo, a stamp that is an
// image rather than a line of text" — and it is the "add images" half of the
// one row the BentoPDF reading still calls `partly` for the editor. Nothing
// here is new work; it is a control over a verb that was finished.
//
// ⛔ The picture is sized by WIDTH and not by its own pixels. Place will size
// it at one point per pixel when asked for neither, which is right for a scan
// that IS the page and wrong for a logo: a 1000-pixel badge would be laid down
// a metre wide. Giving the width alone keeps the picture's proportions, which
// is Place's own promise for that case.
func (s *state) placePicture() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	spec := s.marked()
	at := s.tools.at.Get()
	if at < 0 || at >= len(places) {
		at = 0
	}
	where, wide := places[at], float64(s.tools.wide.Get())
	s.host.Open(func(name string, data []byte) {
		s.changeSaying(fmt.Sprintf("drew %s on %s, %s", name, spec, where.name),
			func(d *ops.Doc) error {
				return d.Place(spec, ops.Placement{
					Data:     data,
					Position: where.where,
					Width:    wide,
				})
			})
	})
}
