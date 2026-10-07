// The workbench itself: what is on the screen, and what each control does to
// the document. Kept free of any build tag so a native test can drive the
// whole thing against a plain byte buffer, which is how the behaviour here is
// checked without a browser.

package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// A host is what the workbench needs from the page it is running in: a way to
// ask for a file and a way to hand one back. The browser supplies one; a test
// supplies another, which is what keeps the rest of this file testable.
type host interface {
	// Open asks the person for a file and calls back with what they chose.
	Open(func(name string, data []byte))
	// Save hands a file to the person under the given name.
	Save(name string, data []byte)
}

// The size of the surface the workbench is laid out on. The canvas keeps its
// aspect in the page, so this is a design size rather than a pixel count.
const (
	surfaceW = 1000
	surfaceH = 720

	// The smallest surface the bands still fit on. Below this the page band
	// would be zero or negative, so a resize to it is refused and the last
	// good layout stands.
	minSurfaceW = 2*margin + 1
)

// minSurfaceH is the smallest height the three bands still fit in. It follows
// the installed face, because two of those bands are sized by their text.
func minSurfaceH() int { return viewTop() + statusH() + margin + 1 }

// Geometry of the three bands: a toolbar, the page, a status line.
//
// ⛔ The band heights are FUNCTIONS, not constants, and that is the point of
// this block. Every band holds text, so every band's height is the text it
// holds plus the chrome around it. When these were written there was only one
// face -- the toolkit's compiled-in 5x7 bitmap -- so the text contributed 7
// pixels to each of them and nobody had to say which 7. Installing a vector
// face made the glyph taller and the bands did not follow: labels clipped, and
// the tests that press at literal offsets pressed past their target.
//
// The chrome figures are the padding a line of type wants around it, not the
// padding a 5x7 bitmap happened to be given: the old numbers were 23 and 15
// around a 7-pixel glyph, which with a 16-pixel one made every band taller
// than the room it had. TestNoPanelOverflowsTheRoomItIsGiven is the control --
// it measures the laid-out panels against their own box, so it fails for any
// face that does not fit rather than for a number that changed.
const (
	margin = 8

	toolbarChrome = 16 // padding around a toolbar button's label
	statusChrome  = 10 // padding around the status line's text
	// ⛔ viewW and viewH are NOT constants any more: they were
	// surfaceW/surfaceH minus the bands, computed once at compile time, which
	// is what kept the workbench at its design size however big the window
	// was. They are methods on state now, over s.w and s.h.
)

// textH is the height of one line of text in the face currently installed.
func textH() int { return toolkit.CurrentFont().Height() }

func toolbarH() int { return textH() + toolbarChrome }
func statusH() int  { return textH() + statusChrome }
func viewTop() int  { return margin + toolbarH() + margin }

// A state is the whole workbench.
type state struct {
	w, h  int
	theme *toolkit.Theme
	host  host

	toolbar *toolkit.HBox
	status  *toolkit.Statusbar
	// view is the band under the strip: the page, or the form, with the tool
	// panel beside it when a group of verbs is open — so it is a widget of
	// whatever kind that arrangement needs rather than always a frame.
	view  toolkit.Widget
	page  *toolkit.Image
	empty *toolkit.Label

	// doc is what every operation acts on, and src is the same document
	// parsed, which is what gets drawn. They are rebuilt together after every
	// change, so what is shown is always what would be saved.
	doc  *ops.Doc
	src  *reader.Document
	name string
	// raw is the file as it arrived. A form is filled in on the file itself
	// rather than on a document rebuilt around it, so the bytes are kept.
	raw []byte
	// reopenPw is the password the document will need to be read back with,
	// which is empty until somebody asks for it to be protected. Reading it
	// back is how the workbench draws it, so protecting a file would
	// otherwise put a message where the page was.
	reopenPw string
	// form is what the document asks to be filled in, when it asks anything,
	// and showingForm says the panel is up instead of the page.
	form        *filling
	showingForm bool
	// tools is the panel of verbs beside the page, and typing every box on
	// the screen that takes characters — which is what says whether an arrow
	// key belongs to a word somebody is writing or to the pages.
	tools *tools
	// thumbs is the rail of minipages: what has been drawn of the document
	// now open, and of which bytes. railView is the rail as a widget, nil when
	// there is no reason for one, and railMore says some of it is still being
	// drawn.
	thumbs   *thumbs
	railView toolkit.Widget
	railMore bool
	// shown is what the view band is holding: the page, a form, a reading, or
	// a sentence saying why there is none of those.
	shown  toolkit.Widget
	typing []*toolkit.Entry
	at     int // the page being shown, counting from one
	note   string

	// dirty says something has changed since the canvas last showed it. A
	// file arrives from the browser long after the press that asked for it,
	// so there is no event left to repaint on; the harness asks here instead.
	dirty bool
}

