// The File and Protect groups: what happens to the file rather than to any
// page of it.
//
// Everything in File is a switch the writer is thrown before it writes, and
// none of them can be thrown back — the library takes each as a decision about
// the file that will be written, not as a setting to be toggled. So they are
// buttons that say what they did, rather than boxes to tick that would be
// stuck on once ticked.

package main

import (
	"fmt"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/mvvm/tkbind"
	"github.com/go-widgets/toolkit"
)

// infoShown is what the panel displays under the two entries it can write.
//
// ⛔ A NAMED SET, not every key in the dictionary. A file chooses what it puts
// in there and how much: it may carry a hundred private keys, or one value a
// megabyte long. So these are drawn, each bounded, and whatever else the file
// says is COUNTED rather than drawn — a panel that quietly showed six of forty
// entries would read exactly like a file that had six.
var infoShown = []reader.Name{
	"Subject", "Keywords", "Creator", "Producer", "CreationDate", "ModDate",
}

// infoWritable are the entries the panel offers a box for, with the hint that
// says what each one is. Title and Author have their own two boxes above them.
//
// ⛔ THE DATES ARE NOT AMONG THEM. A PDF date is D:YYYYMMDDHHmmSSOHH'mm (ISO
// 32000-1, 7.9.4) and nothing in this fleet parses one, so a box to type a date
// into is a box in which a reader writes something no PDF reader will show —
// and a wrong date in a file is worse than one somebody else's program wrote.
// They stay shown as the file wrote them, and `Drop what it says about itself`
// is how they go.
var infoWritable = []struct {
	key  reader.Name
	hint string
}{
	{"Subject", "what it is about"},
	{"Keywords", "words to find it by"},
	{"Creator", "the program it was written in"},
	{"Producer", "the library that wrote the file"},
}

// infoDates are the two entries shown as the file wrote them, for the reason
// above.
var infoDates = []reader.Name{"CreationDate", "ModDate"}

// emptyInfo is a box per writable entry, all empty, which is what the panel
// starts with: it is built before any file is opened, and readInfo fills it
// from the document at the moment one is.
func emptyInfo() map[reader.Name]*mvvm.Observable[string] {
	out := make(map[reader.Name]*mvvm.Observable[string], len(infoWritable))
	for _, w := range infoWritable {
		out[w.key] = mvvm.NewObservable("")
	}
	return out
}

// infoMax bounds one shown value, in runes.
const infoMax = 120

// infoTooLongToEdit says whether an entry is longer than a box should hold, and
// gives it clipped for showing instead. A file chooses what it puts in its
// information dictionary and how much of it.
func (s *state) infoTooLongToEdit(key reader.Name) (bool, string) {
	full := s.infoFull(key)
	if len([]rune(full)) <= infoMax {
		return false, full
	}
	return true, s.infoSays(key)
}

// fileGroup is the panel.
func (s *state) fileGroup() *column {
	box := newColumn()
	box.add(s.entryRow("Title", "what the document is called", s.tools.title), labelledH())
	box.add(s.entryRow("Author", "who wrote it", s.tools.author), labelledH())

	// ⛔ WHAT THE FILE SAYS, SHOWN. The panel used to write only, which is the
	// smaller half of the problem: see setInfo, where not showing is what made
	// writing destructive.
	//
	// ⛔ AND NOW TYPED INTO, for the four that are text. `Say so in the file`
	// wrote a title and an author while the panel below it DISPLAYED four more
	// entries nobody could change — the file's own subject sitting there as a
	// label, read-only for no reason but that nothing had been wired up.
	drawn := 0
	for _, w := range infoWritable {
		if long, v := s.infoTooLongToEdit(w.key); long {
			// ⛔ Shown rather than offered as a box, and the VALUE IS STILL
			// LOADED: a file may carry a subject a megabyte long, which no box
			// can usefully hold, and `Say so in the file` writes every box back
			// — so a value the panel refused to show in full has to go back
			// exactly as it came, not as whatever the box managed to display.
			l := toolkit.NewLabel(v)
			l.Ellipsis = true
			box.add(toolkit.NewFormField(string(w.key)+" (too long to edit here)", l), labelledH())
			continue
		}
		box.add(s.entryRow(string(w.key), w.hint, s.tools.info[w.key]), labelledH())
	}
	for _, k := range infoDates {
		v := s.infoSays(k)
		if v == "" {
			continue
		}
		l := toolkit.NewLabel(v)
		l.Ellipsis = true
		box.add(toolkit.NewFormField(string(k), l), labelledH())
		drawn++
	}
	if more := s.infoMore(drawn); more > 0 {
		l := toolkit.NewLabel(fmt.Sprintf("… and %d more entries, not shown", more))
		l.Disabled().Set(true)
		box.add(l, textH())
	}
	return box
}

