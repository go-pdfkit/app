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
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/mvvm/tkbind"
	"github.com/go-widgets/toolkit"
)

// panelW is how wide the tool panel is, and gap the space between it and the
// page beside it.
// Logical lengths; rescaleLayout in scene.go is the one place that re-derives
// them for the display the page is on.
var (
	panelW = 300
	gap    = toolkit.DefaultBoxSpacing
)

// The groups, in the order they appear on the strip. A group is named by what
// it does to the file rather than by which library call it makes: somebody
// looking for "two to a sheet" is thinking about the sheet, not about NUp.
const (
	groupPages = "Pages"
	groupSheet = "Sheet"
	groupMarks = "Marks"
	// ⛔⛔ STAMP IS A TAB BECAUSE THE MARKS PANEL WAS 21 PIXELS FROM ITS LIMIT.
	// Adding a header and a footer box took it 109 pixels past the room it is
	// given, and TestNoPanelOverflowsTheRoomItIsGiven refuses that outright:
	// a control that can only be scrolled to is one a reader does not know is
	// there. The split is the gate's own answer, as it was for `Hand over`,
	// `Mark up`, `Redraw` and `Contents`.
	//
	// The line was in the groups: Marks writes the same thing across every page
	// by a rule — a watermark, a page number, a running serial — and Stamp puts
	// something at a place you CHOOSE, which is the question the `Where it goes`
	// chooser asks and the one a picture asks too.
	groupStamp   = "Stamp"
	groupFile    = "File"
	groupProtect = "Protect"
	groupRead    = "Read"
	// ⛔ HAND OVER IS A TAB OF ITS OWN BECAUSE THE READ TAB HAD EIGHTEEN
	// CONTROLS. The ribbon's answer to a crowded strip is to shrink a group to
	// icon-sized buttons — measured: the four readings came out 28 pixels wide
	// with no words on them — so nothing was unreachable, it was merely
	// unreadable. Six verbs that all answer "give me a copy" are a tab, and
	// splitting them off puts every control on both tabs back at full size.
	//
	// The split is the right one anyway: everything left on Read answers "what
	// does this page say", and the picture-format chooser, which sat on the Read
	// panel for want of anywhere else, is exactly what this tab is told.
	//
	// (What made the crowding a BUG rather than a squeeze was a name: the Read
	// tab's first group was also called "Read". See ribbonTabs.)
	groupHandOver = "Hand over"
	// ⛔⛔ MARK UP IS A TAB BECAUSE THE READ TAB CROWDED AGAIN, and this is the
	// SECOND time the same measurement forced a split — the first produced
	// `Hand over`. Adding the three shapes made a seventh group, and seven of
	// the tab's controls came out 28 pixels wide with no words on them: the
	// readings, the search and Copy all reduced to bare icons. Nothing was
	// dropped, so the reachability gate passed; it does not measure legibility,
	// and a probe does.
	//
	// The line to split on was already in the groups: Read answers "what does
	// this page say" — read it, search it, copy it — and everything else on that
	// tab CHANGED the document through the selection. So the marks, the shapes,
	// the comments and the two verbs that alter the text moved here, and the
	// boxes they read moved with them.
	groupMarkUp = "Mark up"
	// ⛔⛔ REDRAW IS A TAB BECAUSE THE GATE FOUND THE FILE TAB ALREADY BROKEN.
	// The five redraw verbs were added to the File tab in v0.32.0 and shipped;
	// the suite was green because every control still had a rectangle. The
	// widened gate measured their WIDTH and found six of that tab's controls at
	// 28 pixels with no words on them — Sanitize, Flatten, the two Drops and
	// both Files-inside verbs, reduced to bare icons by a group I added and
	// never looked at.
	//
	// It is the right tab anyway. Redrawing replaces every page with a picture
	// of it, which is why each of its verbs carries a warning the others do not
	// need, and the two spinners and the ground chooser are what it is told.
	groupRedraw = "Redraw"
	// ⛔⛔ CONTENTS IS A TAB BECAUSE THE GATE REFUSED THE FOURTH SPLIT OF THE
	// FILE TAB. Writing the bookmarks needed two verbs beside `Drop the
	// bookmarks`, and a fifth group on File took Sanitize, Flatten and `Drop
	// every annotation` down to 28 pixels with no words on them — the same
	// measurement that produced `Hand over`, `Mark up` and `Redraw`, firing for
	// the third tab in two days. The gate names the fix in its own message:
	// split the tab.
	//
	// The line to split on was in the groups again. Clean, `Files inside` and
	// Size are what a file is stripped of and how big it ends up; a reader
	// deciding what a document's table of contents SAYS is asking about the
	// document, and the rail beside them is already showing the answer.
	groupContents = "Contents"
)

