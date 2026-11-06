package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// outlined builds a document of n pages whose outline is the given entries.
// Each entry is a title, the 1-based page it points at, and the index of its
// parent entry (-1 for a top-level one), so a tree can be written down as a
// list the way a reader sees it.
type mark struct {
	title  string
	page   int
	parent int
}

// writeOutlined is the fixture: a real document with a real /Outlines tree,
// written the way go-pdfkit/pdfkit writes one (document.go:563-581).
//
// wreck, when set, is called with the item refs and dictionaries before they
// are written, so a test can make the file say something a file is allowed to
// say and a walk must survive.
func writeOutlined(t *testing.T, pages int, marks []mark,
	wreck func(refs []reader.Ref, items []reader.Dict)) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()

	pageRefs := make([]reader.Ref, pages)
	kids := reader.Array{}
	for i := range pageRefs {
		pageRefs[i] = w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef,
			"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
				reader.Integer(200), reader.Integer(300)},
			"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("0 g 0 0 50 50 re f")}),
		})
		kids = append(kids, pageRefs[i])
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": kids, "Count": reader.Integer(pages)})

	outlinesRef := w.Reserve()
	refs := make([]reader.Ref, len(marks))
	for i := range marks {
		refs[i] = w.Reserve()
	}
	// ⛔ One object that is NOT a dictionary, appended past the items so a
	// wreck can point a link at it: refs[len(marks)]. Everything else this
	// fixture writes is a dictionary or a stream, and a stream's dictionary
	// reads fine — so without this there is no way to make the walk meet a
	// link to a number, which is exactly what a damaged file carries.
	refs = append(refs, w.Add(reader.Integer(42)))

	items := make([]reader.Dict, len(marks))
	for i, m := range marks {
		d := reader.Dict{"Title": reader.String(m.title)}
		if m.page >= 1 && m.page <= pages {
			d["Dest"] = reader.Array{pageRefs[m.page-1], reader.Name("Fit")}
		}
		if m.parent < 0 {
			d["Parent"] = outlinesRef
		} else {
			d["Parent"] = refs[m.parent]
		}
		items[i] = d
	}
	// Siblings share a parent and are linked in order; the first child of a
	// parent is its /First.
	for i := range marks {
		var prev, next = -1, -1
		for j := i - 1; j >= 0; j-- {
			if marks[j].parent == marks[i].parent {
				prev = j
				break
			}
		}
		for j := i + 1; j < len(marks); j++ {
			if marks[j].parent == marks[i].parent {
				next = j
				break
			}
		}
		if prev >= 0 {
			items[i]["Prev"] = refs[prev]
		}
		if next >= 0 {
			items[i]["Next"] = refs[next]
		}
		first, last := -1, -1
		for j := range marks {
			if marks[j].parent == i {
				if first < 0 {
					first = j
				}
				last = j
			}
		}
		if first >= 0 {
			items[i]["First"] = refs[first]
			items[i]["Last"] = refs[last]
		}
	}
	root := reader.Dict{"Type": reader.Name("Outlines")}
	for j := range marks {
		if marks[j].parent < 0 {
			if _, has := root["First"]; !has {
				root["First"] = refs[j]
			}
			root["Last"] = refs[j]
		}
	}
	if wreck != nil {
		wreck(refs, items)
	}
	for i := range items {
		w.Put(refs[i], items[i])
	}
	w.Put(outlinesRef, root)

	cat := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"Outlines": outlinesRef})
	out, err := w.Finish(reader.Dict{"Root": cat})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// openOutlined opens the fixture in a workbench.
//
// ⛔⛔ s.src IS NOT THE FILE THAT WAS OPENED. The workbench writes the document
// out through ops and reads it BACK on every repaint, so that what is shown is
// always what Save would give — which means it has been renumbered and
// rebuilt, and a malformation planted in the fixture is gone before s.src
// exists. A test that sabotaged the file and then asserted against s.src would
// be testing ops's writer, and passing because of it.
//
// So the hostile cases go through rawOutlined below, which opens the bytes
// themselves. This one is for what a reader sees.
func openOutlined(t *testing.T, pages int, marks []mark,
	wreck func([]reader.Ref, []reader.Dict)) *state {
	t.Helper()
	s := newState(surfaceW, surfaceH, &fakeHost{
		name: "contents.pdf", file: writeOutlined(t, pages, marks, wreck)})
	s.open()
	if s.src == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	return s
}