// infoSays is one entry of the information dictionary as a line of text, or
// empty when the file does not carry it.
//
// ⛔ Clipped, and the dates are shown AS THE FILE WROTE THEM. A PDF date is a
// string like D:20260101120000+01'00' (ISO 32000-1, 7.9.4) and nothing in the
// fleet parses one; rendering it into a pretty date would mean writing that
// parser here, and showing a guess at it would be worse than showing the text.
func (s *state) infoSays(key reader.Name) string {
	r := []rune(s.infoFull(key))
	if len(r) > infoMax {
		return string(r[:infoMax-1]) + outlineEllipsis
	}
	return string(r)
}

// infoFull is that entry UNCUT, which is what a box to edit it has to be given:
// filling the box from the clipped text and writing the boxes back would cut
// the file's own value down to a hundred and twenty runes and an ellipsis, by
// way of showing it.
func (s *state) infoFull(key reader.Name) string {
	if s.doc == nil {
		return ""
	}
	str, ok := s.doc.Info()[key].(reader.String)
	if !ok {
		return ""
	}
	return string(str)
}

// infoMore is how many entries of the information dictionary the panel is NOT
// showing, given how many it drew: everything the file carries, less those and
// the two the fields above already hold.
//
// ⛔ ONE function with ONE guard, because the first version was two — a counter
// that checked for no document and a second that did not — and the panel is
// built BEFORE a file is opened. The empty workbench crashed on a nil pointer
// at startup. Two functions that must agree about the same precondition are
// two chances to write it once.
func (s *state) infoMore(drawn int) int {
	if s.doc == nil {
		return 0
	}
	info := s.doc.Info()
	shown := drawn
	accounted := []reader.Name{"Title", "Author"}
	for _, w := range infoWritable {
		accounted = append(accounted, w.key)
	}
	for _, k := range accounted {
		if _, ok := info[k]; ok {
			shown++
		}
	}
	return len(info) - shown
}

// readInfo fills the two fields from what the document already says.
//
// ⛔⛔ THIS IS WHAT MAKES `Say so in the file` SAFE TO PRESS. ops.SetInfo
// documents that "an empty value removes it", and the fields started empty
// whatever the file said — so opening a paper that carried a title and an
// author and pressing the button deleted both, silently, under a tip that said
// it writes "the title and author typed beside it".
//
// ⛔ At OPEN and not on every refresh. A refresh follows every verb, and
// re-reading the fields there would throw away what somebody had typed but not
// yet written the moment they rotated a page.
// ⛔ AND FROM THE UNCUT VALUE. infoSays clips what it returns to a hundred and
// twenty runes and an ellipsis, which is right for a label and destructive for
// a box that is written back: filling the Title box from it and pressing the
// button would have cut a long title down to the ellipsis the panel drew.
func (s *state) readInfo() {
	s.tools.title.Set(s.infoFull("Title"))
	s.tools.author.Set(s.infoFull("Author"))
	for _, w := range infoWritable {
		s.tools.info[w.key].Set(s.infoFull(w.key))
	}
}

// flatten draws the annotations into the pages, so that what a form says is
// part of the page rather than a layer over it.
func (s *state) flatten() {
	s.changeSaying("flattened what was drawn over the pages", func(d *ops.Doc) error {
		d.Flatten()
		return nil
	})
}

// dropAnnots removes every annotation, links included.
func (s *state) dropAnnots() {
	s.changeSaying("dropped every annotation", func(d *ops.Doc) error {
		d.RemoveAnnotations()
		return nil
	})
}

// dropOutlines removes the bookmarks.
func (s *state) dropOutlines() {
	s.changeSaying("dropped the bookmarks", func(d *ops.Doc) error {
		d.DropOutlines()
		return nil
	})
}

// clearInfo drops what the file says about itself: who made it, with what, and
// when — which is the part of a document nobody means to send.
func (s *state) clearInfo() {
	s.changeSaying("dropped what it said about itself", func(d *ops.Doc) error {
		d.ClearInfo()
		return nil
	})
}

// compress packs the objects into compressed streams, which is what every
// writer since PDF 1.5 does.
func (s *state) compress() {
	before := s.written()
	if !s.changeSaying("packed", func(d *ops.Doc) error {
		d.Compress()
		return nil
	}) {
		return
	}
	after := s.written()
	if before == 0 || after == 0 {
		// The redraw is about to say which of the two steps went wrong, and it
		// says it better than a pair of zeroes would.
		return
	}
	// Saying how much smaller is the only way to know it did anything, and it
	// costs a write that was going to happen anyway.
	s.note = fmt.Sprintf("packed: %d bytes rather than %d", after, before)
	s.refresh()
}

