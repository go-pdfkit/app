package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// fakeHost stands in for the browser: it hands over a document that was made
// here, and keeps whatever the workbench saves.
type fakeHost struct {
	name  string
	file  []byte
	saved []byte
	as    string
	asked int
	// savedAll counts everything handed back, which is what says a verb that
	// produces several files produced them all.
	savedAll int
}

func (h *fakeHost) Open(done func(string, []byte)) {
	h.asked++
	if h.file == nil {
		return // the person changed their mind, which is not an error
	}
	done(h.name, h.file)
}

func (h *fakeHost) Save(name string, data []byte) {
	h.as, h.saved = name, data
	h.savedAll++
}

// samplePDF builds a document of n pages, each carrying a black square and its
// own number, so a rendered page can be told from a blank one.
func samplePDF(t *testing.T, n int) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	kids := make(reader.Array, 0, n)
	for i := 1; i <= n; i++ {
		content := w.Add(&reader.Stream{Dict: reader.Dict{},
			Raw: []byte("0 g 20 20 200 200 re f")})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count":    reader.Integer(n),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// buffer is a canvas of the workbench's size.
func buffer() []byte { return make([]byte, surfaceW*surfaceH*4) }

// inked counts the pixels of a drawn buffer that are not the background.
func inked(buf []byte, bg toolkit.RGBA) int {
	n := 0
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != bg.R || buf[i+1] != bg.G || buf[i+2] != bg.B {
			n++
		}
	}
	return n
}

// opened builds a workbench with a document already in it.
func opened(t *testing.T, pages int) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "sample.pdf", file: samplePDF(t, pages)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	return s, h
}

func TestAnEmptyWorkbenchInvitesAFile(t *testing.T) {
	h := &fakeHost{}
	s := newState(surfaceW, surfaceH, h)
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing was drawn at all")
	}
	if s.statusLine()[0] != "no document" {
		t.Errorf("the status line says %q", s.statusLine()[0])
	}
	// Asking for a file the person does not give leaves the workbench as it
	// was, rather than in some half-open state.
	s.open()
	if h.asked != 1 || s.doc != nil {
		t.Errorf("asked %d times, document %v", h.asked, s.doc != nil)
	}
}

func TestOpeningADocumentDrawsItsFirstPage(t *testing.T) {
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	before := buffer()
	empty.draw(before)

	s, _ := opened(t, 3)
	after := buffer()
	s.draw(after)
	if inked(after, s.theme.Background) <= inked(before, s.theme.Background) {
		t.Error("opening a document did not put more on the screen")
	}
	if s.statusLine()[1] != "page 1 of 3" {
		t.Errorf("the status line says %q", s.statusLine()[1])
	}
}

func TestOpeningSomethingThatIsNotAPDF(t *testing.T) {
	h := &fakeHost{name: "notes.txt", file: []byte("this is not a PDF")}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc != nil {
		t.Error("a document was opened from bytes that are not one")
	}
	if s.note == "" {
		t.Error("nothing was said about why")
	}
	// And the workbench still draws.
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing was drawn")
	}
}

func TestTurningPages(t *testing.T) {
	s, _ := opened(t, 3)
	s.step(1)
	if s.at != 2 {
		t.Errorf("at page %d", s.at)
	}
	s.step(-1)
	if s.at != 1 {
		t.Errorf("at page %d", s.at)
	}
	// Neither end moves past itself.
	s.step(-1)
	if s.at != 1 {
		t.Errorf("stepped before the first page, to %d", s.at)
	}
	s.at = 3
	s.step(1)
	if s.at != 3 {
		t.Errorf("stepped past the last page, to %d", s.at)
	}
	// The arrow keys do the same.
	if !s.handleKeyDown("ArrowRight") || s.at != 3 {
		t.Errorf("the right arrow took it to %d", s.at)
	}
	s.at = 1
	if !s.handleKeyDown("ArrowRight") || s.at != 2 {
		t.Errorf("the right arrow took it to %d", s.at)
	}
	if !s.handleKeyDown("ArrowLeft") || s.at != 1 {
		t.Errorf("the left arrow took it to %d", s.at)
	}
	if !s.handleKeyDown("PageDown") || s.at != 2 {
		t.Errorf("page down took it to %d", s.at)
	}
	if !s.handleKeyDown("PageUp") || s.at != 1 {
		t.Errorf("page up took it to %d", s.at)
	}
	if s.handleKeyDown("KeyQ") {
		t.Error("a key with nothing to do claimed the event")
	}
	// And turning pages with nothing open does nothing.
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	empty.step(1)
	if empty.at != 1 {
		t.Errorf("an empty workbench stepped to %d", empty.at)
	}
}

