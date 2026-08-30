// The tool panel: the verbs that do not fit on a strip, and the controls each
// of them needs.
//
// The strip held nine things. The library behind it holds about twenty-five,
// and most of them need to be told something before they can run — which pages,
// how many to a sheet, what to write, which password. A strip of buttons has
// nowhere to put any of that, and a thirtieth button would not fit on it
// anyway: the nine already reached two thirds of the way across.
//
// So the verbs are grouped, and a group opens a panel beside the page rather
// than instead of it. Beside, because every verb here changes the document and
// the document is redrawn from what would be saved: setting a crop box and
// watching the page come back cropped is the whole point, and a panel that
// covered the page would hide the one thing worth looking at.

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdfkit/ops"
	"github.com/go-widgets/toolkit"
)

// panelW is how wide the tool panel is, and gap the space between it and the
// page beside it.
const (
	panelW = 300
	gap    = toolkit.DefaultBoxSpacing
)

// The groups, in the order they appear on the strip. A group is named by what
// it does to the file rather than by which library call it makes: somebody
// looking for "two to a sheet" is thinking about the sheet, not about NUp.
const (
	groupPages   = "Pages"
	groupSheet   = "Sheet"
	groupMarks   = "Marks"
	groupFile    = "File"
	groupProtect = "Protect"
	groupRead    = "Read"
)

// groupNames is the order they are offered in.
var groupNames = []string{groupPages, groupSheet, groupMarks, groupFile, groupProtect, groupRead}

// tools is the tool panel: which group is open, the widgets of every group
// that has been opened, and what those widgets currently say.
//
// The widgets are built once per group and kept, because they hold what
// somebody has typed: rebuilding them on every change would empty the boxes
// under their hands. What they say is kept here rather than read out of them,
// through a subscription made when each one is built — so a value arrives when
// it changes rather than being copied across every frame.
type tools struct {
	open  string
	built map[string]toolkit.Widget

	// The Pages group.
	spec   string // which pages, empty for the one on the screen
	turn   int    // a quarter turn, in degrees
	moveTo int    // where the page on the screen is to go
	box    string // a crop box, as four numbers
	before int    // where a blank page goes
	every  int    // how many pages a split file holds

	// The Sheet group.
	up int // how many pages to a sheet

	// The Marks group, which acts on a range of its own: the Pages group's
	// box is that group's, and one box shared between two panels would show
	// the wrong thing in whichever of them was not last used.
	markSpec string
	mark     string // what a watermark says
	numbers  string // the shape of a page number
	prefix   string // what comes before a Bates number
	start    int    // the first Bates number
	digits   int    // how many digits it is padded to
	stamp    string // what a stamp says
	at       int    // where on the page it goes
	size     int    // how large, in points

	// The File group.
	title, author string

	// The Protect group.
	openPw, userPw, ownerPw string
	allow                   map[string]bool

	// The Read group: which reading of the document is on the screen instead
	// of the picture of it, or empty for the picture, and which of the
	// writable picture formats a page is handed over in.
	reading string
	picture int
}

// newTools builds the panel's state with the defaults each control starts at.
func newTools() *tools {
	return &tools{
		built:   map[string]toolkit.Widget{},
		turn:    90,
		moveTo:  1,
		before:  1,
		every:   1,
		up:      2,
		mark:    "DRAFT",
		numbers: "{page} / {pages}",
		start:   1,
		digits:  6,
		stamp:   "COPY",
		size:    12,
		allow:   everythingAllowed(),
	}
}

// everythingAllowed is what a protected file lets a reader do until somebody
// says otherwise, which is everything: a password on a file is nearly always
// meant to keep it shut rather than to stop whoever opened it printing it.
func everythingAllowed() map[string]bool {
	out := map[string]bool{}
	for _, a := range allowed {
		out[a.name] = true
	}
	return out
}

// showGroup opens a group of verbs beside the page, or closes it when it is
// the one already open.
func (s *state) showGroup(name string) {
	if s.tools.open == name {
		s.tools.open = ""
	} else {
		s.tools.open = name
		s.showingForm = false
	}
	s.settle()
	s.note = ""
	s.refresh()
}