// rawOutlined opens the fixture's bytes directly, malformations and all, which
// is the only way to put the walk in front of what a file may actually say.
func rawOutlined(t *testing.T, pages int, marks []mark,
	wreck func([]reader.Ref, []reader.Dict)) *reader.Document {
	t.Helper()
	src, err := reader.Open(writeOutlined(t, pages, marks, wreck))
	if err != nil {
		t.Fatalf("the fixture does not open: %v", err)
	}
	return src
}

func TestTheContentsAreReadFromTheDocument(t *testing.T) {
	s := openOutlined(t, 4, []mark{
		{"One", 1, -1},
		{"One point one", 2, 0},
		{"Two", 3, -1},
	}, nil)

	got := s.bookmarks(s.src)
	if len(got) != 3 {
		t.Fatalf("a three-entry outline came back as %d: %+v", len(got), got)
	}
	// ⛔ Reading order, with the child between its parent and the next
	// top-level entry — not every top-level entry first. A contents that
	// reordered the document would be worse than none.
	want := []bookmark{
		{title: "One", page: 1},
		{title: "One point one", page: 2, depth: 1},
		{title: "Two", page: 3},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("row %d is %+v, want %+v", i, got[i], w)
		}
	}
}

func TestAContentsRowGoesToItsPage(t *testing.T) {
	s := openOutlined(t, 4, []mark{{"One", 1, -1}, {"Two", 3, -1}}, nil)
	s.railContents = true
	s.refresh()
	s.draw(buffer())

	rows := railRows(s.railView)
	if len(rows) != 2 {
		t.Fatalf("the contents shows %d rows", len(rows))
	}
	railList(s.railView).OnActivate(1)
	if s.at != 3 {
		t.Errorf("pressing the row for page 3 went to page %d", s.at)
	}
}

// railList walks the rail for the contents list, and railRows for what it says.
//
// ⛔ It used to walk for toolkit.Buttons, one per row. The rows are a ListBox
// now — a Button centres its label, which is what "it looks right-aligned
// instead of left" was, and a column of a thousand Buttons is a thousand
// widgets laid out on every rebuild. One consequence for these tests: a ListBox
// lists EVERY row, where a walk for Buttons only found the pressable ones, so a
// row that goes nowhere is now counted and asserted by what pressing it does.
func railList(w toolkit.Widget) *toolkit.ListBox {
	if lb, ok := w.(*toolkit.ListBox); ok {
		return lb
	}
	p, ok := w.(interface{ Children() []toolkit.Widget })
	if !ok {
		return nil
	}
	for _, c := range p.Children() {
		if lb := railList(c); lb != nil {
			return lb
		}
	}
	return nil
}

func railRows(w toolkit.Widget) []string {
	if lb := railList(w); lb != nil {
		return lb.Items
	}
	return nil
}