// newState builds the workbench.
func newState(w, h int, h2 host) *state {
	s := &state{w: w, h: h, theme: toolkit.DefaultLight(), host: h2, at: 1, tools: newTools()}
	s.empty = toolkit.NewLabel("Open a PDF to begin — nothing leaves this tab.")
	s.view = toolkit.NewFrame(s.empty)
	s.status = toolkit.NewStatusbar([]string{"no document", "", ""})
	s.toolbar = s.strip()
	s.refresh()
	return s
}

// strip is every control at the top, in the order they appear. They are
// buttons rather than a Toolbar because a Toolbar is a strip of square icon
// cells that shows only a label's first letter, and these controls are named
// by their words: "Pages" and "Sheet" would both be reduced to an S.
//
// What is on it is what needs no telling: opening, saving, turning to another
// page, turning the one on the screen over, dropping it. Everything else has
// to be told which pages, or how many, or what to write, and lives in the
// panel that a group opens beside the page.
func (s *state) strip() *toolkit.HBox {
	box := toolkit.NewHBox()
	add := func(label string, style toolkit.ButtonStyle, on func()) {
		b := button(label, style, on)
		if m, ok := stripIcons[label]; ok {
			g := iconoirGlyph(m.name)
			if m.insteadOfTheWord {
				b.Icon = g
			} else {
				b.LeadingIcon().Set(g)
			}
		}
		box.AddFixed(b, controlWidth(b))
	}
	add("Open", toolkit.ButtonProminent, s.open)
	add("Save", toolkit.ButtonProminent, s.save)
	add("<", toolkit.ButtonDefault, func() { s.step(-1) })
	add(">", toolkit.ButtonDefault, func() { s.step(1) })
	add("Rotate", toolkit.ButtonDefault, s.rotate)
	add("Delete", toolkit.ButtonDanger, s.deletePage)
	// Then one control per group of verbs. What each one opens is a panel
	// beside the page rather than another handful of buttons, because most of
	// what is left needs to be told something first.
	for _, name := range groupNames {
		add(name, toolkit.ButtonDefault, func() { s.showGroup(name) })
	}
	add("Fill in", toolkit.ButtonDefault, s.showForm)
	return box
}

// stripIcons is the mark each control on the strip carries.
//
// ⛔ Through the toolkit, from a pack, never drawn here. toolkit.DrawIconoir
// rasterises one of the 1 383 SVGs that go-icons/iconoir embeds, recoloured to
// the ink the button is using and cached per icon and size. A glyph outlined
// by hand in this file would be a per-app duplicate of a layer that exists to
// be shared, and at the fourteen pixels a dense toolbar gives it, a hand-drawn
// outline reads as a square.
//
// insteadOfTheWord is for the two controls whose caption IS a mark: an arrow
// beside a "<" is the same thing said twice, and it costs the width of both.
var stripIcons = map[string]struct {
	name             string
	insteadOfTheWord bool
}{
	"Open":    {name: "folder"},
	"Save":    {name: "download"}, // what saving does here: the tab hands the file back
	"<":       {name: "nav-arrow-left", insteadOfTheWord: true},
	">":       {name: "nav-arrow-right", insteadOfTheWord: true},
	"Rotate":  {name: "crop-rotate-tr"}, // a page-shaped frame being turned
	"Delete":  {name: "trash"},
	"Pages":   {name: "multiple-pages"},
	"Sheet":   {name: "view-grid"}, // several pages laid out on one
	"Marks":   {name: "edit-pencil"},
	"File":    {name: "page"},
	"Protect": {name: "lock"},
	"Read":    {name: "text-magnifying-glass"},
	"Fill in": {name: "input-field"},
}