// body is the panel of the group now open, built the first time it is asked
// for and kept afterwards.
func (s *state) body() toolkit.Widget {
	if w, ok := s.tools.built[s.tools.open]; ok {
		return w
	}
	var rows *column
	switch s.tools.open {
	case groupPages:
		rows = s.pagesGroup()
	case groupSheet:
		rows = s.sheetGroup()
	case groupMarks:
		rows = s.marksGroup()
	case groupProtect:
		rows = s.protectGroup()
	case groupRead:
		rows = s.readGroup()
	default:
		rows = s.fileGroup()
	}
	w := rows.scroller()
	s.tools.built[s.tools.open] = w
	return w
}

// A column is a stack of panel rows that remembers how tall it has grown.
//
// It has to: a ScrollView keeps its child's own width and height and scrolls
// the painting rather than the child, so a child nobody ever gave a size to is
// drawn as nothing at all — an empty panel with a scrollbar down the side of
// it, which is what this looked like the first time it was rendered.
type column struct {
	box *toolkit.VBox
	h   int
}

// Room around and between the rows, and the width they are laid out at: the
// panel less its frame and the scrollbar down its edge.
const (
	rowGap    = 6
	rowsW     = panelW - 34
	labelledH = 52
	bareH     = 30
)

// newColumn starts an empty stack.
func newColumn() *column {
	box := toolkit.NewVBox()
	box.Spacing = rowGap
	return &column{box: box}
}

// add puts a row at the bottom and counts what it cost.
func (c *column) add(w toolkit.Widget, h int) {
	c.box.AddFixed(w, h)
	if c.h > 0 {
		c.h += rowGap
	}
	c.h += h
}

// scroller is the stack in a view that can be scrolled when it is taller than
// the panel, told how tall it is.
func (c *column) scroller() *toolkit.ScrollView { return c.scrollerOf(rowsW) }

// scrollerOf is the same at a width of the caller's choosing, which is what a
// reading of the page needs: it is put where the page was rather than in the
// panel, and the page is a good deal wider than the panel is.
func (c *column) scrollerOf(w int) *toolkit.ScrollView {
	c.box.SetBounds(toolkit.Rect{W: w, H: c.h})
	sv := toolkit.NewScrollView(c.box)
	sv.SetContentSize(w, c.h)
	return sv
}

// pagesGroup is everything that changes which pages there are and what order
// they come in.
func (s *state) pagesGroup() *column {
	box := newColumn()
	box.add(s.entryRow("Which pages", "1-3,7 — empty means this one", "",
		func(v string) { s.tools.spec = v }), labelledH)
	box.add(buttons(
		button("Keep only these", toolkit.ButtonDefault, s.selectPages),
		button("Delete these", toolkit.ButtonDanger, s.deleteRange),
	), bareH)

	turns := toolkit.NewCycleButton("a quarter", "a half", "three quarters")
	turns.Index().Subscribe(func(i int) { s.tools.turn = 90 * (i + 1) })
	box.add(buttons(turns, button("Turn them", toolkit.ButtonDefault, s.turnRange)), bareH)
	box.add(button("Reverse the order", toolkit.ButtonDefault, s.reverse), bareH)

	box.add(s.spinRow("Move this page to", 1, s.tools.moveTo,
		func(v int) { s.tools.moveTo = v }), labelledH)
	box.add(button("Move it there", toolkit.ButtonDefault, s.movePage), bareH)

	box.add(s.entryRow("Crop to, in points", "x0,y0,x1,y1", "",
		func(v string) { s.tools.box = v }), labelledH)
	box.add(button("Crop them", toolkit.ButtonDefault, s.crop), bareH)

	box.add(s.spinRow("Put a blank page before", 1, s.tools.before,
		func(v int) { s.tools.before = v }), labelledH)
	box.add(button("Insert it", toolkit.ButtonDefault, s.insertBlank), bareH)

	box.add(s.spinRow("Split into files of", 1, s.tools.every,
		func(v int) { s.tools.every = v }), labelledH)
	box.add(button("Split and hand them over", toolkit.ButtonProminent, s.split), bareH)

	// Last, and not among the things that take a range: this one asks the
	// document which pages it means rather than being told.
	box.add(blankButton(s), bareH)
	return box
}