func TestAnOutlineThatPointsBackAtItselfIsReadOnce(t *testing.T) {
	// An outline is a tree of indirect references and nothing in the format
	// stops a file pointing /Next at an item already visited.
	//
	// ⛔⛔ TWO BOUNDS, AND THEY DO DIFFERENT THINGS — which this test learnt by
	// being wrong. It first asserted only "does not hang", and PASSED with the
	// cycle guard taken out: the walk stops either way, because maxBookmarks
	// stops it. The hang is prevented by the count; the GUARD is what stops a
	// two-entry outline coming back as a thousand rows of "One, Two, One,
	// Two". A test that measures the bound that was not removed is a test that
	// certifies whichever one is left.
	src := rawOutlined(t, 3, []mark{{"One", 1, -1}, {"Two", 2, -1}},
		func(refs []reader.Ref, items []reader.Dict) {
			items[1]["Next"] = refs[0] // the second points back at the first
		})
	s := newState(surfaceW, surfaceH, &fakeHost{})

	done := make(chan []bookmark, 1)
	go func() { done <- s.bookmarks(src) }()
	select {
	case got := <-done:
		if len(got) != 2 {
			t.Errorf("a two-entry outline that loops came back as %d rows: the cycle is "+
				"being walked until some other bound stops it", len(got))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("walking an outline that points back at itself did not finish at all")
	}
}

func TestAnOutlineLongerThanTheBoundIsCutOff(t *testing.T) {
	// ⛔ The other bound, measured on its own, because the test above no longer
	// does. A file may carry as many entries as it likes; a reader cannot read
	// a hundred thousand of them, and building the rows costs a widget each.
	var marks []mark
	for range maxBookmarks + 50 {
		marks = append(marks, mark{"entry", 1, -1})
	}
	s := newState(surfaceW, surfaceH, &fakeHost{})
	if got := len(s.bookmarks(rawOutlined(t, 2, marks, nil))); got != maxBookmarks {
		t.Errorf("an outline of %d entries gave %d rows, and the bound is %d",
			len(marks), got, maxBookmarks)
	}
}

func TestAnOutlineNestedPastTheBoundStops(t *testing.T) {
	// A file may nest as deep as it likes. The walk recurses, so the bound on
	// depth is the one that would otherwise end the process rather than merely
	// waste it.
	var marks []mark
	for i := range maxOutlineDepth + 10 {
		marks = append(marks, mark{"deep", 1, i - 1})
	}
	s := newState(surfaceW, surfaceH, &fakeHost{})
	got := s.bookmarks(rawOutlined(t, 2, marks, nil))
	if len(got) != maxOutlineDepth {
		t.Errorf("an outline %d deep gave %d rows, and the bound is %d",
			len(marks), len(got), maxOutlineDepth)
	}
}

func TestADocumentWithNoOutlineListsItsPagesInstead(t *testing.T) {
	// ⛔ THE REPORT THIS TEST CARRIES: "I still cannot see the table of
	// contents view". The pane used to be withheld from a document with no
	// /Outlines — which is MOST documents, this fixture among them — so the
	// view shipped and was invisible to the person it shipped for. A contents
	// view only the minority of files can open is a contents view nobody
	// finds.
	//
	// What the fallback lists is the pages. That is not a consolation prize: a
	// column of titles is how you reach page 340 of a scanned book, because it
	// scrolls in one flick where four hundred pictures do not.
	s, _ := opened(t, 3) // samplePDF carries no outline
	if got := s.bookmarks(s.src); len(got) != 0 {
		t.Fatalf("a document with no outline gave %d outline rows", len(got))
	}
	s.draw(buffer())
	if s.railView == nil {
		t.Fatal("a three-page document has no rail at all")
	}
	if got := railRows(s.railView); len(got) != 3 {
		t.Errorf("a three-page document with no outline offered %d places to go, and it has 3 pages", len(got))
	}
	if splitterIn(s.railView) == nil {
		t.Error("the contents and the minipages share the rail with no divider between them")
	}
	if got := len(railTiles(s.railView)); got != 3 {
		t.Errorf("the rail shows %d minipages of 3", got)
	}
	// And the rows go somewhere: a list that cannot be pressed is a picture.
	s.goTo(1)
	rows := s.pageList(s.src)
	if len(rows) != 3 || rows[2].page != 3 || rows[2].title != "Page 3" {
		t.Errorf("the page list reads %+v", rows)
	}
}

// A document that labels its pages something other than their position says
// both, because the label is what its own index refers to and the position is
// what every other control in the workbench counts in.
func TestAPageListSaysTheDocumentsOwnLabel(t *testing.T) {
	s, _ := opened(t, 3)
	if err := s.doc.SetPageLabels([]ops.LabelRule{{Style: ops.LabelLowerRoman}}); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	rows := s.pageList(s.src)
	if len(rows) != 3 {
		t.Fatalf("the page list has %d rows", len(rows))
	}
	if rows[2].title != "Page iii (3)" {
		t.Errorf("page 3 of a roman-numbered document reads %q", rows[2].title)
	}
	// ⛔ The control for the branch that does NOT add the label: a document
	// whose labels are already its page numbers must not read "Page 3 (3)".
	plain, _ := opened(t, 2)
	if got := plain.pageList(plain.src)[1].title; got != "Page 2" {
		t.Errorf("an unlabelled page reads %q", got)
	}
}

func TestABookmarkThatGoesNowhereKeepsItsRow(t *testing.T) {
	// ⛔ Keeps its row: it is still what the document says about itself, and
	// dropping it would renumber every row under it against a reader's own
	// mental list. It loses its press, because a press that went nowhere is a
	// control that does not work rather than one that is off.
	s := openOutlined(t, 3, []mark{{"Real", 1, -1}, {"Broken", 0, -1}}, nil)
	got := s.bookmarks(s.src)
	if len(got) != 2 {
		t.Fatalf("a two-entry outline gave %d rows: %+v", len(got), got)
	}
	if got[1].page != 0 {
		t.Errorf("an entry with no destination resolved to page %d", got[1].page)
	}
	s.railContents = true
	s.refresh()
	s.draw(buffer())
	// ⛔ BOTH rows are listed, and the broken one does NOTHING when pressed.
	// The list shows what the document says about itself; the press is what is
	// withheld. Counting pressable widgets could not tell "the row is gone"
	// from "the row is there and inert", and those are different bugs.
	if n := len(railRows(s.railView)); n != 2 {
		t.Errorf("the contents lists %d rows, and the outline has 2", n)
	}
	was := s.at
	railList(s.railView).OnActivate(1)
	if s.at != was {
		t.Errorf("pressing a row that goes nowhere moved the reader to page %d", s.at)
	}
}

func TestAGoToActionIsADestinationToo(t *testing.T) {
	// Most producers write /A with a /GoTo action rather than /Dest.
	s := newState(surfaceW, surfaceH, &fakeHost{})
	src := rawOutlined(t, 3, []mark{{"Via an action", 0, -1}},
		func(refs []reader.Ref, items []reader.Dict) {
			delete(items[0], "Dest")
			items[0]["A"] = reader.Dict{"S": reader.Name("GoTo"),
				"D": reader.Array{reader.Ref{Num: 0}, reader.Name("Fit")}}
		})
	// The ref above is deliberately not a page: an action pointing at
	// something that is not a page in this document resolves to nowhere rather
	// than to page 1 by accident.
	got := s.bookmarks(src)
	if len(got) != 1 {
		t.Fatalf("one entry gave %d rows", len(got))
	}
	if got[0].page != 0 {
		t.Errorf("an action pointing outside the document resolved to page %d", got[0].page)
	}
}

func TestAFileMaySayAnythingAndTheContentsStillRead(t *testing.T) {
	// ⛔ Every one of these is a shape a PDF is allowed to have, or at least to
	// contain, and none of them may stop the walk. They are gathered because
	// each is one branch: a walk with an unexercised error path is a walk whose
	// error path is a guess.
	long := ""
	for range outlineTitleMax + 50 {
		long += "x"
	}
	s := newState(surfaceW, surfaceH, &fakeHost{})

	for _, c := range []struct {
		what  string
		wreck func([]reader.Ref, []reader.Dict)
		rows  int
		check func(t *testing.T, got []bookmark)
	}{
		{"a title that is not there", func(_ []reader.Ref, items []reader.Dict) {
			delete(items[0], "Title")
		}, 1, func(t *testing.T, got []bookmark) {
			if got[0].title != outlineUntitled {
				t.Errorf("an entry with no title reads %q", got[0].title)
			}
		}},
		{"a title longer than any line", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Title"] = reader.String(long)
		}, 1, func(t *testing.T, got []bookmark) {
			if n := utf8.RuneCountInString(got[0].title); n > outlineTitleMax {
				t.Errorf("a title of %d characters came through at %d, and the bound is %d",
					utf8.RuneCountInString(long), n, outlineTitleMax)
			}
		}},
		{"a long title that is not ASCII", func(_ []reader.Ref, items []reader.Dict) {
			// ⛔ Cut by bytes, this splits a character in half and the row draws
			// a replacement glyph the document never contained. PDF text
			// strings are commonly UTF-16BE, so this is not an exotic case.
			s := ""
			for range outlineTitleMax + 50 {
				s += "é"
			}
			items[0]["Title"] = reader.String(s)
		}, 1, func(t *testing.T, got []bookmark) {
			if n := utf8.RuneCountInString(got[0].title); n > outlineTitleMax {
				t.Errorf("the clipped title is %d characters, and the bound is %d",
					n, outlineTitleMax)
			}
			if !utf8.ValidString(got[0].title) {
				t.Errorf("the clipped title is not valid text: %q", got[0].title)
			}
		}},
		{"a link that is not a reference", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Next"] = reader.Integer(3)
		}, 1, nil},
		{"a link to an object that is not there", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Next"] = reader.Ref{Num: 99999}
		}, 1, nil},
		{"a link to something that is not a dictionary", func(refs []reader.Ref, items []reader.Dict) {
			items[0]["Next"] = refs[len(refs)-1] // the scalar
		}, 1, nil},
		{"an action that is not a GoTo", func(_ []reader.Ref, items []reader.Dict) {
			delete(items[0], "Dest")
			items[0]["A"] = reader.Dict{"S": reader.Name("URI"),
				"URI": reader.String("https://example.invalid")}
		}, 1, func(t *testing.T, got []bookmark) {
			if got[0].page != 0 {
				t.Errorf("a link out to the web resolved to page %d of this document", got[0].page)
			}
		}},
		{"a destination that is not an array", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Dest"] = reader.Name("somewhere")
		}, 1, func(t *testing.T, got []bookmark) {
			if got[0].page != 0 {
				t.Errorf("a named destination resolved to page %d", got[0].page)
			}
		}},
		{"a destination array that is empty", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Dest"] = reader.Array{}
		}, 1, nil},
		{"a destination whose first element is not a page", func(_ []reader.Ref, items []reader.Dict) {
			items[0]["Dest"] = reader.Array{reader.Name("Fit")}
		}, 1, nil},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := s.bookmarks(rawOutlined(t, 2, []mark{{"One", 1, -1}}, c.wreck))
			if len(got) != c.rows {
				t.Fatalf("%s gave %d rows, want %d: %+v", c.what, len(got), c.rows, got)
			}
			if c.check != nil {
				c.check(t, got)
			}
		})
	}
}