// iconoirGlyph draws one icon from the pack at whatever box it is given.
func iconoirGlyph(name string) toolkit.IconFunc {
	return func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
		toolkit.DrawIconoir(p, r, name, ink)
	}
}

// controlWidth is what a control needs to show what it holds.
//
// ⛔ ASKED OF THE CONTROL. This used to be `TextWidth(label) + 2*padding`,
// computed here -- a second copy of a layout the button already knows, and one
// that counted no glyph at all. PreferredWidth measures the caption, the
// leading mark and the paddings with the same numbers Draw lays them out with,
// so the strip cannot disagree with what is painted on it.
func controlWidth(b *toolkit.Button) int {
	if w := b.PreferredWidth(); w > minButtonW {
		return w
	}
	return minButtonW
}

// minButtonW is the floor, so a control named by one character is still
// something to press.
const minButtonW = 28

// open asks for a file and takes it as the document.
func (s *state) open() {
	s.host.Open(func(name string, data []byte) {
		fromPicture := false
		d, err := ops.OpenWithPassword(data, s.tools.openPw.Get())
		if err != nil {
			// Somebody who hands a picture to a PDF workbench wants a PDF of
			// it. Telling them it is not a PDF is true and useless: they knew.
			pic, made := s.pictureDocument(data)
			if made {
				d, data, name, fromPicture = pic, nil, name+".pdf", true
			} else {
				s.fail("cannot open " + name + ": " + err.Error())
				return
			}
		}
		if data == nil {
			// A document begun from a picture has no file behind it yet, so it
			// is written once here to be the file the rest of the workbench
			// reads back.
			out, msg := docBytes(d)
			if msg != nil {
				s.fail("this picture cannot be made into a document: " + msg.Error())
				return
			}
			data = out
		}
		s.doc, s.name, s.at, s.raw = d, name, 1, data
		s.note = ""
		if fromPicture {
			s.note = "this was a picture, and is now a page"
		}
		s.showingForm = false
		s.reopenPw = ""
		s.tools.reading.Set("")
		s.readForm(data)
		if s.form != nil {
			s.note = fmt.Sprintf("this document has a form: %d fields",
				len(s.form.what.Form().Fields()))
		}
		s.refresh()
	})
}

// pictureDocument makes a one-page document of an image, and says whether the
// bytes were one.
//
// A picture at 72 pixels to the inch is a page the size of the picture, which
// is what somebody scanning a receipt or photographing a page expects to get
// back. It is also the only guess available: nothing in a PNG says how large
// it is meant to be on paper.
func (*state) pictureDocument(data []byte) (*ops.Doc, bool) {
	d := ops.New()
	if err := d.Picture(data, 0); err != nil {
		return nil, false
	}
	return d, true
}

// save hands the document back, as it now stands.
func (s *state) save() {
	if s.doc == nil {
		s.fail("there is nothing to save")
		return
	}
	out, msg := s.saveBytes()
	if msg != "" {
		s.fail(msg)
		return
	}
	s.host.Save(saveName(s.name), out)
	s.note = fmt.Sprintf("saved %d bytes", len(out))
	s.refresh()
}

// saveName is what a changed document is offered under.
func saveName(name string) string {
	if name == "" {
		return "document.pdf"
	}
	if len(name) > 4 && name[len(name)-4:] == ".pdf" {
		return name[:len(name)-4] + "-edited.pdf"
	}
	return name + "-edited.pdf"
}

// step moves to another page.
func (s *state) step(by int) {
	if s.doc == nil {
		return
	}
	next := s.at + by
	if next < 1 || next > s.doc.PageCount() {
		return
	}
	s.at = next
	s.refresh()
}

// rotate turns the page being shown a quarter clockwise.
func (s *state) rotate() { s.change(func(d *ops.Doc) error { return d.Rotate(pageSpec(s.at), 90) }) }

// deletePage drops the page being shown.
func (s *state) deletePage() {
	if s.doc != nil && s.doc.PageCount() == 1 {
		s.fail("a document needs a page")
		return
	}
	// Deleting the page at the end leaves the number pointing past the end;
	// nothing is done about it here because redrawing clamps it, which is the
	// one place that has to be right whatever shrank the document.
	at := s.at
	s.change(func(d *ops.Doc) error { return d.Delete(pageSpec(at)) })
}

