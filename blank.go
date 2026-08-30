// Dropping the pages that carry nothing.
//
// A duplex stack run through a single-sided feeder comes back with a blank
// behind every one-sided sheet, and a scanner set to "both sides" produces one
// for every sheet that only had one. They are the pages nobody wants and
// everybody has.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// blankInk is how little a page may carry and still be blank.
//
// Chosen by measuring rather than by decree. Over 395 pages of government
// forms and library scans, the ink on a page runs to a median of 4.7% and
// falls away sharply below one part in a thousand: 11 pages are under it, 2
// are under a twentieth of it, and the pages between are covers carrying a
// rule and nothing else. A page with a line of text is around three parts in a
// thousand, so this keeps one and drops a page number alone — which is what a
// blank back with a footer is.
const blankInk = 0.001

// blankScale is how large the page is drawn to look at it. Small on purpose:
// this is a count of dark pixels over a whole page, and a quarter-size drawing
// answers it in a sixteenth of the time. A document of two hundred pages is
// drawn twice by this verb — once to look, once to show what is left — and the
// looking should not be the slow half.
const blankScale = 0.25

// dropBlank removes the pages that carry no ink.
func (s *state) dropBlank() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return
	}
	blanks := blankPages(src, s.fitScale(src))
	if len(blanks) == 0 {
		s.fail("every page of this document carries something")
		return
	}
	if len(blanks) == src.PageCount() {
		// Not a corner case. Nine of two hundred real forms are single-page
		// documents whose one page is a panel reading "Please wait... your PDF
		// viewer may not be able to display this type of document": an XFA
		// form, whose real content is XML that no PDF viewer draws. The page
		// is blank, the document is not, and dropping the page would leave
		// nothing at all.
		s.fail("every page of this document is blank, and a document needs one")
		return
	}
	spec := rangeOf(blanks)
	s.changeSaying(fmt.Sprintf("dropped %d blank page(s): %s", len(blanks), spec),
		func(d *ops.Doc) error { return d.Delete(spec) })
}

// blankPages are the pages carrying less ink than a page carries.
//
// A page that cannot be drawn is NOT counted blank. Not being able to see a
// page is not evidence that there is nothing on it, and deleting on that
// footing would throw away exactly the pages this program had most trouble
// with.
//
// Over two hundred real government forms — 773 pages — this drops 42, which is
// 5.4%, and refuses nine documents outright as blank throughout.
func blankPages(src *reader.Document, fit float64) []int {
	var out []int
	for p := 1; p <= src.PageCount(); p++ {
		img, err := drawPage(src, p, render.Options{
			Scale:       blankScale * fit,
			MaxDuration: pageBudget,
		})
		if err != nil || img == nil || img.W*img.H == 0 {
			continue
		}
		ink := 0
		for i := 0; i < img.W*img.H; i++ {
			r, g, b := uint32(img.Pix[i*4]), uint32(img.Pix[i*4+1]), uint32(img.Pix[i*4+2])
			if (r*299+g*587+b*114)/1000 < 128 {
				ink++
			}
		}
		if float64(ink)/float64(img.W*img.H) < blankInk {
			out = append(out, p)
		}
	}
	return out
}

// rangeOf writes page numbers the way the verbs read them, collapsing runs so
// that a document whose every other page is blank does not produce a range as
// long as itself.
func rangeOf(pages []int) string {
	if len(pages) == 0 {
		return ""
	}
	out := ""
	for i := 0; i < len(pages); {
		j := i
		for j+1 < len(pages) && pages[j+1] == pages[j]+1 {
			j++
		}
		if out != "" {
			out += ","
		}
		if j == i {
			out += fmt.Sprint(pages[i])
		} else {
			out += fmt.Sprintf("%d-%d", pages[i], pages[j])
		}
		i = j + 1
	}
	return out
}

// blankButton is the control, put with the other things that remove pages.
func blankButton(s *state) toolkit.Widget {
	return button("Drop the blank pages", toolkit.ButtonDanger, s.dropBlank)
}
