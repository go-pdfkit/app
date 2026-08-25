// The workbench itself: what is on the screen, and what each control does to
// the document. Kept free of any build tag so a native test can drive the
// whole thing against a plain byte buffer, which is how the behaviour here is
// checked without a browser.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
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
)

// Geometry of the three bands: a toolbar, the page, a status line.
const (
	margin   = 8
	toolbarH = 30
	statusH  = 22
	viewTop  = margin + toolbarH + margin
	viewH    = surfaceH - viewTop - statusH - margin
	viewW    = surfaceW - 2*margin
)

// A state is the whole workbench.
type state struct {
	w, h  int
	theme *toolkit.Theme
	host  host

	toolbar *toolkit.HBox
	status  *toolkit.Statusbar
	view    *toolkit.Frame
	page    *toolkit.Image
	empty   *toolkit.Label

	// doc is what every operation acts on, and src is the same document
	// parsed, which is what gets drawn. They are rebuilt together after every
	// change, so what is shown is always what would be saved.
	doc  *ops.Doc
	src  *reader.Document
	name string
	at   int // the page being shown, counting from one
	note string

	// dirty says something has changed since the canvas last showed it. A
	// file arrives from the browser long after the press that asked for it,
	// so there is no event left to repaint on; the harness asks here instead.
	dirty bool
}

// newState builds the workbench.
func newState(w, h int, h2 host) *state {
	s := &state{w: w, h: h, theme: toolkit.DefaultLight(), host: h2, at: 1}
	s.empty = toolkit.NewLabel("Open a PDF to begin — nothing leaves this tab.")
	s.view = toolkit.NewFrame(s.empty)
	s.status = toolkit.NewStatusbar([]string{"no document", "", ""})
	s.toolbar = s.tools()
	s.refresh()
	return s
}

// tools is every control on the strip, in the order they appear. They are
// buttons rather than a Toolbar because a Toolbar is a strip of square icon
// cells that shows only a label's first letter, and these controls are named
// by their words: "Two up" and "Sanitize" both begin with the letter the other
// would be reduced to.
func (s *state) tools() *toolkit.HBox {
	box := toolkit.NewHBox()
	add := func(label string, style toolkit.ButtonStyle, on func()) {
		b := toolkit.NewButton(label, on)
		b.Style = style
		box.AddFixed(b, buttonWidth(label))
	}
	add("Open", toolkit.ButtonProminent, s.open)
	add("Save", toolkit.ButtonProminent, s.save)
	add("<", toolkit.ButtonDefault, func() { s.step(-1) })
	add(">", toolkit.ButtonDefault, func() { s.step(1) })
	add("Rotate", toolkit.ButtonDefault, s.rotate)
	add("Delete", toolkit.ButtonDanger, s.deletePage)
	add("Two up", toolkit.ButtonDefault, s.twoUp)
	add("Watermark", toolkit.ButtonDefault, s.watermark)
	add("Sanitize", toolkit.ButtonDefault, s.sanitize)
	return box
}

// buttonWidth is wide enough for the whole of a label, whatever the font in
// force measures it at, with room either side.
func buttonWidth(label string) int {
	w := toolkit.TextWidth(label) + 2*buttonPadding
	if w < minButtonW {
		w = minButtonW
	}
	return w
}

// How much room a control keeps around its own name.
const (
	buttonPadding = 12
	minButtonW    = 28
)

// open asks for a file and takes it as the document.
func (s *state) open() {
	s.host.Open(func(name string, data []byte) {
		d, err := ops.Open(data)
		if err != nil {
			s.fail("cannot open " + name + ": " + err.Error())
			return
		}
		s.doc, s.name, s.at = d, name, 1
		s.note = ""
		s.refresh()
	})
}