// sanitize strips whatever in the file runs rather than shows.
func (s *state) sanitize() {
	s.changeSaying("stripped what runs rather than shows", func(d *ops.Doc) error {
		d.Sanitize()
		return nil
	})
}

// pageSpec names one page the way a range is written.
func pageSpec(at int) string { return fmt.Sprintf("%d", at) }

// change applies an operation and shows the result, or says why it could not.
func (s *state) change(apply func(*ops.Doc) error) bool { return s.changeSaying("", apply) }

// changeSaying is change with something to say for itself when it worked. The
// note is set before the redraw rather than after it, because the redraw is
// what builds the line it appears on — and because a page that ran out of time
// being drawn has more to say than the verb that changed it did.
func (s *state) changeSaying(said string, apply func(*ops.Doc) error) bool {
	if s.doc == nil {
		s.fail("open a document first")
		return false
	}
	if err := apply(s.doc); err != nil {
		s.fail(err.Error())
		return false
	}
	s.note = said
	s.refresh()
	return true
}

// fail puts a message on the status line.
func (s *state) fail(msg string) {
	s.note = msg
	s.refresh()
}

// refresh redraws the page being shown and the status line. Everything is
// rebuilt from the document as it now stands, so what is on the screen is
// always what would come out of Save.
func (s *state) refresh() {
	s.keepPageNumbersInTheDocument()
	s.renderPage()
	s.status = toolkit.NewStatusbar(s.statusLine())
	s.dirty = true
}

// keepPageNumbersInTheDocument holds the panel's page numbers inside the
// document they refer to, so that dropping pages 3 to 9 does not leave "Put a
// blank page before 7" offering a page that is no longer there.
//
// ⛔ This is the direction the panel did not have. Its numbers used to be
// plain fields filled in by a Subscribe on each control, so a value could
// travel from the control to the model and NEVER the other way: the only way
// back in was to rebuild the widget, and a panel is built once and kept. The
// scene therefore could not correct them, and did not try -- which is
// invisible until the document shrinks under a number somebody set before it
// did.
//
// Now the control and the model are the same Observable, so a Set here is what
// the panel shows.
func (s *state) keepPageNumbersInTheDocument() {
	if s.doc == nil {
		return
	}
	n := s.doc.PageCount()
	for _, v := range []*mvvm.Observable[int]{s.tools.moveTo, s.tools.before} {
		if v.Get() > n {
			v.Set(n)
		}
		if v.Get() < 1 {
			v.Set(1)
		}
	}
}

// takeDirty reports whether anything has changed since it was last asked, and
// forgets it.
func (s *state) takeDirty() bool {
	was := s.dirty
	s.dirty = false
	return was
}

// statusLine is what the bottom of the window says.
func (s *state) statusLine() []string {
	if s.doc == nil {
		return []string{"no document", s.note, "nothing leaves this tab"}
	}
	where := fmt.Sprintf("page %d of %d", s.at, s.doc.PageCount())
	if s.showingForm && s.form != nil {
		where = fmt.Sprintf("%d fields", len(s.form.what.Form().Fields()))
	}
	return []string{s.name, where, s.note}
}

// reopenBytes is the document as it would be saved, or the reason it cannot
// be written. It reports that reason rather than an error, because the only
// thing to do with it is show it.
// saveBytes is what a press of Save writes. A form that has been filled in is
// saved as the file it came from with the answers appended, because that is
// the only way of saving one that keeps it a form; anything else here rebuilds
// the document, and the form does not survive that.
func (s *state) saveBytes() ([]byte, string) {
	if s.form != nil && s.form.changed > 0 {
		return s.form.bytes()
	}
	return s.reopenBytes()
}

func (s *state) reopenBytes() ([]byte, string) {
	out, err := docBytes(s.doc)
	if err != nil {
		return nil, "this document cannot be written: " + err.Error()
	}
	return out, ""
}

// reopen writes the document and reads it back, which is how what is on the
// screen is kept the same as what would come out of Save.
func (s *state) reopen() (*reader.Document, string) {
	src, _, msg := s.reopenWithBytes()
	return src, msg
}

// reopenWithBytes is the same and hands back what it was read from, which the
// rail keys its minipages on: the bytes are the one thing that says whether
// the document is still the document those pictures were drawn from.
func (s *state) reopenWithBytes() (*reader.Document, []byte, string) {
	out, msg := s.reopenBytes()
	if msg != "" {
		return nil, nil, msg
	}
	src, err := openBytes(out, s.reopenPw)
	if err != nil {
		return nil, nil, "this document cannot be read back: " + err.Error()
	}
	return src, out, ""
}

