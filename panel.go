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
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/mvvm/tkbind"
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

	// ⛔ Every one of these is an Observable, and the control that shows it
	// holds THE SAME ONE -- two-way, through mvvm/tkbind. They used to be
	// plain fields, and what that looked like was
	//
	//	e.Text().Subscribe(func(v string) { s.tools.spec = v })
	//
	// in twenty-seven places: a copy OUT of the widget into a field, and no
	// way back in except rebuilding the widget. That is a datum crossing a
	// boundary by being copied, which is the thing MVVM is for not having, and
	// the direction that is missing is the one nobody notices -- it works
	// until something other than the control changes the model, and then the
	// panel shows the old number while the document has the new one.
	//
	// movePage is where that bit: it follows the page it moved, so s.at
	// changes underneath the "Move this page to" spinner.

	// The Pages group.
	spec   *mvvm.Observable[string] // which pages, empty for the one on the screen
	turn   *mvvm.Observable[int]    // a quarter turn, in degrees
	moveTo *mvvm.Observable[int]    // where the page on the screen is to go
	box    *mvvm.Observable[string] // a crop box, as four numbers
	before *mvvm.Observable[int]    // where a blank page goes
	every  *mvvm.Observable[int]    // how many pages a split file holds

	// The Sheet group.
	up *mvvm.Observable[int] // how many pages to a sheet

	// The Marks group, which acts on a range of its own: the Pages group's
	// box is that group's, and one box shared between two panels would show
	// the wrong thing in whichever of them was not last used.
	markSpec *mvvm.Observable[string]
	mark     *mvvm.Observable[string] // what a watermark says
	numbers  *mvvm.Observable[string] // the shape of a page number
	prefix   *mvvm.Observable[string] // what comes before a Bates number
	start    *mvvm.Observable[int]    // the first Bates number
	digits   *mvvm.Observable[int]    // how many digits it is padded to
	stamp    *mvvm.Observable[string] // what a stamp says
	at       *mvvm.Observable[int]    // where on the page it goes
	size     *mvvm.Observable[int]    // how large, in points

	// The File group.
	title, author *mvvm.Observable[string]

	// The Protect group.
	openPw, userPw, ownerPw *mvvm.Observable[string]
	// allow is one Observable PER PERMISSION rather than an Observable of a
	// map: a map is not comparable, so one Observable over the whole of it
	// could not tell a change from a re-set, and every tick would repaint
	// every box.
	allow map[string]*mvvm.Observable[bool]

	// The Read group: which reading of the document is on the screen instead
	// of the picture of it, or empty for the picture, and which of the
	// writable picture formats a page is handed over in.
	reading *mvvm.Observable[string]
	picture *mvvm.Observable[int]
}

// newTools builds the panel's state with the defaults each control starts at.
func newTools() *tools {
	return &tools{
		built:    map[string]toolkit.Widget{},
		spec:     mvvm.NewObservable(""),
		turn:     mvvm.NewObservable(90),
		moveTo:   mvvm.NewObservable(1),
		box:      mvvm.NewObservable(""),
		before:   mvvm.NewObservable(1),
		every:    mvvm.NewObservable(1),
		up:       mvvm.NewObservable(2),
		markSpec: mvvm.NewObservable(""),
		mark:     mvvm.NewObservable("DRAFT"),
		numbers:  mvvm.NewObservable("{page} / {pages}"),
		prefix:   mvvm.NewObservable(""),
		start:    mvvm.NewObservable(1),
		digits:   mvvm.NewObservable(6),
		stamp:    mvvm.NewObservable("COPY"),
		at:       mvvm.NewObservable(0),
		size:     mvvm.NewObservable(12),
		title:    mvvm.NewObservable(""),
		author:   mvvm.NewObservable(""),
		openPw:   mvvm.NewObservable(""),
		userPw:   mvvm.NewObservable(""),
		ownerPw:  mvvm.NewObservable(""),
		allow:    everythingAllowed(),
		reading:  mvvm.NewObservable(""),
		picture:  mvvm.NewObservable(0),
	}
}