// save hands the document back, as it now stands.
func (s *state) save() {
	if s.doc == nil {
		s.fail("there is nothing to save")
		return
	}
	out, msg := s.reopenBytes()
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

// twoUp lays the pages out two to a sheet.
func (s *state) twoUp() {
	s.change(func(d *ops.Doc) error { return d.NUp(2) })
	s.at = 1
	s.refresh()
}

// watermark writes across every page.
func (s *state) watermark() {
	s.change(func(d *ops.Doc) error { return d.Watermark("all", "DRAFT") })
}

// sanitize strips whatever in the file runs rather than shows.
func (s *state) sanitize() {
	s.change(func(d *ops.Doc) error {
		d.Sanitize()
		return nil
	})
}

// pageSpec names one page the way a range is written.
func pageSpec(at int) string { return fmt.Sprintf("%d", at) }

// change applies an operation and shows the result, or says why it could not.
func (s *state) change(apply func(*ops.Doc) error) {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	if err := apply(s.doc); err != nil {
		s.fail(err.Error())
		return
	}
	s.note = ""
	s.refresh()
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
	s.renderPage()
	s.status = toolkit.NewStatusbar(s.statusLine())
	s.dirty = true
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
	return []string{s.name, where, s.note}
}

// reopenBytes is the document as it would be saved, or the reason it cannot
// be written. It reports that reason rather than an error, because the only
// thing to do with it is show it.
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
	out, msg := s.reopenBytes()
	if msg != "" {
		return nil, msg
	}
	src, err := openBytes(out)
	if err != nil {
		return nil, "this document cannot be read back: " + err.Error()
	}
	return src, ""
}

// These three are variables so a test can watch what the workbench does when
// writing, re-reading or drawing a document it is already holding goes wrong.
// None of the three should ever fail on a document that opened, which is
// exactly why it is worth being able to see what happens when one does.
var (
	docBytes  = (*ops.Doc).Bytes
	openBytes = reader.Open
	drawPage  = render.Page
)

// renderPage draws the current page into the view, or leaves the invitation
// there when there is nothing to draw.
func (s *state) renderPage() {
	s.page = nil
	if s.doc == nil {
		s.view = toolkit.NewFrame(s.empty)
		return
	}
	src, msg := s.reopen()
	if msg != "" {
		s.view = toolkit.NewFrame(toolkit.NewLabel(msg))
		return
	}
	s.src = src
	if s.at > src.PageCount() {
		s.at = src.PageCount()
	}
	if s.at < 1 {
		s.at = 1
	}
	img, err := drawPage(src, s.at, render.Options{Scale: s.fitScale(src)})
	if err != nil {
		s.view = toolkit.NewFrame(toolkit.NewLabel("this page cannot be drawn: " + err.Error()))
		return
	}
	s.page = toolkit.NewImageFit(img.Pix, img.W, img.H)
	s.view = toolkit.NewFrame(s.page)
}

// fitScale is how much to magnify the page so that it fills the view without
// spilling out of it.
func (s *state) fitScale(src *reader.Document) float64 {
	// The page number is clamped before this is called, and pageSize always
	// falls back on a real paper size, so neither can go wrong here.
	page, _ := src.Page(s.at)
	w, h := pageSize(src, page)
	byWidth := float64(viewW-2*margin) / w
	byHeight := float64(viewH-2*margin) / h
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
	s.toolbar.SetBounds(painter.Rect{X: margin, Y: margin, W: s.w - 2*margin, H: toolbarH})
	s.toolbar.Draw(p, s.theme)
	s.view.SetBounds(painter.Rect{X: margin, Y: viewTop, W: viewW, H: viewH})
	s.view.Draw(p, s.theme)
	s.status.SetBounds(painter.Rect{X: 0, Y: s.h - statusH, W: s.w, H: statusH})
	s.status.Draw(p, s.theme)
}

// handleClick routes a press to whatever is under it.
func (s *state) handleClick(x, y int) bool {
	s.toolbar.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: x, Y: y})
	return true
}

// handleMove routes a pointer move.
func (s *state) handleMove(x, y int) bool {
	s.toolbar.OnEvent(toolkit.Event{Kind: toolkit.EventMouseMove, X: x, Y: y})
	return true
}

// handleRelease routes a release.
func (s *state) handleRelease(x, y int) bool {
	s.toolbar.OnEvent(toolkit.Event{Kind: toolkit.EventMouseUp, X: x, Y: y})
	return true
}

// handleKeyDown moves between pages with the arrow keys.
func (s *state) handleKeyDown(key string) bool {
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
