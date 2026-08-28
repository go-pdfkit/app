package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// The heights of the rows of the groups this file drives.
var (
	marksRows = []int{labelledH, labelledH, bareH, labelledH, bareH,
		labelledH, labelledH, bareH, labelledH, labelledH, bareH}
	fileRows    = []int{bareH, bareH, bareH, bareH, bareH, bareH, labelledH, labelledH, bareH}
	protectRows = []int{labelledH, bareH, labelledH, labelledH,
		bareH, bareH, bareH, bareH, bareH, bareH, bareH, bareH}
	readRows = []int{bareH, bareH, bareH, bareH}
)

// content is the first page of a document as it would be saved.
func content(t *testing.T, d *ops.Doc) []byte {
	t.Helper()
	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	back, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	c, err := back.PageContent(1)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// typeInto puts the caret in the box on a row and types a word into it.
//
// The caret is sent to the end first. Since toolkit v0.273.0 a press puts the
// caret WHERE IT LANDS rather than after the last letter — which is what a text
// box should do — and these rows are pressed in the middle, which for a box
// that already holds a default lands before the text. Somebody adding to what
// is there goes to the end first, and so does this.
func typeInto(t *testing.T, s *state, rows []int, n int, word string) {
	t.Helper()
	x, y := rowAt(t, s, rows, n, 1)
	press(s, x, y)
	if !s.editing() {
		t.Fatalf("pressing row %d did not put the caret in a box", n)
	}
	s.toCaret(toolkit.Event{Kind: toolkit.EventKeyDown, Code: "End"})
	for _, c := range strings.Split(word, "") {
		if !s.handleChar(c) {
			t.Fatalf("row %d refused a character", n)
		}
	}
}

func TestTheMarksPanelWritesWhatWasTypedWhereItWasAsked(t *testing.T) {
	s, _ := opened(t, 3)
	openGroup(t, s, groupMarks)

	// A range of its own, and a watermark in words somebody chose.
	typeInto(t, s, marksRows, 0, "1")
	typeInto(t, s, marksRows, 1, "!")
	x, y := rowAt(t, s, marksRows, 2, 1)
	press(s, x, y)
	if !bytes.Contains(content(t, s.doc), []byte("(DRAFT!) Tj")) {
		t.Error("the watermark is not on the first page")
	}
	if s.doc.PageCount() != 3 {
		t.Errorf("watermarking changed the page count to %d", s.doc.PageCount())
	}

	// Page numbers, in a shape somebody chose.
	typeInto(t, s, marksRows, 3, "!")
	x, y = rowAt(t, s, marksRows, 4, 1)
	press(s, x, y)
	if !bytes.Contains(content(t, s.doc), []byte("(1 / 3!) Tj")) {
		t.Error("the page number is not on the first page")
	}

	// A Bates number, padded and started where the numbers say.
	typeInto(t, s, marksRows, 5, "AB")
	x, y = rowAt(t, s, marksRows, 6, 0)
	press(s, x+51, y-8) // the + of the left of the two numbers on that row
	px, py := rowAt(t, s, marksRows, 6, 1)
	press(s, px+120, py-8) // and the + of the right one
	if s.tools.digits != 7 {
		t.Fatalf("the Bates number is padded to %d digits", s.tools.digits)
	}
	s.tools.digits = 6
	if s.tools.start != 2 {
		t.Fatalf("the Bates number starts at %d", s.tools.start)
	}
	x, y = rowAt(t, s, marksRows, 7, 1)
	press(s, x, y)
	if !bytes.Contains(content(t, s.doc), []byte("(AB000002) Tj")) {
		t.Errorf("the Bates number is not on the first page: %q", s.note)
	}

	// A stamp, where the list says.
	typeInto(t, s, marksRows, 8, "?")
	x, y = rowAt(t, s, marksRows, 9, 1)
	press(s, x+120, y-8) // the + of the point size, which shares the row
	if s.tools.size != 13 {
		t.Fatalf("the stamp is %d points", s.tools.size)
	}
	x, y = rowAt(t, s, marksRows, 9, 0)
	press(s, x, y) // opens the list
	press(s, x, y+40)
	if s.tools.at == 0 {
		t.Fatal("no place was chosen from the list")
	}
	x, y = rowAt(t, s, marksRows, 10, 1)
	press(s, x, y)
	if !bytes.Contains(content(t, s.doc), []byte("(COPY?) Tj")) {
		t.Error("the stamp is not on the first page")
	}
	if !strings.Contains(s.note, places[s.tools.at].name) {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestAMarkOnEveryPageWhenNoRangeIsGiven(t *testing.T) {
	s, _ := opened(t, 2)
	if s.marked() != "all" {
		t.Errorf("with nothing typed the range is %q", s.marked())
	}
	s.tools.markSpec = " 2 "
	if s.marked() != " 2 " {
		t.Errorf("with a range typed it is %q", s.marked())
	}
	// A place that is not one falls back on the middle rather than reaching
	// past the end of the list.
	s.tools.at = 99
	s.stamp()
	if !strings.Contains(s.note, places[0].name) {
		t.Errorf("the status line says %q", s.note)
	}
}

func TestTheFilePanel(t *testing.T) {
	s, _ := opened(t, 3)
	openGroup(t, s, groupFile)
	for _, n := range []int{0, 1, 2, 3, 4} {
		x, y := rowAt(t, s, fileRows, n, 1)
		press(s, x, y)
		if s.note == "" {
			t.Errorf("the control on row %d said nothing for itself", n)
		}
	}
	// Packing says how much smaller the file came out.
	x, y := rowAt(t, s, fileRows, 5, 1)
	press(s, x, y)
	if !strings.Contains(s.note, "rather than") {
		t.Errorf("packing said %q", s.note)
	}

	// A title and an author, which are what a file says about itself.
	typeInto(t, s, fileRows, 6, "T")
	typeInto(t, s, fileRows, 7, "A")
	x, y = rowAt(t, s, fileRows, 8, 1)
	press(s, x, y)
	out, err := s.doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	back, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	info, ok := back.GetDict(back.Trailer(), "Info")
	if !ok {
		t.Fatal("the file says nothing about itself")
	}
	if got, _ := reader.ToString(mustResolve(back, info.Get("Title"))); string(got) != "T" {
		t.Errorf("the title is %q", got)
	}
	if got, _ := reader.ToString(mustResolve(back, info.Get("Author"))); string(got) != "A" {
		t.Errorf("the author is %q", got)
	}
}

func TestHowLargeAFileThatCannotBeWrittenIs(t *testing.T) {
	// written is asked before and after packing; a document that cannot be
	// written at all has no size, and the redraw says so for itself.
	s, _ := opened(t, 2)
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errors.New("no") }
	if n := s.written(); n != 0 {
		t.Errorf("a document that cannot be written came to %d bytes", n)
	}
	s.compress()
	if strings.Contains(s.note, "rather than") {
		t.Errorf("packing claimed a size it could not measure: %q", s.note)
	}
	// What went wrong is on the screen where the page was, which says it
	// better than a pair of zeroes on the status line would.
	if s.note != "packed" {
		t.Errorf("packing said %q", s.note)
	}
}

func TestProtectingAFileAndTakingItOffAgain(t *testing.T) {
	s, h := opened(t, 2)
	openGroup(t, s, groupProtect)

	// Nothing to protect it with is said rather than done.
	x, y := rowAt(t, s, protectRows, 9, 1)
	press(s, x, y)
	if !strings.Contains(s.note, "needs a user password") {
		t.Errorf("protecting with no password said %q", s.note)
	}

	// A password, typed into a box that shows dots rather than letters.
	typeInto(t, s, protectRows, 2, "shh")
	typeInto(t, s, protectRows, 3, "owner")
	if s.tools.userPw != "shh" || s.tools.ownerPw != "owner" {
		t.Fatalf("the boxes hold %q and %q", s.tools.userPw, s.tools.ownerPw)
	}
	// One of the permissions taken away.
	x, y = rowAt(t, s, protectRows, 6, 0)
	press(s, x, y)
	if s.tools.allow["copy text out of it"] {
		t.Error("the tick did not come off")
	}

	x, y = rowAt(t, s, protectRows, 9, 1)
	press(s, x, y)
	if !strings.Contains(s.note, "AES-256") {
		t.Fatalf("protecting said %q", s.note)
	}
	// The page is still drawn, because the workbench reads its own output
	// back with the password it was just given.
	if s.page == nil {
		t.Error("a protected document was not drawn")
	}
	s.save()
	if _, err := reader.Open(h.saved); err == nil {
		t.Error("what was saved opens with no password")
	}
	if _, err := reader.OpenWithPassword(h.saved, "owner"); err != nil {
		t.Errorf("the owner password does not open what was saved: %v", err)
	}
	back, err := reader.OpenWithPassword(h.saved, "shh")
	if err != nil {
		t.Fatalf("what was saved does not open with the password: %v", err)
	}
	p, ok := back.Protection()
	if !ok || p.Permissions.Allows(reader.PermCopy) {
		t.Errorf("protected %v, allowing %s", ok, p.Permissions)
	}

	// And taken off again.
	x, y = rowAt(t, s, protectRows, 10, 1)
	press(s, x, y)
	s.save()
	if _, err := reader.Open(h.saved); err != nil {
		t.Errorf("what was saved after the protection came off does not open: %v", err)
	}
}

func TestWhatAFileIsProtectedWith(t *testing.T) {
	// A document opened from a protected file says how it was protected; one
	// opened from a file that was not says that instead.
	plain, _ := opened(t, 2)
	openGroup(t, plain, groupProtect)
	x, y := rowAt(t, plain, protectRows, 11, 1)
	press(plain, x, y)
	if !strings.Contains(plain.note, "not protected") {
		t.Errorf("it said %q", plain.note)
	}

	locked := lockedPDF(t, "shh")
	s := newState(surfaceW, surfaceH, &fakeHost{name: "locked.pdf", file: locked})
	openGroup(t, s, groupProtect)
	// Without the password it will not open at all.
	s.open()
	if s.doc != nil || s.note == "" {
		t.Errorf("a protected file opened with no password, saying %q", s.note)
	}
	// With it, it does.
	typeInto(t, s, protectRows, 0, "shh")
	s.open()
	if s.doc == nil {
		t.Fatalf("the password did not open it: %q", s.note)
	}
	x, y = rowAt(t, s, protectRows, 11, 1)
	press(s, x, y)
	if !strings.Contains(s.note, "AES") || !strings.Contains(s.note, "the user") {
		t.Errorf("it said %q", s.note)
	}
	if openedAs(true) == openedAs(false) {
		t.Error("the owner and the user are said the same way")
	}

	// Protecting a document there is none of says so, and leaves the
	// password that reads it back alone.
	none := newState(surfaceW, surfaceH, &fakeHost{})
	none.tools.userPw = "x"
	none.encrypt()
	if none.reopenPw != "" || none.note == "" {
		t.Errorf("reopen password %q, note %q", none.reopenPw, none.note)
	}
	none.decrypt()
	if none.note == "" {
		t.Error("taking the protection off nothing said nothing")
	}
}

func TestTheFileAndProtectPanelsWithNothingOpen(t *testing.T) {
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	for _, act := range []func(){
		empty.flatten, empty.dropAnnots, empty.dropOutlines, empty.clearInfo,
		empty.compress, empty.setInfo, empty.protection,
	} {
		empty.note = ""
		act()
		if empty.note == "" {
			t.Error("a control with no document said nothing")
		}
	}
}

func TestReadingWhatAPageSaysAndWhatItCarries(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{name: "words.pdf", file: wordyPDF(t)})
	s.open()
	openGroup(t, s, groupRead)

	x, y := rowAt(t, s, readRows, 0, 1)
	press(s, x, y)
	if s.tools.reading != readingText {
		t.Fatalf("the reading is %q", s.tools.reading)
	}
	if s.page != nil {
		t.Error("the picture of the page was drawn as well as the reading of it")
	}
	// Pressing it again puts the page back.
	press(s, x, y)
	if s.tools.reading != "" {
		t.Errorf("the reading is %q", s.tools.reading)
	}

	// What it carries: one picture, handed over under a name that says what
	// it is.
	x, y = rowAt(t, s, readRows, 1, 1)
	press(s, x, y)
	if s.tools.reading != readingImages {
		t.Fatalf("the reading is %q", s.tools.reading)
	}
	// The button beside the picture hands it over. Where it is is swept for
	// rather than computed: the list is put where the page was, not in the
	// panel, and what says the control is wired is that a press finds it.
	before := s.tools.reading
	pressed := false
	for y := viewTop; y < viewTop+viewH && !pressed; y += 4 {
		for x := s.pageW() - 80; x < s.pageW() && !pressed; x += 8 {
			s.handleClick(x, y)
			s.draw(buffer())
			pressed = strings.Contains(s.note, "handed over")
		}
	}
	if !pressed {
		t.Errorf("no press beside a picture handed it over; the status line says %q", s.note)
	}
	if !strings.Contains(s.note, ".jpg") {
		t.Errorf("the picture was handed over as %q", s.note)
	}
	if s.tools.reading != before {
		t.Error("handing a picture over changed which reading is on the screen")
	}

	// And the page comes back.
	x, y = rowAt(t, s, readRows, 2, 1)
	press(s, x, y)
	if s.tools.reading != "" || s.page == nil {
		t.Errorf("the reading is %q and the page %v", s.tools.reading, s.page != nil)
	}
}

func TestAPageWithNothingToRead(t *testing.T) {
	s, _ := opened(t, 2) // the sample carries a square and no text
	s.tools.reading = readingText
	s.refresh()
	if s.page != nil {
		t.Error("a picture of the page was drawn under the reading")
	}
	s.tools.reading = readingImages
	s.refresh()
	if s.page != nil {
		t.Error("a picture of the page was drawn under the list")
	}
}

func TestHowAPictureIsNamedByWhatItHolds(t *testing.T) {
	for filter, want := range map[reader.Name]string{
		"DCTDecode":   ".jpg",
		"JPXDecode":   ".jp2",
		"JBIG2Decode": ".jbig2",
		"":            ".samples",
	} {
		im := extract.Image{Filter: filter}
		if got := pictureSuffix(im); got != want {
			t.Errorf("a %q picture is called %s, want %s", filter, got, want)
		}
		if holds(im) == "" {
			t.Errorf("a %q picture holds nothing that can be said", filter)
		}
	}
}

func TestAPageThatCannotBeReadAtAll(t *testing.T) {
	// A page whose content stream cannot be decoded can be neither read nor
	// looked into, and both readings say so rather than showing nothing.
	s := newState(surfaceW, surfaceH, &fakeHost{name: "odd.pdf", file: unreadablePDF(t)})
	s.open()
	if s.doc == nil {
		t.Fatal("the document did not open")
	}
	for _, what := range []string{readingText, readingImages} {
		s.tools.reading = what
		s.refresh()
		if s.page != nil {
			t.Errorf("a page was drawn for the %s reading", what)
		}
	}
}

// wordyPDF is a document whose page says something and carries a picture.
func wordyPDF(t *testing.T) []byte {
	t.Helper()
	var jpg bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{255, 0, 0, 255})
	if err := jpeg.Encode(&jpg, im, nil); err != nil {
		t.Fatal(err)
	}
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
	font := w.Add(reader.Dict{"Type": reader.Name("Font"),
		"Subtype": reader.Name("Type1"), "BaseFont": reader.Name("Helvetica")})
	pic := w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(2), "Height": reader.Integer(2),
		"ColorSpace": reader.Name("DeviceRGB"), "BitsPerComponent": reader.Integer(8),
		"Filter": reader.Name("DCTDecode"),
	}, Raw: jpg.Bytes()})
	body := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
		"BT /F1 12 Tf 20 300 Td (Hello there) Tj ET\nq 60 0 0 60 20 60 cm /P1 Do Q")})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pages, "Contents": body,
		"Resources": reader.Dict{
			"Font":    reader.Dict{"F1": font},
			"XObject": reader.Dict{"P1": pic},
		},
	})
	w.Put(pages, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pages})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// unreadablePDF is a document whose page carries a content stream that no
// filter can undo.
func unreadablePDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pages,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(100), reader.Integer(100)},
		"Contents": w.Add(&reader.Stream{
			Dict: reader.Dict{"Filter": reader.Name("DCTDecode")}, Raw: []byte("not a jpeg")}),
	})
	w.Put(pages, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pages})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// lockedPDF is a document behind a password.
func lockedPDF(t *testing.T, password string) []byte {
	t.Helper()
	d, err := ops.Open(samplePDF(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	d.Encrypt(reader.Encryption{UserPassword: password, OwnerPassword: "owner",
		Permissions: reader.AllPermissions})
	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return out
}