func TestANestedContentsRowIsIndented(t *testing.T) {
	s := openOutlined(t, 3, []mark{{"One", 1, -1}, {"Under it", 2, 0}}, nil)
	s.railContents = true
	s.refresh()
	s.draw(buffer())
	rows := railRows(s.railView)
	if len(rows) != 2 {
		t.Fatalf("the contents shows %d rows", len(rows))
	}
	// ⛔ The nesting is what makes a contents a contents rather than a list.
	if len(rows[1]) <= len("Under it") {
		t.Errorf("the nested row reads %q, which is not indented", rows[1])
	}
}

func TestTheContentsAndTheMinipagesAreBothThere(t *testing.T) {
	// ⛔⛔ THE WHOLE POINT, and the thing the first version of this feature got
	// wrong: the contents went IN PLACE OF the minipages. They are different
	// answers to the same question — the contents say what a part is called,
	// the minipages say what it looks like — and a reader after "the table
	// near the start" is served by both at once and by neither alone.
	s := openOutlined(t, 6, []mark{{"One", 1, -1}, {"Two", 4, -1}}, nil)
	s.draw(buffer())

	rows := railRows(s.railView)
	if len(rows) != 2 {
		t.Errorf("the rail offers %d contents rows, and the document has 2", len(rows))
	}
	tiles := railTiles(s.railView)
	if len(tiles) != 6 {
		t.Errorf("the rail shows %d minipages, and the document has 6 pages", len(tiles))
	}
	// And they are stacked, not overlaid: the divider between them exists.
	sp := splitterIn(s.railView)
	if sp == nil {
		t.Fatal("the contents and the minipages share the rail with no divider between them")
	}
	if sp.Handles() != 1 {
		t.Errorf("the rail's splitter has %d dividers", sp.Handles())
	}
}