// groupNames is the order they are offered in.
var groupNames = []string{
	groupPages, groupSheet, groupMarks, groupStamp, groupFile, groupContents, groupRedraw,
	groupProtect, groupRead, groupMarkUp, groupHandOver,
}

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

	// ⛔ CUTTING AND TILING KEEP THEIR OWN NUMBERS, and they look like the same
	// two numbers. Divide cuts one page into smaller pages of its own content;
	// Poster spreads one page over several sheets to print it larger. "2 × 2"
	// means halves in the first and quarters-enlarged in the second, so one
	// pair shared between them would show the wrong thing in whichever was not
	// last used — the reason the Marks group has a range of its own.
	cutAcross, cutDown     *mvvm.Observable[int]
	tilesAcross, tilesDown *mvvm.Observable[int]

	// paper is which sheet size the pages are set to, and ink which colour
	// their text is painted. Both are the position in their own list.
	paper *mvvm.Observable[int]
	ink   *mvvm.Observable[int]

	// The redrawing: which colour the pages are drawn on, and the brightness
	// and contrast, both as PERCENTAGES because a spin button holds whole
	// numbers — 100 contrast means "leave it alone", which is convert's 1.
	ground           *mvvm.Observable[int]
	bright, contrast *mvvm.Observable[int]

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
	// header and footer are the two lines `Head and foot them` writes, which
	// are their own boxes rather than the stamp's: a reader who has typed a
	// header should not lose it by stamping something else.
	header, footer *mvvm.Observable[string]
	at             *mvvm.Observable[int] // where on the page it goes
	size           *mvvm.Observable[int] // how large, in points
	// wide is how wide a PLACED PICTURE is drawn, in points, and it is its own
	// box rather than `size`: that one is a font size, and 12-point type beside
	// a 12-point logo would be two different meanings of one control.
	wide *mvvm.Observable[int]

	// The File group. info holds the other entries of the information
	// dictionary the panel offers a box for, one Observable per KEY rather
	// than one over a map: a map is not comparable, so one Observable over the
	// whole of it could not tell a change from a re-set.
	title, author *mvvm.Observable[string]
	info          map[reader.Name]*mvvm.Observable[string]

	// The Protect group.
	openPw, userPw, ownerPw *mvvm.Observable[string]
	// allow is one Observable PER PERMISSION rather than an Observable of a
	// map: a map is not comparable, so one Observable over the whole of it
	// could not tell a change from a re-set, and every tick would repaint
	// every box.
	allow map[string]*mvvm.Observable[bool]

	// heading is what a printed table of contents is titled. Empty means the
	// library's own "Contents", which is what the box's placeholder shows.
	heading *mvvm.Observable[string]

	// find is what the search is looking for, and comment what a sticky note or
	// a margin comment will say.
	find    *mvvm.Observable[string]
	comment *mvvm.Observable[string]
	// replacement is what `Replace the words` puts in place of the selection.
	replacement *mvvm.Observable[string]
	// fieldName is what a field made from a selection is called, or empty to
	// take the name from the words themselves; fieldKind is which kind of
	// field, by position in fieldKinds.
	fieldName *mvvm.Observable[string]
	fieldKind *mvvm.Observable[int]

	// The Read group: which reading of the document is on the screen instead
	// of the picture of it, or empty for the picture, and which of the
	// writable picture formats a page is handed over in.
	reading *mvvm.Observable[string]
	picture *mvvm.Observable[int]
}

