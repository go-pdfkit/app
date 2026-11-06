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
	out, err := docBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	s := openedWith(t, out)
	s.draw(buffer())

	// ⛔ The VALUES. A FormField draws its own caption and does not hold it as
	// a child label, so this walk cannot see "Subject" — the same caption
	// mechanism the Title and Author rows have always used. What it can see is
	// what the file said, which is the part that was missing.
	said := joined(s.fileGroup().box)
	for _, want := range []string{
		"What it is about",
		"The program that made it",
		"The library that wrote it",
	} {
		if !strings.Contains(said, want) {
			t.Errorf("the panel does not say %q; it says:\n%s", want, said)
		}
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
