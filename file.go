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
	"github.com/go-widgets/toolkit"
)

// fileGroup is the panel.
func (s *state) fileGroup() *column {
	box := newColumn()
	box.add(button("Sanitize — drop what runs", toolkit.ButtonDefault, s.sanitize), bareH)
	box.add(button("Flatten the annotations in", toolkit.ButtonDefault, s.flatten), bareH)
	box.add(button("Drop every annotation", toolkit.ButtonDanger, s.dropAnnots), bareH)
	box.add(button("Drop the bookmarks", toolkit.ButtonDanger, s.dropOutlines), bareH)
	box.add(button("Drop what it says about itself", toolkit.ButtonDanger, s.clearInfo), bareH)
	box.add(button("Pack it smaller", toolkit.ButtonDefault, s.compress), bareH)

	box.add(s.entryRow("Title", "what the document is called", "",
		func(v string) { s.tools.title = v }), labelledH)
	box.add(s.entryRow("Author", "who wrote it", "",
		func(v string) { s.tools.author = v }), labelledH)
	box.add(button("Say so in the file", toolkit.ButtonDefault, s.setInfo), bareH)
	return box
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

// setInfo puts a title and an author into the file.
func (s *state) setInfo() {
	title, author := s.tools.title, s.tools.author
	s.changeSaying("said who it is by", func(d *ops.Doc) error {
		d.SetInfo("Title", title)
		d.SetInfo("Author", author)
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
	box.add(s.secretRow("Password to open a file with", func(v string) { s.tools.openPw = v }), labelledH)
	box.add(toolkit.NewLabel("Type it before pressing Open."), bareH)

	box.add(s.secretRow("User password", func(v string) { s.tools.userPw = v }), labelledH)
	box.add(s.secretRow("Owner password", func(v string) { s.tools.ownerPw = v }), labelledH)
	for _, a := range allowed {
		name := a.name
		box.add(tickRow("May "+name, s.tools.allow[name],
			func(on bool) { s.tools.allow[name] = on }), bareH)
	}
	box.add(button("Protect it", toolkit.ButtonProminent, s.encrypt), bareH)
	box.add(button("Take the protection off", toolkit.ButtonDanger, s.decrypt), bareH)
	box.add(button("What is it protected with?", toolkit.ButtonDefault, s.protection), bareH)
	return box
}

// secretRow is a box that shows dots rather than what is typed into it.
func (s *state) secretRow(label string, to func(string)) toolkit.Widget {
	e := toolkit.NewEntry("")
	e.Mask = '•'
	e.Text().Subscribe(to)
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
	if s.tools.userPw == "" && s.tools.ownerPw == "" {
		s.fail("protecting a file needs a user password or an owner password")
		return
	}
	var perms reader.Permissions
	for _, a := range allowed {
		if s.tools.allow[a.name] {
			perms |= a.bit
		}
	}
	how := reader.Encryption{
		UserPassword:  s.tools.userPw,
		OwnerPassword: s.tools.ownerPw,
		Permissions:   perms,
	}
	was := s.reopenPw
	s.reopenPw = s.tools.userPw
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