// These three are variables so a test can watch what the workbench does when
// writing, re-reading or drawing a document it is already holding goes wrong.
// None of the three should ever fail on a document that opened, which is
// exactly why it is worth being able to see what happens when one does.
var (
	docBytes  = (*ops.Doc).Bytes
	openBytes = reader.OpenWithPassword
	drawPage  = render.Page
)

// renderPage draws the current page into the view, or leaves the invitation
// there when there is nothing to draw.
func (s *state) renderPage() {
	s.page = nil
	s.railView, s.railMore = nil, false
	if s.doc == nil {
		s.show(s.empty)
		return
	}
	if s.showingForm && s.form != nil {
		s.show(s.form.panel(s))
		return
	}
	src, raw, msg := s.reopenWithBytes()
	if msg != "" {
		s.show(toolkit.NewLabel(msg))
		return
	}
	s.src = src
	if s.at > src.PageCount() {
		s.at = src.PageCount()
	}
	if s.at < 1 {
		s.at = 1
	}
	// A reading of the page is read out of the document written and read back,
	// like the picture of it, so what is listed is what would come out of Save.
	if s.tools.reading.Get() != "" {
		s.show(s.readingView(src))
		return
	}
	// The rail is settled BEFORE the page is scaled, because the page is
	// scaled to the room the rail leaves it.
	s.railView, s.railMore = s.rail(src, stampOf(raw))
	img, err := drawPage(src, s.at, render.Options{
		Scale:       s.fitScale(src),
		MaxDuration: pageBudget,
	})
	// A page that ran out of time comes back as far as it got, which is worth
	// showing: somebody scrolling would rather see most of a figure than a
	// sentence saying there was one. Anything else, and there is no picture.
	partial := errors.Is(err, render.ErrTimedOut) && img != nil
	if err != nil && !partial {
		// Through show, like every other thing the view band can hold: a page
		// that cannot be drawn is the moment somebody most needs the rail and
		// the panel, because what they want next is another page.
		s.show(toolkit.NewLabel("this page cannot be drawn: " + err.Error()))
		return
	}
	if partial {
		// Showing half a page without saying so would be the one thing worse
		// than showing nothing, so the status line says which it is. This is
		// the only place renderPage writes the note, and it overwrites
		// whatever was there, because what is on the screen right now matters
		// more than what happened before it.
		s.note = fmt.Sprintf("this page was still being drawn after %s; this is as far as it got", pageBudget)
	}
	s.page = toolkit.NewImageFit(img.Pix, img.W, img.H)
	s.show(s.page)
}

// show puts a widget in the view band, with the tool panel beside it when a
// group is open.
//
// Beside, and not over it: every verb in the panel changes the document, and
// the document is drawn from what would come out of Save — so setting a crop
// box and watching the page come back cropped is the whole of what the control
// is for, and a panel that covered the page would hide it.
func (s *state) show(w toolkit.Widget) {
	// Kept, because the rail fills in over several frames and each of those
	// frames has to put the band back together around whatever it is holding.
	s.shown = w
	// Wrapped in the toolkit's own popover host, which is what draws a list a
	// drop-down has opened on top of everything and offers it the next press
	// before anything under it sees one. Without it a list opens onto nothing:
	// the chevron works and no option can be chosen.
	s.view = toolkit.NewPopoverHost(s.arrange(w))
	// Laid out here rather than only when it is painted, because a press can
	// arrive before the next frame does: the view is built afresh by every
	// change, and a widget nobody has given bounds to is under no point at
	// all, so the press after a change would land on nothing.
	s.view.SetBounds(painter.Rect{X: margin, Y: viewTop(), W: s.viewW(), H: s.viewH()})
}

// arrange is the view band's contents: the page, and the tool panel beside it
// when a group is open.
func (s *state) arrange(w toolkit.Widget) toolkit.Widget {
	page := toolkit.NewFrame(w)
	if s.railView == nil && s.tools.open == "" {
		return page
	}
	// An HBox, which is what puts things side by side; the page takes whatever
	// the rail and the panel leave. The rail goes on the left, where a reader
	// looks first and where every document viewer has put it.
	row := toolkit.NewHBox()
	row.Spacing = gap
	if s.railView != nil {
		row.AddFixed(toolkit.NewFrame(s.railView), railW)
	}
	row.AddFlex(page, 1)
	if s.tools.open != "" {
		row.AddFixed(toolkit.NewFrame(s.body()), panelW)
	}
	return row
}

