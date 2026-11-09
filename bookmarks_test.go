package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/ops"
)

// marksOf is the table of contents of the document as it would be SAVED, read
// back out of the written bytes.
//
// ⛔ OUT OF THE FILE AND NOT OUT OF THE WORKBENCH'S OWN STATE. SetOutline is a
// decision the writer is handed, so the only place an outline exists is in the
// bytes it writes; a test that asked the state what it had been told would pass
// on a library that wrote nothing at all.
func marksOf(t *testing.T, s *state) []bookmark {
	t.Helper()
	src, msg := s.reopen()
	if msg != "" {
		t.Fatalf("the document could not be written and read back: %s", msg)
	}
	return s.bookmarks(src)
}

// titlesOf is those bookmarks as "title@page" at their depth, which is the
// whole of what a rail row says and enough to tell one tree from another.
func titlesOf(marks []bookmark) []string {
	out := make([]string, 0, len(marks))
	for _, m := range marks {
		out = append(out, fmt.Sprintf("%s%s@%d", strings.Repeat("  ", m.depth), m.title, m.page))
	}
	return out
}

func TestBookmarkingThePageKeepsTheOnesAlreadyThere(t *testing.T) {
	s := openOutlined(t, 4, []mark{
		{"One", 1, -1},
		{"Under one", 2, 0},
		{"Two", 3, -1},
	}, nil)
	s.at = 4

	s.bookmarkPage()

	if !strings.Contains(s.note, "bookmarked page 4") {
		t.Errorf("the status line says %q", s.note)
	}
	got := titlesOf(marksOf(t, s))
	want := []string{"One@1", "  Under one@2", "Two@3", "Page 4@4"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the saved document's contents are\n%v\nwant\n%v", got, want)
	}
}

