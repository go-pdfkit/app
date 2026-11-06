// What tells one word from the next.

package main

import (
	"testing"

	"github.com/go-pdfkit/extract"
)

// line is the glyphs of one line of real text, at the measurements a real page
// gives them.
//
// ⛔ THE NUMBERS ARE READ OFF A PAGE, not invented. They are the opening of
// gotex-wazero/out.pdf, printed with extract.Runs: a 10pt face, every glyph its
// own run, and Space reported as 10.00 — exactly the font size.
func line() []extract.Run {
	at := func(text string, x, w float64) extract.Run {
		return extract.Run{Text: text, X: x, Y: 107.89, Width: w, Size: 10, Space: 10}
	}
	return []extract.Run{
		at("L", 92.00, 5.85), at("e", 98.00, 4.64),
		at("m", 105.84, 8.52), at("o", 114.84, 5.14), at("t", 119.84, 3.15),
		at("e", 122.84, 4.64), at("u", 127.84, 5.70), at("r", 133.84, 3.80),
		at("g", 140.68, 4.84), at("o", 145.68, 5.14), at("t", 150.68, 3.15),
		at("e", 153.68, 4.64), at("x", 158.68, 4.79),
	}
}

// ⛔⛔ "Lemoteurgotex" IS WHAT IT SAID. extract reports Run.Space as the font
// SIZE rather than the width of a space — go-pdfkit/extract#28, and go-pdfkit's
// own latex package documents the same producer: "a composite font has no
// character at 32 and answers with whatever its fallback width is; one real
// producer answers with the font size, and taking that at face value turns a
// page into one long word."
//
// needsSpace asked for a gap wider than HALF of that, so 5pt at 10pt text. The
// real gap between two words here is 3.2pt and between two letters 0.15pt, so
// every gap failed the test and no space was ever found. Every link row in the
// contents pane read as one long word, on every real document.
func TestWordsAreSeparatedOnARealLine(t *testing.T) {
	var got string
	runs := line()
	for i, r := range runs {
		if i > 0 && needsSpace(runs[i-1], r) {
			got += " "
		}
		got += r.Text
	}
	if want := "Le moteur gotex"; got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

// ⛔ The control that it does not now put a space between every letter, which
// is the other way to be wrong and would pass a test that only looked for
// spaces.
func TestLettersOfAWordAreNotSeparated(t *testing.T) {
	runs := line()
	if needsSpace(runs[0], runs[1]) { // "L" then "e", 0.15pt apart
		t.Error("two letters of one word were separated")
	}
	if !needsSpace(runs[1], runs[2]) { // "e" then "m", 3.2pt apart
		t.Error("two words were run together")
	}
}

// ⛔ A font that reports a BELIEVABLE space is believed. The fallback exists
// for producers that answer with the size; it must not override a real number,
// or a page whose spacing is genuinely unusual is read by a guess.
func TestABelievableSpaceIsBelieved(t *testing.T) {
	// A 10pt face reporting a 2.5pt space: a gap of 2pt is a word break at
	// half of that, and 1pt is not.
	a := extract.Run{Text: "a", X: 0, Y: 0, Width: 5, Size: 10, Space: 2.5}
	near := extract.Run{Text: "b", X: 6, Y: 0, Width: 5, Size: 10, Space: 2.5}
	far := extract.Run{Text: "b", X: 7, Y: 0, Width: 5, Size: 10, Space: 2.5}
	if needsSpace(a, near) { // gap 1.0, threshold 1.25
		t.Error("a gap narrower than half the reported space was a word break")
	}
	if !needsSpace(a, far) { // gap 2.0, threshold 1.25
		t.Error("a gap wider than half the reported space was not a word break")
	}
}

// ⛔ And a run on another line is always a different word, whatever the gap:
// the last word of one line and the first of the next are never joined.
func TestADifferentLineIsAlwaysASeparateWord(t *testing.T) {
	a := extract.Run{Text: "a", X: 100, Y: 100, Width: 5, Size: 10, Space: 10}
	b := extract.Run{Text: "b", X: 100, Y: 88, Width: 5, Size: 10, Space: 10}
	if !needsSpace(a, b) {
		t.Error("the end of one line was joined to the start of the next")
	}
}