// pageW is how much width the page has: the whole band, less the panel when
// one is open. The page is scaled to fit what is left, so opening a group
// shrinks the page rather than pushing it off the edge.
// viewW and viewH are the band under the strip, in the surface's own pixels.
// They follow s.w and s.h so the workbench fills whatever it is given.
func (s *state) viewW() int { return s.w - 2*margin }
func (s *state) viewH() int { return s.h - viewTop() - statusH() - margin }

// resize lays the workbench out on a new surface. The size arrives in DEVICE
// pixels -- the caller has already multiplied by the screen's pixel ratio --
// because everything here is drawn by us into a framebuffer, and a framebuffer
// smaller than the screen is a framebuffer the browser has to stretch. That
// stretch is what made the text look soft.
//
// A surface too small to hold the bands is refused rather than laid out
// negative: a window dragged to nothing should leave the last good layout
// standing, not a frame of arithmetic errors.
func (s *state) resize(w, h int) bool {
	if w < minSurfaceW || h < minSurfaceH() {
		return false
	}
	if w == s.w && h == s.h {
		return false
	}
	s.w, s.h = w, h
	s.refresh()
	return true
}

func (s *state) pageW() int {
	w := s.viewW()
	if s.railView != nil {
		w -= railW + gap
	}
	if s.tools.open != "" {
		w -= panelW + gap
	}
	return w
}

// pageBudget is how long one page may be drawn for before what has been drawn
// so far is shown instead. A handful of pages take minutes — of the 59 432
// pages the renderer was measured against, 1 131 were still going after twenty
// seconds and one took two hundred and seventy-three — and a window that stops
// answering for that long reads as broken rather than as busy.
const pageBudget = 5 * time.Second

// fitScale is how much to magnify the page so that it fills the view without
// spilling out of it.
func (s *state) fitScale(src *reader.Document) float64 {
	// The page number is clamped before this is called, and pageSize always
	// falls back on a real paper size, so neither can go wrong here.
	page, _ := src.Page(s.at)
	w, h := pageSize(src, page)
	byWidth := float64(s.pageW()-2*margin) / w
	byHeight := float64(s.viewH()-2*margin) / h
	if byWidth < byHeight {
		return byWidth
	}
	return byHeight
}

// pageSize is how large a page is, in points, the way it will be shown.
func pageSize(src *reader.Document, page reader.Dict) (w, h float64) {
	box := [4]float64{0, 0, 612, 792}
	for _, key := range []reader.Name{"CropBox", "MediaBox"} {
		o, _ := src.Resolve(page.Get(key))
		arr, ok := reader.ToArray(o)
		if !ok || len(arr) < 4 {
			continue
		}
		good := true
		for i := 0; i < 4; i++ {
			e, _ := src.Resolve(arr[i])
			v, ok := reader.ToFloat(e)
			if !ok {
				good = false
				break
			}
			box[i] = v
		}
		if good {
			break
		}
	}
	w, h = box[2]-box[0], box[3]-box[1]
	if w < 0 {
		w = -w
	}
	if h < 0 {
		h = -h
	}
	rot, _ := reader.ToInt(mustResolve(src, page.Get("Rotate")))
	if (rot/90)%2 != 0 {
		w, h = h, w
	}
	return w, h
}

// mustResolve follows a reference; a document that opened cannot fail to.
func mustResolve(src *reader.Document, o reader.Object) reader.Object {
	out, _ := src.Resolve(o)
	return out
}

// draw paints the whole workbench.
func (s *state) draw(buf []byte) {
	fillBG(buf, s.theme.Background)
	p := painter.NewPixelPainter(buf, s.w, s.h)
	s.toolbar.SetBounds(painter.Rect{X: margin, Y: margin, W: s.w - 2*margin, H: toolbarH()})
	s.toolbar.Draw(p, s.theme)
	s.status.SetBounds(painter.Rect{X: 0, Y: s.h - statusH(), W: s.w, H: statusH()})
	s.status.Draw(p, s.theme)
	// The view last, because a list opened near the foot of the panel is drawn
	// over whatever is below it — and below it is the status line.
	s.view.SetBounds(painter.Rect{X: margin, Y: viewTop(), W: s.viewW(), H: s.viewH()})
	s.view.Draw(p, s.theme)
}