func TestRotatingThePageOnTheScreen(t *testing.T) {
	s, _ := opened(t, 2)
	s.rotate()
	if s.note != "" {
		t.Fatalf("rotating said %q", s.note)
	}
	got, err := s.doc.Rotation(1)
	if err != nil || got != 90 {
		t.Errorf("page one is turned %d degrees, %v", got, err)
	}
	if turned, _ := s.doc.Rotation(2); turned != 0 {
		t.Errorf("page two was turned too, to %d", turned)
	}
	// A turned page comes out the other way round on the screen.
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing was drawn after turning the page")
	}
}

func TestDeletingThePageOnTheScreen(t *testing.T) {
	s, _ := opened(t, 3)
	s.at = 2
	s.deletePage()
	if s.doc.PageCount() != 2 {
		t.Errorf("%d pages are left", s.doc.PageCount())
	}
	// Deleting the last page steps back rather than pointing past the end.
	s.at = 2
	s.deletePage()
	if s.doc.PageCount() != 1 || s.at != 1 {
		t.Errorf("%d pages left, showing page %d", s.doc.PageCount(), s.at)
	}
	// The last page of all is kept: a document needs one.
	s.deletePage()
	if s.doc.PageCount() != 1 {
		t.Errorf("the last page was deleted")
	}
	if s.note == "" {
		t.Error("nothing was said about why")
	}
}

func TestLayingPagesOutTwoUp(t *testing.T) {
	s, _ := opened(t, 4)
	s.tools.up = 2
	s.nUp()
	if s.doc.PageCount() != 2 {
		t.Errorf("%d sheets", s.doc.PageCount())
	}
	if s.at != 1 {
		t.Errorf("showing sheet %d", s.at)
	}
}

func TestWatermarkingAndSanitising(t *testing.T) {
	s, _ := opened(t, 2)
	s.watermark()
	if !strings.Contains(s.note, "DRAFT") {
		t.Errorf("watermarking said %q", s.note)
	}
	s.sanitize()
	if s.note == "" {
		t.Error("sanitising said nothing for itself")
	}
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// The mark is in a compressed stream, so the file is read back rather
	// than searched.
	back, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	content, err := back.PageContent(1)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(content, "(DRAFT) Tj") {
		t.Error("the watermark is not on the page")
	}
}