func TestTheContentsPaneCanBeDraggedTaller(t *testing.T) {
	// ⛔ Dragged, not Set — the lesson that cost a release. The divider between
	// the contents and the minipages is the same kind of control as the one
	// beside the page, and the only way to know it works is to press it, move
	// and let go.
	s := openOutlined(t, 8, []mark{{"One", 1, -1}, {"Two", 4, -1}, {"Three", 7, -1}}, nil)
	s.draw(buffer())
	before := s.contentsH.Get()

	dragHandle(t, s, splitterIn(s.railView), 0, toolkit.Scaled(60))

	if s.contentsH.Get() <= before {
		t.Errorf("the divider was dragged 60 down and the contents pane is still %d tall "+
			"(was %d)", s.contentsH.Get(), before)
	}
	// And the minipages are still there underneath, which is the whole
	// arrangement.
	s.refresh()
	s.draw(buffer())
	if got := len(railTiles(s.railView)); got != 8 {
		t.Errorf("after the drag the rail shows %d minipages of 8", got)
	}
}

func TestPuttingTheContentsAwayLeavesTheMinipages(t *testing.T) {
	s := openOutlined(t, 4, []mark{{"One", 1, -1}}, nil)
	s.draw(buffer())
	if len(railRows(s.railView)) == 0 {
		t.Fatal("precondition: the contents should be up")
	}

	s.toggleContents()
	s.refresh()
	s.draw(buffer())
	if got := len(railRows(s.railView)); got != 0 {
		t.Errorf("the contents were put away and %d rows are still there", got)
	}
	if got := len(railTiles(s.railView)); got != 4 {
		t.Errorf("putting the contents away left %d minipages of 4", got)
	}
}

