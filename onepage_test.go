// A document of a single sheet.

package main

import (
	"strings"
	"testing"
)

// ⛔⛔ A ONE-PAGE DOCUMENT HAS NO RAIL, AND THE STAMP LIVED IN THE RAIL.
// thumbs is created inside rail(), which returns before creating it when the
// document has fewer than two pages — "a document of a single sheet says where
// you are by being a single sheet". Everything that reached for the document's
// identity through s.thumbs.stamp therefore read a nil pointer on exactly the
// documents that have no rail.
//
// contentsJSON is the seam the browser check reads. Opening a one-page
// document and asking what the contents say crashed the tab.
func TestAOnePageDocumentSaysWhatItsContentsAre(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	if s.src.PageCount() != 1 {
		t.Fatalf("the fixture has %d pages", s.src.PageCount())
	}
	if s.thumbs != nil {
		t.Fatal("precondition: a single sheet has no rail, so no thumbs")
	}
	if !s.railContents {
		t.Fatal("precondition: the contents switch starts up")
	}
	if got := s.contentsJSON(); !strings.HasPrefix(got, "[") {
		t.Errorf("the contents of a one-page document read %q", got)
	}
}

// ⛔ And its words can be selected. The page is the only one there is, so a
// workbench that could not select on it could not select on the commonest
// thing somebody drops into a PDF tool: a single scanned or exported sheet.
func TestAOnePageDocumentCanBeSelected(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	runs := s.textRuns(s.src, s.docStamp)
	if len(runs) == 0 {
		t.Fatal("a one-page document offers no text to select")
	}
	b := runs[0].Bounds
	if !s.beginSelect(b.X, b.Y+b.H/2) {
		t.Fatal("a press on the only page did not begin a selection")
	}
	s.endSelect(b.X+b.W*6, b.Y+b.H/2)
	if s.sel.SelectedText() == "" {
		t.Error("dragging across the only page selected nothing")
	}
}