// everythingAllowed is what a protected file lets a reader do until somebody
// says otherwise, which is everything: a password on a file is nearly always
// meant to keep it shut rather than to stop whoever opened it printing it.
func everythingAllowed() map[string]*mvvm.Observable[bool] {
	out := map[string]*mvvm.Observable[bool]{}
	for _, a := range allowed {
		out[a.name] = mvvm.NewObservable(true)
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
	rowGap = 6
	rowsW  = panelW - 34

	// What a row costs besides its text. See the band geometry in scene.go.
	//
	// 15 rather than 16 because the face changed and its line is a pixel
	// taller: a control row stays about thirty-two pixels whatever is set in
	// it, so the padding gives back what the line takes. Tuning a free design
	// parameter until TestNoPanelOverflowsTheRoomItIsGiven passes is what that
	// test is FOR -- it states what the layout has to satisfy, and the padding
	// is what there is to satisfy it with.
	bareChrome = 15 // around a control's own text
	labelGap   = 10 // between a caption and the control it names
)

// bareH is a row holding one control and no caption.
func bareH() int { return textH() + bareChrome }

// labelledH is a row holding a caption above its control, so it is two lines
// of text, not one.
func labelledH() int { return textH() + labelGap + bareH() }

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
	box.add(s.entryRow("Which pages", "1-3,7 — empty means this one", s.tools.spec), labelledH())
	box.add(buttons(
		s.verb("Keep only these", toolkit.ButtonDefault, s.selectPages, s.opened),
		s.verb("Delete these", toolkit.ButtonDanger, s.deleteRange, s.opened),
	), bareH())

	turns := toolkit.NewCycleButton("a quarter", "a half", "three quarters")
	// The positions and what they mean, bound through the table that says so.
	// This was `90 * (i + 1)` in a Subscribe: a mapping spelled as arithmetic,
	// readable only forwards, with no way for anything else to put a quarter
	// turn on the screen.
	tkbind.BindCycleValues(s.tools.turn, turns, quarterTurns, s.repaint)
	box.add(buttons(turns, s.verb("Turn them", toolkit.ButtonDefault, s.turnRange, s.opened)), bareH())
	box.add(s.verb("Reverse the order", toolkit.ButtonDefault, s.reverse, s.opened), bareH())

	box.add(s.spinRow("Move this page to", 1, s.tools.moveTo), labelledH())
	box.add(s.verb("Move it there", toolkit.ButtonDefault, s.movePage, s.opened), bareH())

	box.add(s.entryRow("Crop to, in points", "x0,y0,x1,y1", s.tools.box), labelledH())
	box.add(s.verb("Crop them", toolkit.ButtonDefault, s.crop, s.opened), bareH())

	box.add(s.spinRow("Put a blank page before", 1, s.tools.before), labelledH())
	box.add(s.verb("Insert it", toolkit.ButtonDefault, s.insertBlank, s.opened), bareH())

	box.add(s.spinRow("Split into files of", 1, s.tools.every), labelledH())
	box.add(s.verb("Split and hand them over", toolkit.ButtonProminent, s.split, s.opened), bareH())

	// Last, and not among the things that take a range: this one asks the
	// document which pages it means rather than being told.
	box.add(blankButton(s), bareH())
	return box
}

// sheetGroup is everything that puts more than one page's worth on a sheet, or
// more than one file into this one.
func (s *state) sheetGroup() *column {
	box := newColumn()
	box.add(s.spinRow("Pages to a sheet", 1, s.tools.up), labelledH())
	box.add(s.verb("Lay them out", toolkit.ButtonDefault, s.nUp, s.opened), bareH())
	box.add(s.verb("Fold it into a booklet", toolkit.ButtonDefault, s.booklet, s.opened), bareH())
	box.add(s.verb("Add a file after this one", toolkit.ButtonDefault, s.merge, s.opened), bareH())
	box.add(s.verb("Lay a file over this one", toolkit.ButtonDefault, s.overlay, s.opened), bareH())
	return box
}

// The four rows a panel is made of. Each one BINDS its control to the datum it
// shows, two ways, through mvvm/tkbind -- so the control and the model are the
// same value seen twice rather than two values kept in step by hand.
//
// repaint is what every binding is given as its invalidate: a datum that
// changed has to reach the canvas, and the canvas is redrawn when the scene
// says it is dirty.