// newTools builds the panel's state with the defaults each control starts at.
func newTools() *tools {
	return &tools{
		built:       map[string]toolkit.Widget{},
		spec:        mvvm.NewObservable(""),
		turn:        mvvm.NewObservable(90),
		moveTo:      mvvm.NewObservable(1),
		box:         mvvm.NewObservable(""),
		before:      mvvm.NewObservable(1),
		every:       mvvm.NewObservable(1),
		up:          mvvm.NewObservable(2),
		cutAcross:   mvvm.NewObservable(2),
		cutDown:     mvvm.NewObservable(1),
		tilesAcross: mvvm.NewObservable(2),
		tilesDown:   mvvm.NewObservable(2),
		paper:       mvvm.NewObservable(0),
		ink:         mvvm.NewObservable(0),
		ground:      mvvm.NewObservable(0),
		bright:      mvvm.NewObservable(0),
		contrast:    mvvm.NewObservable(100),
		markSpec:    mvvm.NewObservable(""),
		mark:        mvvm.NewObservable("DRAFT"),
		numbers:     mvvm.NewObservable("{page} / {pages}"),
		prefix:      mvvm.NewObservable(""),
		start:       mvvm.NewObservable(1),
		digits:      mvvm.NewObservable(6),
		stamp:       mvvm.NewObservable("COPY"),
		header:      mvvm.NewObservable(""),
		footer:      mvvm.NewObservable("{page} of {pages}"),
		at:          mvvm.NewObservable(0),
		size:        mvvm.NewObservable(12),
		// 144 points is two inches: a signature or a logo on a letter, which is
		// what a placed picture nearly always is. ⛔ NOT ops' own default, which
		// is one point per pixel — right for `Picture`, where the picture IS the
		// page, and wrong here, where a 1000-pixel logo would be laid down 1000
		// points wide and run off the paper.
		wide:        mvvm.NewObservable(144),
		title:       mvvm.NewObservable(""),
		author:      mvvm.NewObservable(""),
		info:        emptyInfo(),
		heading:     mvvm.NewObservable(""),
		openPw:      mvvm.NewObservable(""),
		userPw:      mvvm.NewObservable(""),
		ownerPw:     mvvm.NewObservable(""),
		allow:       everythingAllowed(),
		find:        mvvm.NewObservable(""),
		comment:     mvvm.NewObservable(""),
		replacement: mvvm.NewObservable(""),
		fieldName:   mvvm.NewObservable(""),
		fieldKind:   mvvm.NewObservable(0),
		reading:     mvvm.NewObservable(""),
		picture:     mvvm.NewObservable(0),
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
	case groupStamp:
		rows = s.stampGroup()
	case groupMarks:
		rows = s.marksGroup()
	case groupProtect:
		rows = s.protectGroup()
	case groupContents:
		rows = s.contentsGroup()
	case groupRead:
		rows = s.readGroup()
	case groupMarkUp:
		rows = s.markUpGroup()
	case groupRedraw:
		rows = s.redrawGroup()
	case groupHandOver:
		rows = s.handOverGroup()
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
// Room around and between the rows, and the width they are laid out at: the
// panel less its frame and the scrollbar down its edge.
//
// What a row costs besides its text: 15 rather than 16 because the face
// changed and its line is a pixel taller, so a control row stays about
// thirty-two pixels whatever is set in it and the padding gives back what the
// line takes. Tuning a free design parameter until
// TestNoPanelOverflowsTheRoomItIsGiven passes is what that test is FOR -- it
// states what the layout has to satisfy, and the padding is what there is to
// satisfy it with.
var (
	rowGap     = 6
	rowsW      = panelW - 34
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

	turns := toolkit.NewCycleButton("a quarter", "a half", "three quarters")
	// The positions and what they mean, bound through the table that says so.
	// This was `90 * (i + 1)` in a Subscribe: a mapping spelled as arithmetic,
	// readable only forwards, with no way for anything else to put a quarter
	// turn on the screen.
	tkbind.BindCycleValues(s.tools.turn, turns, quarterTurns, s.repaint)
	box.add(toolkit.NewFormField("Turn them by", turns), labelledH())

	box.add(s.spinRow("Move this page to", 1, s.tools.moveTo), labelledH())
	box.add(s.entryRow("Crop to, in points", "x0,y0,x1,y1", s.tools.box), labelledH())
	box.add(s.spinRow("Put a blank page before", 1, s.tools.before), labelledH())
	// ⛔ The paper and the crop box are different things and sit apart: the box
	// above cuts the page down to part of itself, this sets the sheet it is on.
	// A page cropped to a postcard on A4 paper is still on A4.
	box.add(s.chooseRow("Set the paper to", paperNames(), s.tools.paper), labelledH())
	box.add(s.chooseRow("Paint the text", inkNames(), s.tools.ink), labelledH())
	box.add(s.spinRow("Split into files of", 1, s.tools.every), labelledH())
	return box
}

// sheetGroup is everything that puts more than one page's worth on a sheet, or
// more than one file into this one.
func (s *state) sheetGroup() *column {
	box := newColumn()
	box.add(s.spinRow("Pages to a sheet", 1, s.tools.up), labelledH())
	box.add(buttons(
		s.spinRow("Cut across", 1, s.tools.cutAcross),
		s.spinRow("and down", 1, s.tools.cutDown),
	), labelledH())
	box.add(buttons(
		s.spinRow("Tiles across", 1, s.tools.tilesAcross),
		s.spinRow("and down", 1, s.tools.tilesDown),
	), labelledH())
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

// button is a control that is pressable whenever it is on the screen. The
// ribbon's verbs do not go through it -- they are Actions -- so the one caller
// left is the row a reading of the page builds per picture it found.
func button(label string, style toolkit.ButtonStyle, on func()) *toolkit.Button {
	b := toolkit.NewButton(label, on)
	b.Style = style
	return b
}

// opened is the rule nearly every verb has: there is a document to act on.
func (s *state) opened() bool { return s.doc != nil }

// reconsider tells every verb to look again at whether it may run.
//
// ⛔ The rule is not a second copy of the guard inside the handler, and it is
// not there to make the handler safe -- the handler keeps its own guard. It is
// there to SAY SO BEFOREHAND. A control that looks pressable, is pressed, and
// answers with a sentence in the status line has told somebody afterwards what
// it could have shown them before: the arrows grey at the ends of a document,
// Delete greys on a document of one page, and everything greys when there is
// nothing open.
//
// An Action does not watch anything -- whether a verb may run is a question,
// asked when something says the answer may have changed -- so this is called
// from refresh, which is what runs after every change to the document.
func (s *state) reconsider() {
	for id, a := range s.acts {
		a.Enabled.Set(s.mayRun(s.rules[id]))
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

// split hands the pieces over. Nothing is changed here: what was open stays
// open, and the pieces are copies of it.
//
// ⛔⛔ SEVERAL PIECES GO IN ONE ZIP, and that is the whole of BentoPDF's "PDFs
// to ZIP". This handed over one download per piece and said so itself —
// "a browser may ask before it takes more than one" — which is an apology for
// the verb rather than a verb: a document of two hundred pages split into
// twenties is ten prompts, each of which can be refused separately, and the
// person who refuses the fourth has no way of knowing which four they have.
// `Hand over every page, zipped` had reached the same conclusion about
// pictures; this is the same answer for documents.
//
// ⛔ One piece is still one PDF. A zip holding a single file is a wrapper
// somebody has to open for nothing.
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
	// ⛔ ONE PIECE'S BYTES AT A TIME, written into the zip as it is produced.
	// Collecting them all first and zipping afterwards holds the whole document
	// twice over — every piece plus the archive of them — and this runs inside a
	// browser tab, on a file somebody chose, split as finely as they like: a
	// hundred-megabyte scan split a page at a time is the case that decides it.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, part := range parts {
		out, perr := docBytes(part)
		if perr != nil {
			s.fail("part " + strconv.Itoa(i+1) + " cannot be written: " + perr.Error())
			return
		}
		if len(parts) == 1 {
			s.handOver(partName(s.name, 1), out)
			return
		}
		// The zip is built in memory: Create only refuses a name already used
		// or a writer already closed, neither of which can happen with one
		// entry per piece, and a bytes.Buffer never fails to take bytes.
		w, _ := zw.Create(partName(s.name, i+1))
		w.Write(out)
	}
	zw.Close()
	s.handOver(strings.TrimSuffix(saveName(s.name), "-edited.pdf")+"-pieces.zip", buf.Bytes())
	s.note = fmt.Sprintf("handed over %d pieces in one zip", len(parts))
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

// merge adds the pages of one or more other files after this one's, in the
// order they were chosen.
//
// ⛔ SEVERAL AT ONCE, which is what the row this answers asks for: a merge is
// nearly always "these five scans into one document", and one file at a time
// means five presses and five dialogs, each of which can be cancelled, leaving
// somebody with a document that is part-way through a job and no record of how
// far. The other verbs that ask for a file take exactly one and keep the single
// picker.
//
// ⛔ A file among them that cannot be read STOPS the merge and says which one.
// Appending the ones before it and dropping the rest would leave a document
// nobody asked for, and saying nothing about the others would be worse.
func (s *state) merge() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	s.host.OpenMany(func(files []chosen) {
		if len(files) == 0 {
			return // the person changed their mind, which is not an error
		}
		others := make([]*ops.Doc, 0, len(files))
		names := make([]string, 0, len(files))
		for _, f := range files {
			other, ok := s.documentOf(f)
			if !ok {
				return // the arm that refused it has said why
			}
			others = append(others, other)
			names = append(names, f.name)
		}
		said := "added " + names[0]
		if len(names) > 1 {
			said = fmt.Sprintf("added %d files: %s", len(names), strings.Join(names, ", "))
		}
		s.changeSaying(said, func(d *ops.Doc) error {
			for _, other := range others {
				d.Append(other)
			}
			return nil
		})
	})
}

// documentOf is one chosen file as a document, whatever shape it arrived in,
// or false with the reason already on the status line.
//
// ⛔⛔ A CHOSEN FILE IS WHATEVER THE FIRST ONE MAY BE. There is one filter list
// behind every picker, so the moment Open accepted a picture, a drawing and a
// comic archive, these choosers started offering them too — and refusing them
// here is the SECOND direction of the gate in opened.go: a file the dialog shows
// and the code will not take is a file somebody chose for nothing.
func (s *state) documentOf(f chosen) (*ops.Doc, bool) {
	other, err := ops.Open(f.data)
	if err == nil {
		return other, true
	}
	// Putting a scan after a document, laying a signature over a page and
	// interleaving a photographed back page are the three things anybody tries.
	from, tried := s.documentFrom(f.data)
	if !tried {
		s.fail("cannot open " + f.name + ": " + err.Error())
		return nil, false
	}
	if from.doc == nil {
		return nil, false // the arm that recognised it has already said why
	}
	return from.doc, true
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
		other, ok := s.documentOf(chosen{name: name, data: data})
		if !ok {
			return
		}
		s.changeSaying(said+" "+name, func(d *ops.Doc) error { return join(d, other) })
	})
}