// contains reports whether a file holds a piece of text.
func contains(data []byte, s string) bool {
	for i := 0; i+len(s) <= len(data); i++ {
		if string(data[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}

func TestSavingHandsBackAReadableDocument(t *testing.T) {
	s, h := opened(t, 3)
	s.at = 2
	s.deletePage()
	s.save()
	if h.as != "sample-edited.pdf" {
		t.Errorf("saved as %q", h.as)
	}
	back, err := reader.Open(h.saved)
	if err != nil {
		t.Fatalf("what was saved does not open: %v", err)
	}
	if back.PageCount() != 2 {
		t.Errorf("the saved document has %d pages", back.PageCount())
	}
	if s.note == "" {
		t.Error("nothing was said about the save")
	}
}

func TestSavingWithNothingOpen(t *testing.T) {
	h := &fakeHost{}
	s := newState(surfaceW, surfaceH, h)
	s.save()
	if h.saved != nil {
		t.Error("something was saved")
	}
	if s.note == "" {
		t.Error("nothing was said about why")
	}
}

func TestTheNameADocumentIsSavedUnder(t *testing.T) {
	for in, want := range map[string]string{
		"report.pdf": "report-edited.pdf",
		"report":     "report-edited.pdf",
		"":           "document.pdf",
		"a.PDF":      "a.PDF-edited.pdf",
	} {
		if got := saveName(in); got != want {
			t.Errorf("saveName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEveryControlIsReachableByClicking(t *testing.T) {
	// The workbench is driven by pressing the strip at the top; a press
	// somewhere along it has to reach a control rather than fall through.
	s, _ := opened(t, 3)
	buf := buffer()
	s.draw(buf) // lays the toolbar out, which is what gives it bounds
	reached := 0
	for x := margin; x < surfaceW-margin; x += 4 {
		before := s.doc.PageCount()
		at := s.at
		s.handleClick(x, margin+toolbarH/2)
		if s.doc.PageCount() != before || s.at != at || s.note != "" {
			reached++
		}
		s.note = ""
	}
	if reached == 0 {
		t.Error("no press anywhere along the strip reached a control")
	}
	// A press well below the strip reaches nothing, and does not break.
	s.handleClick(surfaceW/2, surfaceH-30)
	if !s.handleMove(10, 10) || !s.handleRelease(10, 10) {
		t.Error("a move or a release was not taken")
	}
}

func TestAControlWithNothingOpenSaysSo(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	for _, act := range []func(){s.rotate, s.deletePage, s.nUp, s.watermark, s.sanitize} {
		s.note = ""
		act()
		if s.note == "" {
			t.Error("a control with no document said nothing")
		}
	}
}

func TestADocumentThatCannotBeDrawn(t *testing.T) {
	// A page whose content is filtered as an image cannot be drawn; the
	// workbench says so rather than showing nothing and no reason.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(100), reader.Integer(100)},
		"Contents": w.Add(&reader.Stream{
			Dict: reader.Dict{"Filter": reader.Name("DCTDecode")}, Raw: []byte("not a jpeg")}),
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{name: "odd.pdf", file: out}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatal("the document did not open")
	}
	if s.page != nil {
		t.Error("a page was drawn from content that cannot be read")
	}
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing at all was drawn")
	}
}

func TestPagesOfEveryShape(t *testing.T) {
	// The view has to fit a page whatever shape it is, and a page that says
	// nothing about its own size is still shown.
	cases := []struct {
		name string
		box  reader.Object
		rot  reader.Object
	}{
		{"tall", reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(800)}, nil},
		{"wide", reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(800), reader.Integer(200)}, nil},
		{"turned", reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(800)}, reader.Integer(90)},
		{"turned twice", reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(800)}, reader.Integer(180)},
		{"a box that is not one", reader.Array{reader.Name("x")}, nil},
		{"no box at all", nil, nil},
		{"corners the wrong way round", reader.Array{reader.Integer(300), reader.Integer(400), reader.Integer(0), reader.Integer(0)}, nil},
	}
	for _, c := range cases {
		w := reader.NewWriter("1.7")
		pagesRef := w.Reserve()
		page := reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef,
			"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 0 0 50 50 re f")}),
		}
		if c.box != nil {
			page["MediaBox"] = c.box
		}
		if c.rot != nil {
			page["Rotate"] = c.rot
		}
		w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
			"Kids": reader.Array{w.Add(page)}, "Count": reader.Integer(1)})
		root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
		out, err := w.Finish(reader.Dict{"Root": root})
		if err != nil {
			t.Fatal(err)
		}
		s := newState(surfaceW, surfaceH, &fakeHost{name: "p.pdf", file: out})
		s.open()
		if s.doc == nil {
			t.Fatalf("%s: the document did not open", c.name)
		}
		buf := buffer()
		s.draw(buf)
		if inked(buf, s.theme.Background) == 0 {
			t.Errorf("%s: nothing was drawn", c.name)
		}
	}
}

func TestShowingAPageThatIsNoLongerThere(t *testing.T) {
	// Nothing outside can set the page number, but a document that shrinks
	// under one must still draw: the workbench clamps rather than reaching
	// past the end.
	s, _ := opened(t, 3)
	s.at = 9
	s.refresh()
	if s.at != 3 {
		t.Errorf("showing page %d of three", s.at)
	}
	s.at = 0
	s.refresh()
	if s.at != 1 {
		t.Errorf("showing page %d", s.at)
	}
}

func TestWhenTheDocumentInHandCannotBeWritten(t *testing.T) {
	// Nothing a person can do on the strip should leave a document that will
	// not write, but if one ever did, the workbench has to say so on the
	// status line and in the view rather than show a stale page or nothing.
	s, h := opened(t, 2)
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errors.New("the ink ran out") }

	s.save()
	if !strings.Contains(s.note, "the ink ran out") {
		t.Errorf("saving said %q", s.note)
	}
	if h.saved != nil {
		t.Error("a document that cannot be written was handed back anyway")
	}
	s.refresh()
	if s.page != nil {
		t.Error("a page was drawn from a document that cannot be written")
	}
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("the reason was not drawn")
	}
}

func TestWhenTheDocumentJustWrittenCannotBeReadBack(t *testing.T) {
	// The workbench draws what it would save, so it reads its own output
	// back. A file it has just written should always open again; if one did
	// not, the view says which of the two steps went wrong.
	s, _ := opened(t, 2)
	was := openBytes
	t.Cleanup(func() { openBytes = was })
	openBytes = func([]byte) (*reader.Document, error) { return nil, errors.New("not a PDF after all") }

	s.refresh()
	if s.page != nil {
		t.Error("a page was drawn from a document that cannot be read back")
	}
	src, msg := s.reopen()
	if src != nil || !strings.Contains(msg, "read back") {
		t.Errorf("reopening said %q", msg)
	}
}