// entryRow is a named box to type in, showing and setting a string.
func (s *state) entryRow(label, hint string, to *mvvm.Observable[string]) toolkit.Widget {
	e := toolkit.NewEntry(to.Get())
	e.Placeholder = hint
	tkbind.BindEntry(to, e, s.repaint)
	s.typing = append(s.typing, e)
	return toolkit.NewFormField(label, e)
}

// spinRow is a named number.
func (s *state) spinRow(label string, min int, to *mvvm.Observable[int]) toolkit.Widget {
	sp := toolkit.NewSpinButton(min, pageCeiling, to.Get(), 1)
	tkbind.BindSpin(to, sp, s.repaint)
	return toolkit.NewFormField(label, sp)
}

// chooseRow is a named list to choose from. The list is drawn over whatever is
// under it by the popover host the view is wrapped in, which is the one thing
// a drop-down cannot do for itself.
func (s *state) chooseRow(label string, options []string, to *mvvm.Observable[int]) toolkit.Widget {
	d := toolkit.NewDropDown(options, to.Get())
	tkbind.BindChoice(to, d, s.repaint)
	return toolkit.NewFormField(label, d)
}

// tickRow is a box to tick, which names itself.
func (s *state) tickRow(label string, on *mvvm.Observable[bool]) toolkit.Widget {
	c := toolkit.NewCheckButton(label, on.Get())
	tkbind.BindCheck(on, c, s.repaint)
	return c
}

// repaint is what a binding calls when either side moved. Only the canvas
// needs telling: the control and the model are already the same value.
func (s *state) repaint() { s.dirty = true }

// quarterTurns is what each position of the turn control means, in degrees.
var quarterTurns = []int{90, 180, 270}

// pageCeiling is as high as any of these numbers is allowed to go. It is not a
// page count: the document changes under the control, and a number that is too
// large is refused by the operation itself, which is the one place that knows.
const pageCeiling = 9999

// button is one control of the panel or the strip, pressable whenever it is
// on the screen.
func button(label string, style toolkit.ButtonStyle, on func()) *toolkit.Button {
	b := toolkit.NewButton(label, on)
	b.Style = style
	return b
}

// verb is a control that may only be pressed when the workbench is in a state
// that allows it: it carries a command, and the command's rule decides whether
// the control is pressable at all.
//
// ⛔ The rule is not a second copy of the guard inside the handler, and it is
// not there to make the handler safe -- the handler keeps its own guard, and
// Command.Execute refuses anyway. It is there to SAY SO BEFOREHAND. A control
// that looks pressable, is pressed, and answers with a sentence in the status
// line has told somebody afterwards what it could have shown them before: the
// arrows grey at the ends of a document, Delete greys on a document of one
// page, and everything greys when there is nothing open.
func (s *state) verb(label string, style toolkit.ButtonStyle, on func(), when func() bool) *toolkit.Button {
	cmd := mvvm.NewCommand(on, when)
	s.commands = append(s.commands, cmd)
	b := button(label, style, nil)
	tkbind.BindButton(cmd, b, s.repaint)
	return b
}

// opened is the rule nearly every verb has: there is a document to act on.
func (s *state) opened() bool { return s.doc != nil }

// reconsider tells every verb to look again at whether it may run.
//
// Commands do not watch anything -- CanExecute is a question, asked when
// something says the answer may have changed -- so this is called from
// refresh, which is what runs after every change to the document.
func (s *state) reconsider() {
	for _, c := range s.commands {
		c.RaiseCanExecuteChanged()
	}
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
	if strings.TrimSpace(s.tools.spec.Get()) == "" {
		return pageSpec(s.at)
	}
	return s.tools.spec.Get()
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
	spec, by := s.where(), s.tools.turn.Get()
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
	from, to := s.at, s.tools.moveTo.Get()
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
	box, err := parseBox(s.tools.box.Get())
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
	at := s.tools.before.Get()
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
	parts, err := s.doc.Split(s.tools.every.Get())
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
	n := s.tools.up.Get()
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