// sheetGroup is everything that puts more than one page's worth on a sheet, or
// more than one file into this one.
func (s *state) sheetGroup() *column {
	box := newColumn()
	box.add(s.spinRow("Pages to a sheet", 1, s.tools.up, func(v int) { s.tools.up = v }), labelledH)
	box.add(button("Lay them out", toolkit.ButtonDefault, s.nUp), bareH)
	box.add(button("Fold it into a booklet", toolkit.ButtonDefault, s.booklet), bareH)
	box.add(button("Add a file after this one", toolkit.ButtonDefault, s.merge), bareH)
	box.add(button("Lay a file over this one", toolkit.ButtonDefault, s.overlay), bareH)
	return box
}

// entryRow is a named box to type in, bound to where what is typed goes.
func (s *state) entryRow(label, hint, initial string, to func(string)) toolkit.Widget {
	e := toolkit.NewEntry(initial)
	e.Placeholder = hint
	e.Text().Subscribe(to)
	s.typing = append(s.typing, e)
	return toolkit.NewFormField(label, e)
}

// spinRow is a named number, bound to where the number goes.
func (s *state) spinRow(label string, min, initial int, to func(int)) toolkit.Widget {
	sp := toolkit.NewSpinButton(min, pageCeiling, initial, 1)
	sp.Value().Subscribe(to)
	return toolkit.NewFormField(label, sp)
}

// chooseRow is a named list to choose from, bound to where the choice goes.
// The list is drawn over whatever is under it by the popover host the view is
// wrapped in, which is the one thing a drop-down cannot do for itself.
func chooseRow(label string, options []string, chosen int, to func(int)) toolkit.Widget {
	d := toolkit.NewDropDown(options, chosen)
	d.Selected().Subscribe(to)
	return toolkit.NewFormField(label, d)
}

// tickRow is a box to tick, which names itself.
func tickRow(label string, on bool, to func(bool)) toolkit.Widget {
	c := toolkit.NewCheckButton(label, on)
	c.Checked().Subscribe(to)
	return c
}

// pageCeiling is as high as any of these numbers is allowed to go. It is not a
// page count: the document changes under the control, and a number that is too
// large is refused by the operation itself, which is the one place that knows.
const pageCeiling = 9999

// button is one control of the panel or the strip.
func button(label string, style toolkit.ButtonStyle, on func()) *toolkit.Button {
	b := toolkit.NewButton(label, on)
	b.Style = style
	return b
}

// buttons puts controls side by side on one row, sharing the width.
func buttons(ws ...toolkit.Widget) toolkit.Widget {
	row := toolkit.NewHBox()
	for _, w := range ws {
		row.AddFlex(w, 1)
	}
	return row
}

// settle takes the focus out of every box, which is what has to happen when a
// panel is put away or another one takes its place: a box nobody can see any
// more must not go on taking the arrow keys that turn the pages.
func (s *state) settle() {
	for _, e := range s.typing {
		e.SetFocused(false)
	}
}

// where is the range the Pages group acts on: what was typed, or the page on
// the screen when nothing was.
func (s *state) where() string {
	if strings.TrimSpace(s.tools.spec) == "" {
		return pageSpec(s.at)
	}
	return s.tools.spec
}

// selectPages keeps the pages the range names and drops the rest.
func (s *state) selectPages() {
	spec := s.where()
	s.changeSaying("kept "+spec, func(d *ops.Doc) error { return d.Select(spec) })
}

// deleteRange drops the pages the range names, unless that would be all of
// them: a document needs a page, and one with none cannot be shown, saved or
// opened again.
func (s *state) deleteRange() {
	spec := s.where()
	if s.doc != nil && emptied(s.doc, spec) {
		s.fail("that would delete every page, and a document needs one")
		return
	}
	s.changeSaying("deleted "+spec, func(d *ops.Doc) error { return d.Delete(spec) })
}

// emptied reports whether deleting the pages a range names would leave none. A
// range that cannot be read at all is not this function's to complain about:
// the operation itself says what is wrong with it, in its own words.
func emptied(d *ops.Doc, spec string) bool {
	nums, err := ops.ParseRange(spec, d.PageCount())
	if err != nil {
		return false
	}
	left := d.PageCount()
	gone := map[int]bool{}
	for _, n := range nums {
		if !gone[n] {
			gone[n] = true
			left--
		}
	}
	return left == 0
}