// pointer sends a press, a move or a release to the strip and to the view.
//
// Both, because the view is where every control that is not on the strip now
// lives — a panel that got no events would be a picture of controls rather
// than controls — and because a widget that is not under the point ignores
// what it is handed anyway.
func (s *state) pointer(kind toolkit.EventKind, x, y int) bool {
	view := s.view // a control may put another view in its place
	if kind == toolkit.EventClick {
		// Nothing under the pointer takes the caret away from the box that
		// has it. The toolkit moves focus on a click by walking the container
		// for its focusable descendants, and that walk cannot see into a
		// scroll view or through a form field — so a press on a second box
		// leaves the first one focused too, and every letter typed after it
		// goes into the first. Clearing them all first leaves exactly the one
		// this press lands in.
		s.settle()
	}
	hit(s.toolbar, kind, x, y)
	hit(view, kind, x, y)
	return true
}

// hit sends a pointer event to a widget in that widget's own coordinates.
//
// A container reads the point it is given as local to itself and adds its own
// origin back on before hit-testing its children, so what has to arrive is the
// press with that origin taken off. Handing over the surface point instead
// moves every press down the screen by the widget's own top edge: on a strip
// eight pixels from the top that is a near miss, and on a panel that starts
// forty-six pixels down it is a press on the wrong row.
func hit(w toolkit.Widget, kind toolkit.EventKind, x, y int) {
	b := w.Bounds()
	w.OnEvent(toolkit.Event{Kind: kind, X: x - b.X, Y: y - b.Y})
}

// handleClick routes a press to whatever is under it.
func (s *state) handleClick(x, y int) bool { return s.pointer(toolkit.EventClick, x, y) }

// handleMove routes a pointer move.
func (s *state) handleMove(x, y int) bool { return s.pointer(toolkit.EventMouseMove, x, y) }

// handleRelease routes a release.
func (s *state) handleRelease(x, y int) bool { return s.pointer(toolkit.EventMouseUp, x, y) }

// handleChar puts a printable character into the box being typed into; with
// nothing being typed into, the workbench has nowhere to put it.
func (s *state) handleChar(text string) bool {
	return s.toCaret(toolkit.Event{Kind: toolkit.EventChar, Code: text})
}

// toCaret hands a keystroke to the box that has the caret, and reports whether
// there was one to hand it to.
//
// The box is addressed directly rather than through the toolkit's own focus
// walk, because that walk cannot reach it: it descends through a widget only
// when the widget can enumerate its focusable children, and neither ScrollView
// nor FormField does — so a control inside a scrolling panel, which is where
// every control here lives, is invisible to it. The workbench knows which
// boxes it built and which one was last pressed, so it says so.
func (s *state) toCaret(ev toolkit.Event) bool {
	e := s.caret()
	if e == nil {
		return false
	}
	e.OnEvent(ev)
	s.dirty = true
	return true
}

// caret is the box being typed into, or nil when nothing is. It is what
// decides who an arrow key belongs to: a word somebody is in the middle of
// writing, or the pages.
func (s *state) caret() *toolkit.Entry {
	for _, e := range s.typing {
		if e.Focused() {
			return e
		}
	}
	return nil
}

// editing reports whether anything is being typed into.
func (s *state) editing() bool { return s.caret() != nil }

// handleKeyDown moves between pages with the arrow keys, unless something is
// being typed into, in which case the key belongs to that.
func (s *state) handleKeyDown(key string) bool {
	if s.toCaret(toolkit.Event{Kind: toolkit.EventKeyDown, Code: key}) {
		return true
	}
	switch key {
	case "ArrowLeft", "PageUp":
		s.step(-1)
		return true
	case "ArrowRight", "PageDown":
		s.step(1)
		return true
	}
	return false
}

// fillBG paints the whole buffer one colour, which is what a scene starts
// from: the widgets draw over it.
func fillBG(buf []byte, c toolkit.RGBA) {
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = c.R, c.G, c.B, c.A
	}
}