func TestTheSwitchIsOfferedForAnyOpenDocument(t *testing.T) {
	empty := newState(surfaceW, surfaceH, &fakeHost{})
	if empty.acts["contents"].IsEnabled() {
		t.Error("the contents switch is offered with no document to show the contents of")
	}

	// ⛔ A document with no outline gets the switch TOO, and the tooltip says
	// what the list will be. It used to be greyed here — for most documents,
	// which is how the view came to be reported missing by somebody running
	// the build that added it.
	plain, _ := opened(t, 3)
	if !plain.acts["contents"].IsEnabled() {
		t.Error("the contents switch is greyed for a document that carries no outline")
	}
	plain.toggleContents() // down, so the tooltip says what the next press does
	// ⛔ And it says LINKS, because that is what the fallback lists. It said
	// "pages" first, and the person who asked for the view said that was not
	// what they meant by a table of contents.
	if !strings.Contains(plain.contentsCtl.Tip, "list the links this document carries") {
		t.Errorf("the switch says %q, which does not say what pressing it lists",
			plain.contentsCtl.Tip)
	}
	plain.toggleContents()

	s := openOutlined(t, 3, []mark{{"One", 1, -1}}, nil)
	s.draw(buffer())
	if !s.acts["contents"].IsEnabled() {
		t.Fatal("the contents switch is greyed for a document that carries contents")
	}
	icon, tip := s.contentsIcon(), s.contentsCtl.Tip
	s.toggleContents()
	if s.railContents {
		t.Fatal("pressing the switch did not put the contents away")
	}
	if s.contentsIcon() == icon {
		t.Errorf("both views carry the mark %q", icon)
	}
	if s.contentsCtl.Tip == tip {
		t.Errorf("both views carry the tooltip %q", tip)
	}
	have := map[string]bool{}
	for _, n := range toolkit.IconoirNames() {
		have[n] = true
	}
	for _, n := range []string{"list", "media-image-list"} {
		if !have[n] {
			t.Errorf("the switch asks for the mark %q, which the pack does not have: it "+
				"would draw NOTHING, silently", n)
		}
	}
}