// written is how large the file would be if it were saved now, or zero when
// there is nothing to write or it cannot be written at all — which the control
// that asked, or the redraw after it, says for itself.
func (s *state) written() int {
	if s.doc == nil {
		return 0
	}
	out, msg := s.reopenBytes()
	if msg != "" {
		return 0
	}
	return len(out)
}

// setInfo puts what the panel's boxes say into the file: the title, the author
// and the four other text entries it offers.
//
// ⛔ EVERY BOX, EVERY TIME, including the ones nobody touched — which is safe
// only because readInfo filled them all from the document when it was opened.
// SetInfo documents that "an empty value removes it", so a box the panel had
// left empty is an instruction to delete that entry; see readInfo for the day
// that cost a paper its title and its author.
func (s *state) setInfo() {
	title, author := s.tools.title.Get(), s.tools.author.Get()
	said := make(map[reader.Name]string, len(infoWritable))
	for _, w := range infoWritable {
		said[w.key] = s.tools.info[w.key].Get()
	}
	s.changeSaying("said what it is", func(d *ops.Doc) error {
		d.SetInfo("Title", title)
		d.SetInfo("Author", author)
		for k, v := range said {
			d.SetInfo(k, v)
		}
		return nil
	})
}

// allowed is what a reader of the protected file may do.
var allowed = []struct {
	name string
	bit  reader.Permissions
}{
	{"print it", reader.PermPrint},
	{"change it", reader.PermModify},
	{"copy text out of it", reader.PermCopy},
	{"annotate it", reader.PermAnnotate},
	{"fill its forms in", reader.PermFillForms},
}

// protectGroup is the panel: a password to open a file that has one, and a
// password to put on the file that will be written.
func (s *state) protectGroup() *column {
	box := newColumn()
	box.add(s.secretRow("Password to open a file with", s.tools.openPw), labelledH())
	box.add(toolkit.NewLabel("Type it before pressing Open."), bareH())

	box.add(s.secretRow("User password", s.tools.userPw), labelledH())
	box.add(s.secretRow("Owner password", s.tools.ownerPw), labelledH())
	for _, a := range allowed {
		box.add(s.tickRow("May "+a.name, s.tools.allow[a.name]), bareH())
	}
	return box
}

// secretRow is a box that shows dots rather than what is typed into it.
func (s *state) secretRow(label string, to *mvvm.Observable[string]) toolkit.Widget {
	e := toolkit.NewEntry(to.Get())
	e.Mask = '•'
	tkbind.BindEntry(to, e, s.repaint)
	s.typing = append(s.typing, e)
	return toolkit.NewFormField(label, e)
}

// encrypt protects the file that will be written.
//
// What is drawn afterwards is still what would be saved: the workbench writes
// the document, and reads it back with the password just given. It is not the
// same bytes twice — encryption needs randomness, so every redraw produces a
// different file — but it is the same document, protected the same way.
func (s *state) encrypt() {
	if s.tools.userPw.Get() == "" && s.tools.ownerPw.Get() == "" {
		s.fail("protecting a file needs a user password or an owner password")
		return
	}
	var perms reader.Permissions
	for _, a := range allowed {
		if s.tools.allow[a.name].Get() {
			perms |= a.bit
		}
	}
	how := reader.Encryption{
		UserPassword:  s.tools.userPw.Get(),
		OwnerPassword: s.tools.ownerPw.Get(),
		Permissions:   perms,
	}
	was := s.reopenPw
	s.reopenPw = s.tools.userPw.Get()
	if !s.changeSaying("protected with AES-256", func(d *ops.Doc) error {
		d.Encrypt(how)
		return nil
	}) {
		s.reopenPw = was
	}
}

// decrypt writes the file without protection.
func (s *state) decrypt() {
	was := s.reopenPw
	s.reopenPw = ""
	if !s.changeSaying("the protection will not be written", func(d *ops.Doc) error {
		d.Decrypt()
		return nil
	}) {
		s.reopenPw = was
	}
}

// protection says how the file this document was read from was protected. It
// says nothing about how it will be written, which is what the two buttons
// above it decide.
func (s *state) protection() {
	if s.doc == nil {
		s.fail("open a document first")
		return
	}
	p, ok := s.doc.Protection()
	if !ok {
		s.fail("the file this came from was not protected")
		return
	}
	s.note = fmt.Sprintf("%s, revision %d, opened as %s, allowing %s",
		p.Method, p.Revision, openedAs(p.Owner), p.Permissions)
	s.refresh()
}

// openedAs says which password the file was opened with.
func openedAs(owner bool) string {
	if owner {
		return "the owner, so the permissions do not apply"
	}
	return "the user"
}
