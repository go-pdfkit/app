package main

import (
	"strings"
	"testing"
)

// TestNoPasswordEverReachesWhatTheWorkbenchSays is a security control over the
// one thing on screen that is not masked.
//
// ⛔ An error or a report that quotes its input would put a password in the
// status line in plain sight, beside a `•••••` box that was carefully hiding
// the same characters. A screenshot, a support request, or somebody standing
// behind you is all it takes after that.
//
// ⛔⛔ The first draft of this PASSED and proved nothing. It set a password,
// called encrypt and then protection, and asserted the note did not contain
// it — but protection() reports on the file AS OPENED, so it answered "the
// file this came from was not protected" and never reached the line that
// formats a report at all. Two deliberate leaks planted in those paths both
// survived. A security test has to be shown to FAIL, or it is a sentence
// about a path nobody took.
func TestNoPasswordEverReachesWhatTheWorkbenchSays(t *testing.T) {
	const secret = "correcthorsebatterystaple"
	say := func(s *state) string { return s.note + " " + strings.Join(s.statusLine(), " ") }

	// A file that really is protected, opened with the password, and asked
	// what it is protected with — which is the path that formats a report.
	s := newState(surfaceW, surfaceH, &fakeHost{name: "locked.pdf", file: lockedPDF(t, secret)})
	s.tools.openPw.Set(secret)
	s.open()
	if s.doc == nil {
		t.Fatalf("the password did not open the file: %q", s.note)
	}
	if got := say(s); strings.Contains(got, secret) {
		t.Errorf("opening a protected file said the password: %q", got)
	}
	s.protection()
	if !strings.Contains(s.note, "AES") {
		t.Fatalf("this is not the report path: %q", s.note)
	}
	if got := say(s); strings.Contains(got, secret) {
		t.Errorf("the protection report said the password: %q", got)
	}

	// The wrong password, which is the path that formats a refusal.
	w := newState(surfaceW, surfaceH, &fakeHost{name: "locked.pdf", file: lockedPDF(t, secret)})
	w.tools.openPw.Set(secret + "-wrong")
	w.open()
	if w.doc != nil {
		t.Fatal("the wrong password opened the file, so this is not the refusal path")
	}
	if got := say(w); strings.Contains(got, secret) {
		t.Errorf("refusing a password said it: %q", got)
	}

	// And protecting a file, which is handed both of them.
	p, _ := opened(t, 2)
	p.tools.userPw.Set(secret)
	p.tools.ownerPw.Set(secret + "-owner")
	p.encrypt()
	if !strings.Contains(p.note, "protected") {
		t.Fatalf("this is not the protecting path: %q", p.note)
	}
	if got := say(p); strings.Contains(got, secret) {
		t.Errorf("protecting a file said the password: %q", got)
	}
}