func TestDeletingTheLastPageWhileItIsTheOneShown(t *testing.T) {
	// Deleting the page at the end leaves the number pointing past the end,
	// so the workbench steps back to the new last page rather than to
	// nothing.
	s, _ := opened(t, 3)
	s.at = 3
	s.refresh()
	s.deletePage()
	if s.doc.PageCount() != 2 {
		t.Fatalf("%d pages left", s.doc.PageCount())
	}
	if s.at != 2 {
		t.Errorf("showing page %d of two", s.at)
	}
	if s.page == nil {
		t.Error("nothing is being shown")
	}
}

func TestAnOperationThatWillNotGoThrough(t *testing.T) {
	// Every control on the strip hands its work to change, and change is
	// what puts the reason on the status line. No verb on the strip can fail
	// on a document that opened, so the failure is handed in here directly.
	s, _ := opened(t, 2)
	s.change(func(*ops.Doc) error { return errors.New("that page is not there") })
	if !strings.Contains(s.note, "that page is not there") {
		t.Errorf("the status line said %q", s.note)
	}
}

func TestAPageBoxWithSomethingThatIsNotANumberInIt(t *testing.T) {
	// A box of the right length whose corners are not all numbers is not a
	// box: the page falls back on the paper size rather than on nonsense.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(100), reader.Name("tall")},
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 0 0 50 50 re f")}),
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	s := newState(surfaceW, surfaceH, &fakeHost{name: "odd-box.pdf", file: out})
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing was drawn")
	}
}

func TestAControlIsWideEnoughForItsName(t *testing.T) {
	// A control is as wide as the whole of its name plus room either side,
	// with a floor so that a one-character one is still something to press.
	long := buttonWidth("Watermark")
	short := buttonWidth("<")
	if long <= short {
		t.Errorf("Watermark is %d wide and < is %d", long, short)
	}
	if got := buttonWidth(""); got != minButtonW {
		t.Errorf("a nameless control is %d wide, want the floor of %d", got, minButtonW)
	}
	if long < toolkit.TextWidth("Watermark") {
		t.Errorf("Watermark does not fit in %d", long)
	}
	// The whole strip fits across the surface, which is what keeps a control
	// from being pushed off the end of it.
	total := 0
	for _, label := range []string{"Open", "Save", "<", ">", "Rotate", "Delete", "Two up", "Watermark", "Sanitize"} {
		total += buttonWidth(label)
	}
	if total > surfaceW-2*margin {
		t.Errorf("the strip is %d wide and the surface is %d", total, surfaceW-2*margin)
	}
}

func TestAPageThatRanOutOfTimeIsShownAsFarAsItGot(t *testing.T) {
	// The renderer is given a budget, and a page that overruns it comes back
	// half drawn together with render.ErrTimedOut. Half a page is worth more
	// to somebody scrolling than a sentence saying there was one, so the
	// workbench shows it rather than the error.
	s, _ := opened(t, 1)
	was := drawPage
	t.Cleanup(func() { drawPage = was })
	var budget time.Duration
	drawPage = func(_ *reader.Document, _ int, opt render.Options) (*raster.Image, error) {
		budget = opt.MaxDuration
		return raster.New(4, 4), render.ErrTimedOut
	}

	s.refresh()
	if budget <= 0 {
		t.Error("the renderer was given no budget at all")
	}
	if s.page == nil {
		t.Fatal("the part of the page that was drawn was thrown away")
	}
	if !strings.Contains(s.note, "as far as it got") {
		t.Errorf("half a page was shown without saying so; the status line said %q", s.note)
	}
	if got := s.statusLine(); !strings.Contains(strings.Join(got, "|"), "as far as it got") {
		t.Errorf("the status line does not carry it: %q", got)
	}
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("nothing reached the screen")
	}
}

func TestAPageThatRanOutOfTimeWithNothingDrawnSaysSo(t *testing.T) {
	// Coming back timed out and empty is not something the renderer does,
	// but if it did there is no picture to show, so the view has to fall
	// back on saying why rather than dereferencing nothing.
	s, _ := opened(t, 1)
	was := drawPage
	t.Cleanup(func() { drawPage = was })
	drawPage = func(*reader.Document, int, render.Options) (*raster.Image, error) {
		return nil, render.ErrTimedOut
	}

	s.refresh()
	if s.page != nil {
		t.Error("a page was drawn from nothing")
	}
	buf := buffer()
	s.draw(buf)
	if inked(buf, s.theme.Background) == 0 {
		t.Error("the reason was not drawn")
	}
}
