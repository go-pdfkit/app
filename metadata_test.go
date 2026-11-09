// What the file says about itself, shown as well as written.

package main

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// describedPDF is a document that ALREADY says a title and an author, written
// out as a file.
//
// ⛔ The bytes matter: the fields are filled when a file is OPENED, so a test
// that opens a blank sample and then sets the information on the document in
// memory never reaches the code it means to test. The first version of this
// file did exactly that and reported the defect still present after it was
// fixed.
func describedPDF(t *testing.T, title, author string) []byte {
	t.Helper()
	d, err := ops.OpenWithPassword(samplePDF(t, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInfo("Title", title)
	d.SetInfo("Author", author)
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// said is one entry of the information dictionary, as text.
func said(t *testing.T, s *state, key reader.Name) string {
	t.Helper()
	if s.doc == nil {
		t.Fatal("no document")
	}
	v, ok := s.doc.Info()[key]
	if !ok {
		return ""
	}
	str, ok := v.(reader.String)
	if !ok {
		t.Fatalf("%s is a %T", key, v)
	}
	return string(str)
}

// ⛔⛔ PRESSING IT WITHOUT TYPING MUST NOT ERASE WHAT IS THERE. The panel's two
// fields started empty whatever the document said, and ops.SetInfo documents
// that "an empty value REMOVES it" — so opening a paper that carries a title
// and an author and pressing `Say so in the file` deleted both. The control's
// own tip says it writes "the title and author typed beside it", and nothing
// warned that typing nothing meant deleting.
//
// This is the whole of what the gap inventory called the smallest item in the
// list: the panel writes and never shows. It is not only a missing view — not
// showing is what made the writing destructive.
func TestSayingWhoItIsByKeepsWhatIsAlreadyThere(t *testing.T) {
	s := openedWith(t, describedPDF(t, "A title its author wrote", "Someone Real"))
	s.draw(buffer())

	if got := said(t, s, "Title"); got != "A title its author wrote" {
		t.Fatalf("precondition: the document's title reads %q", got)
	}

	// The reader opened the file and pressed the button. They typed nothing.
	s.setInfo()

	if got := said(t, s, "Title"); got != "A title its author wrote" {
		t.Errorf("the title is now %q; pressing the button erased what the file said", got)
	}
	if got := said(t, s, "Author"); got != "Someone Real" {
		t.Errorf("the author is now %q", got)
	}
}

// ⛔ And the fields SHOW what is there, which is both how a reader learns what
// the file says and why the button above is no longer destructive: what it
// writes back is what was shown.
func TestTheFieldsShowWhatTheFileSays(t *testing.T) {
	s := openedWith(t, describedPDF(t, "A title its author wrote", "Someone Real"))
	s.draw(buffer())

	if got := s.tools.title.Get(); got != "A title its author wrote" {
		t.Errorf("the Title field reads %q", got)
	}
	if got := s.tools.author.Get(); got != "Someone Real" {
		t.Errorf("the Author field reads %q", got)
	}
}

// ⛔ What the reader TYPES still wins — the control for the test above, which
// would pass on a panel that had stopped writing anything at all.
func TestWhatTheReaderTypesIsWhatIsWritten(t *testing.T) {
	s := openedWith(t, describedPDF(t, "The old title", "Someone"))
	s.draw(buffer())

	s.tools.title.Set("The new title")
	s.setInfo()

	if got := said(t, s, "Title"); got != "The new title" {
		t.Errorf("the title is %q; what was typed was not written", got)
	}
}

// labelsIn is the text of every label under a widget, in order.
func labelsIn(w toolkit.Widget) []string {
	if l, ok := w.(*toolkit.Label); ok {
		return []string{l.Text().Get()}
	}
	p, ok := w.(interface{ Children() []toolkit.Widget })
	if !ok {
		return nil
	}
	var out []string
	for _, c := range p.Children() {
		out = append(out, labelsIn(c)...)
	}
	return out
}

func joined(w toolkit.Widget) string { return strings.Join(labelsIn(w), "\n") }

// ⛔ THE REST OF WHAT THE FILE SAYS IS SHOWN. This is the gap inventory's
// "View Metadata" row: the panel wrote two entries and displayed none, so a
// reader could not learn what the file already claimed about itself — which is
// exactly what somebody checking a document before sending it needs.
func TestThePanelShowsWhatElseTheFileSays(t *testing.T) {
	d, err := ops.OpenWithPassword(samplePDF(t, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInfo("Subject", "What it is about")
	d.SetInfo("Creator", "The program that made it")
	d.SetInfo("Producer", "The library that wrote it")
	// ⛔ A DATE AS THE FILE WROTE IT (ISO 32000-1, 7.9.4). It is shown and not
	// offered as a box: nothing in this fleet parses one, so a box to type a
	// date into is a box in which a reader writes something no PDF reader will
	// show.
	d.SetInfo("CreationDate", "D:20260101120000+01'00'")
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	s := openedWith(t, out)
	s.draw(buffer())

	// ⛔ IN THE BOXES, which is where they moved: for a while these were labels,
	// and the panel displayed four entries of the file's own information
	// dictionary that nobody could change. A FormField draws its own caption
	// and does not hold it as a child widget, so neither this walk nor the one
	// over the labels can see "Subject"; what they can see is what the file
	// said, which is the part that matters.
	var typed []string
	for _, e := range entriesIn(s.fileGroup().box) {
		typed = append(typed, e.Text().Get())
	}
	said := strings.Join(typed, "\n")
	for _, want := range []string{
		"What it is about",
		"The program that made it",
		"The library that wrote it",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("no box in the panel holds %q; they hold:\n%s", want, said)
		}
	}
	if shown := joined(s.fileGroup().box); !strings.Contains(shown, "D:20260101120000+01'00'") {
		t.Errorf("the panel does not show the file's own creation date; it shows:\n%s", shown)
	}
}

// TestTheFourOtherEntriesAreWrittenBack is the verb behind those boxes: the
// panel showed them for four versions and `Say so in the file` wrote only the
// title and the author, so the subject a reader typed went nowhere.
func TestTheFourOtherEntriesAreWrittenBack(t *testing.T) {
	s := openedWith(t, describedPDF(t, "A title", "Someone"))
	s.draw(buffer())

	s.tools.info["Subject"].Set("What it is about")
	s.tools.info["Keywords"].Set("one two three")
	s.tools.info["Creator"].Set("The program that made it")
	s.tools.info["Producer"].Set("The library that wrote it")
	s.setInfo()

	for _, c := range []struct{ key, want string }{
		{"Subject", "What it is about"},
		{"Keywords", "one two three"},
		{"Creator", "The program that made it"},
		{"Producer", "The library that wrote it"},
		// ⛔ And the two the panel never touched are still there, because every
		// box is written every time and these were filled when the file opened.
		{"Title", "A title"},
		{"Author", "Someone"},
	} {
		if got := s.infoFull(reader.Name(c.key)); got != c.want {
			t.Errorf("after writing, %s reads %q, want %q", c.key, got, c.want)
		}
	}
}

// TestAnEntryTooLongToEditIsNotCutDownByWritingTheOthers is the trap this verb
// walks past: the panel clips a value to show it, and `Say so in the file`
// writes every box. A box filled from the CLIPPED text would turn the file's
// own two-thousand-character subject into a hundred and twenty runes and an
// ellipsis, by way of displaying it.
func TestAnEntryTooLongToEditIsNotCutDownByWritingTheOthers(t *testing.T) {
	long := strings.Repeat("long ", 400)
	d, err := ops.OpenWithPassword(samplePDF(t, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInfo("Subject", long)
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	s := openedWith(t, out)
	s.draw(buffer())

	// The precondition: it IS too long to be offered as a box, so this test is
	// about the value surviving and not about a box nobody filled.
	if tooLong, _ := s.infoTooLongToEdit("Subject"); !tooLong {
		t.Fatalf("a %d-rune subject was offered as a box; this test needs one that is not",
			len([]rune(long)))
	}
	// And the panel SAYS why there is no box for it, rather than leaving a row
	// out and reading like a file that has no subject at all.
	if shown := joined(s.fileGroup().box); !strings.Contains(shown, outlineEllipsis) {
		t.Errorf("the panel shows no clipped value for the long subject; it shows:\n%s", shown)
	}
	s.tools.title.Set("A new title")
	s.setInfo()

	if got := s.infoFull("Subject"); got != long {
		t.Errorf("writing the other boxes left the subject %d runes long, want %d",
			len([]rune(got)), len([]rune(long)))
	}
}

// ⛔ AND IT SAYS HOW MANY IT IS NOT SHOWING. A file may carry any keys it
// likes. A panel that drew six of forty would read exactly like a file that
// had six, which is the same lie a listing tells when it silently stops.
func TestThePanelSaysHowManyEntriesItIsNotShowing(t *testing.T) {
	d, err := ops.OpenWithPassword(samplePDF(t, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInfo("Title", "A title")
	for _, k := range []reader.Name{"Private1", "Private2", "Private3"} {
		d.SetInfo(k, "something this panel does not draw")
	}
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	s := openedWith(t, out)
	s.draw(buffer())

	if said := joined(s.fileGroup().box); !strings.Contains(said, "3 more entries") {
		t.Errorf("the panel does not say what it left out; it says:\n%s", said)
	}
}

// ⛔ A VALUE THE FILE MADE ENORMOUS IS CUT, and says it was. The file chooses
// this string and a panel is not a place to render a megabyte of it.
func TestAnEnormousValueIsCutAndSaysSo(t *testing.T) {
	d, err := ops.OpenWithPassword(samplePDF(t, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	d.SetInfo("Subject", strings.Repeat("long ", 400))
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	s := openedWith(t, out)
	s.draw(buffer())

	got := s.infoSays("Subject")
	if n := len([]rune(got)); n != infoMax {
		t.Errorf("a 2000-character subject came back %d runes, want %d", n, infoMax)
	}
	if !strings.HasSuffix(got, outlineEllipsis) {
		t.Errorf("a cut value does not say it was cut: %q", got)
	}
}

// ⛔ And with no document at all there is nothing to say and nothing to crash
// on: the panel is built before a file is opened.
func TestThePanelWithNoDocumentSaysNothing(t *testing.T) {
	s := newState(surfaceW, surfaceH, &fakeHost{})
	if got := s.infoSays("Title"); got != "" {
		t.Errorf("with no document the title reads %q", got)
	}
	if got := s.infoMore(0); got != 0 {
		t.Errorf("with no document there are %d entries left over", got)
	}
	if said := joined(s.fileGroup().box); strings.Contains(said, "more entries") {
		t.Errorf("an empty workbench counts entries it does not have:\n%s", said)
	}
}

// ⛔ And emptying a field ON PURPOSE still removes the entry. Keeping what is
// there must not become "this can never be cleared": `Drop what it says about
// itself` removes everything at once, and a reader who wants only the author
// gone has nowhere else to do it.
func TestClearingAFieldOnPurposeStillRemovesIt(t *testing.T) {
	s := openedWith(t, describedPDF(t, "A title", "Someone"))
	s.draw(buffer())

	s.tools.author.Set("")
	s.setInfo()

	if got := said(t, s, "Author"); got != "" {
		t.Errorf("the author is still %q after being cleared on purpose", got)
	}
	if got := said(t, s, "Title"); got != "A title" {
		t.Errorf("clearing the author took the title with it: %q", got)
	}
}