// turnRange turns the pages the range names.
func (s *state) turnRange() {
	spec, by := s.where(), s.tools.turn
	s.changeSaying(fmt.Sprintf("turned %s by %d degrees", spec, by),
		func(d *ops.Doc) error { return d.Rotate(spec, by) })
}

// reverse puts the pages in the opposite order.
func (s *state) reverse() {
	s.changeSaying("reversed", func(d *ops.Doc) error {
		d.Reverse()
		return nil
	})
}

// movePage takes the page on the screen somewhere else in the order, and
// follows it there.
func (s *state) movePage() {
	from, to := s.at, s.tools.moveTo
	if !s.changeSaying(fmt.Sprintf("moved page %d to %d", from, to),
		func(d *ops.Doc) error { return d.Move(from, to) }) {
		return
	}
	// Follow the page: somebody who moved the one they were looking at is
	// still looking at it.
	s.at = to
	s.refresh()
}

// crop cuts the pages the range names down to a box.
func (s *state) crop() {
	box, err := parseBox(s.tools.box)
	if err != nil {
		s.fail(err.Error())
		return
	}
	spec := s.where()
	s.changeSaying("cropped "+spec, func(d *ops.Doc) error { return d.Crop(spec, box) })
}

// parseBox reads a crop box written as four numbers.
func parseBox(spec string) ([4]float64, error) {
	var box [4]float64
	parts := strings.Split(strings.TrimSpace(spec), ",")
	if len(parts) != 4 {
		return box, fmt.Errorf("a crop box is four numbers: x0,y0,x1,y1")
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return box, fmt.Errorf("%q is not a number", strings.TrimSpace(p))
		}
		box[i] = v
	}
	return box, nil
}

// insertBlank puts an empty page of the same size before another one.
func (s *state) insertBlank() {
	at := s.tools.before
	s.changeSaying(fmt.Sprintf("a blank page before %d", at),
		func(d *ops.Doc) error { return d.InsertBlank(at) })
}

// split hands over one file per piece. Nothing is changed here: what was open
// stays open, and the pieces are copies of it.
func (s *state) split() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	parts, err := s.doc.Split(s.tools.every)
	if err != nil {
		s.fail(err.Error())
		return
	}
	for i, part := range parts {
		out, perr := docBytes(part)
		if perr != nil {
			s.fail("part " + strconv.Itoa(i+1) + " cannot be written: " + perr.Error())
			return
		}
		s.host.Save(partName(s.name, i+1), out)
	}
	s.note = fmt.Sprintf("handed over %d files — a browser may ask before it takes more than one", len(parts))
	s.refresh()
}

// partName is what one piece of a split document is offered under.
func partName(name string, n int) string {
	return fmt.Sprintf("%s-%03d.pdf", strings.TrimSuffix(saveName(name), "-edited.pdf"), n)
}

// nUp lays several pages on one sheet.
func (s *state) nUp() {
	n := s.tools.up
	if !s.changeSaying(fmt.Sprintf("%d to a sheet", n), func(d *ops.Doc) error { return d.NUp(n) }) {
		return
	}
	s.at = 1
	s.refresh()
}

// booklet lays the pages out so that the sheets fold into a booklet.
func (s *state) booklet() {
	if !s.changeSaying("folded", func(d *ops.Doc) error { return d.Booklet() }) {
		return
	}
	s.at = 1
	s.refresh()
}

// merge adds another file's pages after this one's.
func (s *state) merge() {
	s.withAnother("added", func(d, other *ops.Doc) error { d.Append(other); return nil })
}

// overlay draws another file's pages on top of this one's.
func (s *state) overlay() {
	s.withAnother("laid over", func(d, other *ops.Doc) error { return d.Overlay(other) })
}

// withAnother asks for a second file and joins it to the one already open.
// The picker is asked for the same way Open asks: the press that reaches it is
// the one somebody made, which is what a browser requires before it will show
// a file chooser at all.
func (s *state) withAnother(said string, join func(d, other *ops.Doc) error) {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	s.host.Open(func(name string, data []byte) {
		other, err := ops.Open(data)
		if err != nil {
			s.fail("cannot open " + name + ": " + err.Error())
			return
		}
		s.changeSaying(said+" "+name, func(d *ops.Doc) error { return join(d, other) })
	})
}