// TestBookmarkingAPassageTitlesItWithTheWords is the other half of the verb:
// what a reader who has just dragged across a heading means by it.
func TestBookmarkingAPassageTitlesItWithTheWords(t *testing.T) {
	h := &fakeHost{name: "wordy.pdf", file: wordyPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	selectTheWord(t, s, "Hello there")

	s.bookmarkPage()

	got := titlesOf(marksOf(t, s))
	if len(got) != 1 || got[0] != "Hello there@1" {
		t.Errorf("the saved contents are %v, want one row titled with the words", got)
	}
}

// TestABookmarkTitleIsOneLineAndNoLongerThanARailRow holds the two cuts made to
// a passage before it becomes a title, both of which a reader can reach by
// dragging across more than a heading.
func TestABookmarkTitleIsOneLineAndNoLongerThanARailRow(t *testing.T) {
	for _, c := range []struct{ why, said, want string }{
		{"a short heading is taken as it is", "A heading", "A heading"},
		{"a passage of several lines is its first", "A heading\nand its paragraph", "A heading"},
		{"a long line is cut and said to be cut",
			strings.Repeat("a", bookmarkTitleMax+20), strings.Repeat("a", bookmarkTitleMax) + "…"},
	} {
		if got := oneLine(c.said, bookmarkTitleMax); got != c.want {
			t.Errorf("%s: %q became %q, want %q", c.why, c.said, got, c.want)
		}
	}
}

// TestADocumentsOwnBookmarksNeverSkipALevel is the invariant bookmarkPage
// relies on when it does not branch on ops.Nest's refusal: the rows it hands
// over cannot hold a rung missing, because the walk that produced them descends
// one level at a time.
func TestADocumentsOwnBookmarksNeverSkipALevel(t *testing.T) {
	s := openOutlined(t, 6, []mark{
		{"One", 1, -1},
		{"Under one", 2, 0},
		{"Under that", 3, 1},
		{"Under that too", 4, 2},
		{"Two", 5, -1},
		{"Under two", 6, 4},
	}, nil)

	rows := s.levelsNow()
	if len(rows) != 6 {
		t.Fatalf("the walk found %d entries, want 6: %+v", len(rows), rows)
	}
	was := -1
	for i, r := range rows {
		if r.Depth > was+1 {
			t.Errorf("entry %d (%q) sits at depth %d under one at %d",
				i, r.Title, r.Depth, was)
		}
		was = r.Depth
	}
	if _, err := ops.Nest(rows); err != nil {
		t.Errorf("the document's own bookmarks were refused: %v", err)
	}
}

// TestABookmarkThatGoesNowhereTakesItsChildrenWithIt is the case that would
// otherwise write NOTHING while saying it had written something: the rail keeps
// a row whose destination could not be followed, and a tree being written
// cannot — so dropping the parent has to drop what hung off it, or the child is
// left a level adrift and ops.Nest refuses the whole outline.
func TestABookmarkThatGoesNowhereTakesItsChildrenWithIt(t *testing.T) {
	s := openOutlined(t, 3, []mark{
		{"Real", 1, -1},
		{"Broken", 0, -1},
		{"Under the broken one", 2, 1},
		{"Real again", 3, -1},
	}, nil)

	// The precondition: the rail DOES show the broken row, so this test is
	// about what the writer leaves out and not about what the walk missed.
	rail := titlesOf(s.marksFor(s.src, s.docStamp))
	if len(rail) != 4 {
		t.Fatalf("the rail shows %v, and this test needs the broken row in it", rail)
	}

	rows := s.levelsNow()
	if _, err := ops.Nest(rows); err != nil {
		t.Fatalf("the rows left for the writer were refused: %v (%+v)", err, rows)
	}
	s.at = 3
	s.bookmarkPage()
	got := titlesOf(marksOf(t, s))
	want := []string{"Real@1", "Real again@3", "Page 3@3"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the saved contents are\n%v\nwant\n%v", got, want)
	}
}

// TestAddingAFileAfterThisOneKeepsBOTHTablesOfContents settles a verdict rather
// than a defect. The gap inventory said of `Add a file after this one` that
// "the added file's bookmarks are not carried", which was read off nothing: ops
// carries EVERY source's outline, in the order the documents first contribute a
// page, and the only way to know which of those two sentences is true is to
// merge two outlined documents and read the result.
//
// ⛔ A claim about somebody else's code, written in my own notes, is not a
// measurement. This is the measurement.
func TestAddingAFileAfterThisOneKeepsBOTHTablesOfContents(t *testing.T) {
	s := openOutlined(t, 2, []mark{{"This one", 1, -1}}, nil)
	s.host.(*fakeHost).name = "other.pdf"
	s.host.(*fakeHost).file = writeOutlined(t, 2, []mark{{"The other one", 1, -1}}, nil)

	s.merge()

	if s.doc == nil || s.doc.PageCount() != 4 {
		t.Fatalf("the merge left %q", s.note)
	}
	got := titlesOf(marksOf(t, s))
	want := []string{"This one@1", "The other one@3"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the merged document's contents are\n%v\nwant\n%v", got, want)
	}
}

func TestATableOfContentsIsWrittenFromAFile(t *testing.T) {
	s := openOutlined(t, 5, []mark{{"Whatever was there", 1, -1}}, nil)
	s.host.(*fakeHost).name = "toc.txt"
	s.host.(*fakeHost).file = []byte("Introduction\t1\n  Why\t2\nChapter one\t4\n")

	s.bookmarksFromFile()

	if !strings.Contains(s.note, "wrote 3 bookmarks from toc.txt") {
		t.Errorf("the status line says %q", s.note)
	}
	got := titlesOf(marksOf(t, s))
	want := []string{"Introduction@1", "  Why@2", "Chapter one@4"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the saved document's contents are\n%v\nwant\n%v", got, want)
	}
}

// TestABookmarkFileThatCannotBeReadIsRefusedNamingTheLine: the library names
// the line and the workbench hands that sentence on, because a person looking
// at a file they wrote needs to be told which line of it is wrong.
func TestABookmarkFileThatCannotBeReadIsRefusedNamingTheLine(t *testing.T) {
	for _, c := range []struct{ why, text, says string }{
		{"a line with no tab", "Introduction\t1\nChapter one 4\n", "line 2"},
		{"a page that is not a number", "Introduction\tfour\n", "not a page number"},
		{"nothing to read at all", "\n   \n", "holds no bookmarks"},
	} {
		s := openOutlined(t, 4, []mark{{"Left alone", 1, -1}}, nil)
		s.host.(*fakeHost).name = "toc.txt"
		s.host.(*fakeHost).file = []byte(c.text)

		s.bookmarksFromFile()

		if !strings.Contains(s.note, c.says) {
			t.Errorf("%s: the status line says %q, which does not say %q", c.why, s.note, c.says)
		}
		// ⛔ AND THE DOCUMENT IS UNTOUCHED. A refusal that had already dropped
		// the bookmarks would be worse than no refusal at all.
		if got := titlesOf(marksOf(t, s)); len(got) != 1 || got[0] != "Left alone@1" {
			t.Errorf("%s: the refused file left the contents as %v", c.why, got)
		}
	}
}

func TestABookmarkFileLargerThanTheBoundIsRefused(t *testing.T) {
	s := openOutlined(t, 2, []mark{{"Left alone", 1, -1}}, nil)
	s.host.(*fakeHost).name = "huge.txt"
	s.host.(*fakeHost).file = []byte(strings.Repeat("Introduction\t1\n", maxContentsBytes/15+1))

	s.bookmarksFromFile()

	if !strings.Contains(s.note, "a table of contents is read up to") {
		t.Errorf("the status line says %q", s.note)
	}
	if got := titlesOf(marksOf(t, s)); len(got) != 1 {
		t.Errorf("the refused file left the contents as %v", got)
	}
}

func TestBookmarksFromAFileNeedADocumentFirst(t *testing.T) {
	h := &fakeHost{}
	s := newState(surfaceW, surfaceH, h)

	s.bookmarksFromFile()

	if !strings.Contains(s.note, "open a document first") {
		t.Errorf("the status line says %q", s.note)
	}
	if h.asked != 0 {
		t.Errorf("the file chooser was shown %d times with nothing open", h.asked)
	}
}

// TestNestingCountsEveryRowAndNotTheHeadings: the status line reports the rows
// a reader will see in the rail.
func TestNestingCountsEveryRowAndNotTheHeadings(t *testing.T) {
	marks, err := ops.ParseOutline("One\t1\n  Under\t2\n    Deeper\t3\nTwo\t4\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 2 {
		t.Fatalf("%d top-level entries", len(marks))
	}
	if got := countMarks(marks); got != 4 {
		t.Errorf("counted %d bookmarks, want 4", got)
	}
}
