// The document's own table of contents: the /Outlines tree a PDF carries so a
// viewer can offer bookmarks.
//
// ⛔ EVERY LINK IN IT COMES OUT OF THE FILE. An outline is a doubly-linked tree
// of indirect references — /First and /Next and /Parent — and nothing in the
// format stops a file pointing /Next back at an item already visited, or
// nesting a thousand deep, or carrying a hundred thousand entries. A walk that
// trusted them would hang the tab on a document somebody was handed, which is
// the whole threat model of a workbench whose point is that nothing leaves it.
// Hence seen, maxBookmarks and maxOutlineDepth below: bounded on all three
// axes, and the bound is reported rather than hidden.
//
// The structure is the one go-pdfkit/pdfkit writes (document.go:563-581): an
// item is a /Title and a /Dest of [pageRef /Fit] or [pageRef /XYZ x y null],
// with /Parent, /Prev, /Next, /First, /Last and /Count links. A destination may
// also arrive as an /A action dictionary, which is what most other producers
// emit.

package main

import (
	"fmt"
	"strconv"

	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// A bookmark is one row of the contents: what it says, which page it goes to,
// and how deeply it is nested.
//
// uri is set only on a row standing for a LINK that leaves the document. Such a
// row has no page to go to, and pressing it says where it would go rather than
// going: a workbench whose whole promise is that nothing leaves this tab does
// not navigate the tab away from itself.
type bookmark struct {
	title string
	page  int
	depth int
	uri   string
}

// The bounds on what a file may ask this walk to do.
//
// A thousand entries is more than any table of contents a person reads, and
// twenty levels is deeper than any document is structured; both are here to
// stop a file, not a book. maxOutlineDepth also bounds the recursion itself,
// which is the one of the three that would otherwise end the process rather
// than merely waste it.
const (
	maxBookmarks = 1000
	// maxOutlineDepth bounds the recursion as well as the nesting, which is
	// the one of the three bounds that would otherwise end the process rather
	// than merely waste it.
	maxOutlineDepth = 20
	// outlineTitleMax is counted in RUNES, the ellipsis included.
	outlineTitleMax = 200
	outlineEllipsis = "…"
	outlineUntitled = "(untitled)"
)

// toggleContents shows or hides the contents pane above the minipages.
//
// ⛔ It rebuilds the rail rather than refreshing: refresh re-reads the whole
// document and redraws every page, and nothing about the document has changed
// — only whether one of two views of it is on the screen.
func (s *state) toggleContents() {
	s.railContents = !s.railContents
	s.contentsCtl.Tip = s.contentsTip()
	s.railView, s.railMore = nil, false
	s.dirty = true
}

// settleContentsSwitch puts the switch in the state this document deserves:
// offered only when there are contents to show, and saying so either way.
// It is only ever called from refresh, which newState runs after the ribbon is
// built, so the switch always exists by the time this does anything — no guard
// for a nil one, because a branch nothing can reach is not a guard.
func (s *state) settleContentsSwitch() {
	s.contentsCtl.Tip = s.contentsTip()
	s.acts["contents"].Enabled.Set(s.mayRun(s.rules["contents"]))
}

// contentsIcon and contentsTip say whether the contents are up and what the
// next press does, which is the whole of what the switch's fixed label cannot.
func (s *state) contentsIcon() string {
	if s.railContents {
		return "media-image-list"
	}
	return "list"
}

func (s *state) contentsTip() string {
	switch {
	case s.railContents:
		return "The contents are above the minipages. Press to put them away."
	case s.opened() && len(s.marks) == 0:
		// ⛔ It says WHAT THE LIST WILL BE. The switch used to be GREYED here,
		// with this tooltip explaining that the document carried no outline —
		// and since most documents carry none, the view was reported as
		// missing by somebody who had the build that added it. A contents view
		// only the minority of files can open is a contents view nobody finds.
		return "Press to list the links this document carries above the minipages; it has no outline of its own."
	}
	return "Press to show the document's contents above the minipages."
}

// contentsOf is what the contents pane lists, in order of preference: the
// document's own outline, then the LINKS it carries, and a list of its pages
// only when it has neither.
//
// ⛔⛔ THE FALLBACK IS THE LINKS, AND IT WAS A LIST OF PAGES FIRST. That was
// wrong, and the person who asked for this view said so as soon as they saw it:
// what they meant by a table of contents was a display of the document's links.
// The difference is not presentational. A link is structure the AUTHOR wrote —
// this phrase leads there — and the whole value of a contents view is that
// somebody decided those places were worth naming. A list of pages is
// arithmetic over the document: it is the same list for every file of the same
// length, and it tells a reader nothing they could not get by counting.
//
// Most documents carry no outline, which is why there is a fallback at all. A
// great many of those do carry links.
// ⛔ It takes the document's STAMP and reads both memos itself, rather than
// being handed a list. It used to take the marks, which left every caller free
// to pass a freshly walked list — and the links branch below had no memo at
// all, so the one caller that mattered re-read the text of every linked page on
// every frame. A signature nobody can call the expensive way is the fix; a
// comment asking callers to remember is not.
func (s *state) contentsOf(src *reader.Document, stamp [2]uint64) []bookmark {
	if marks := s.marksFor(src, stamp); len(marks) > 0 {
		return marks
	}
	if links := s.linksFor(src, stamp); len(links) > 0 {
		return links
	}
	return s.pageList(src)
}

// pageList is one row per page, named the way the document names it.
//
// A document may label its pages something other than their position — "iv",
// "A-2", a plate number — and those labels are what its own index refers to,
// so a row shows the label and the position together when they differ. The
// rows are NOT bounded the way the outline is: the outline's bound exists
// because a file chooses that tree and can make it enormous or circular, and
// a page list is as long as the document the rail is already drawing a tile
// for each page of.
func (s *state) pageList(src *reader.Document) []bookmark {
	n := src.PageCount()
	var labels []string
	if s.doc != nil {
		labels = s.doc.PageLabels()
	}
	out := make([]bookmark, 0, n)
	for i := 1; i <= n; i++ {
		title := fmt.Sprintf("Page %d", i)
		if i <= len(labels) && labels[i-1] != "" && labels[i-1] != strconv.Itoa(i) {
			title = fmt.Sprintf("Page %s (%d)", labels[i-1], i)
		}
		out = append(out, bookmark{title: title, page: i})
	}
	return out
}

// contentsPane is the document's own table of contents: one row per bookmark,
// indented by its depth, each a press that goes to its page.
//
// ⛔⛔ A LIST, NOT A COLUMN OF BUTTONS, AND THAT IS TWO OF THREE REPORTED
// DEFECTS. It was a VBox of toolkit.Buttons inside a ScrollView, which is wrong
// twice over:
//
//   - A Button is "a clickable rectangle with a CENTRED label" (its own doc).
//     A column of centred rows of different lengths has no edge to read down,
//     and that is what came back as "it looks right-aligned instead of left".
//     A ListBox draws every row from a fixed left margin.
//   - A VBox builds one widget per row and a ScrollView lays out and paints all
//     of them, so a thousand-row contents cost a thousand widgets every rebuild.
//     A ListBox scrolls VIRTUALLY — it paints only the rows that fit in its
//     bounds, so "a list with thousands of rows costs the same per frame as one
//     with a handful". The rail is rebuilt on every page change, so that cost
//     was being paid over and over.
func (s *state) contentsPane(marks []bookmark) toolkit.Widget {
	items := make([]string, len(marks))
	for i, m := range marks {
		items[i] = s.contentsLabel(m)
	}
	lb := toolkit.NewListBox(items)
	lb.RowHeight = lineH
	// Which row is lit is WHERE THE READER IS, the way a minipage is lit in the
	// other view. The first row for the page, because an outline may name the
	// same page twice and the first is the one that starts it.
	for i, m := range marks {
		if m.page == s.at && m.uri == "" {
			lb.Selected().Set(i)
			break
		}
	}
	lb.OnActivate = func(i int) {
		if i >= 0 && i < len(marks) {
			s.pressContents(marks[i])
		}
	}
	return lb
}

// contentsLabel is what one row SAYS: its depth as an indent, and its title cut
// to what the pane is currently wide enough to show.
//
// ⛔ Cut to the PANE, not to a number chosen beside it. The rows were clipped to
// linkTitleMax runes — two hundred — in a rail 132 logical pixels wide, so every
// row ran off its pane and the view was reported "unusable as it is". The width
// here is the one the reader has dragged the divider to, so widening the rail
// shows more of each row instead of the same cut text in a wider box.
func (s *state) contentsLabel(m bookmark) string {
	label := m.title
	for range m.depth {
		label = "  " + label
	}
	return clipToWidth(label, s.contentsTextW())
}

// contentsTextW is how much width a row's text actually has: the rail, less the
// bar the list scrolls with and the margin it draws from.
func (s *state) contentsTextW() int {
	return s.railPx() - scrollbarW - 2*contentsGutter
}

// contentsGutter is the margin a row's text is drawn from, and so also the
// furthest a top-level row may start from the pane's left edge and still read
// as a column. ⛔ It is what a test measures the rows against: the property is
// where the text LANDS, and a test naming the widget would have passed on the
// centred buttons this replaced.
var contentsGutter = 6

// clipToWidth cuts a string to what fits in px, ending it with an ellipsis when
// it had to cut. It measures rather than counting characters, because the rail
// is a width in pixels and "how many letters fit" is not a constant.
func clipToWidth(s string, px int) string {
	if px <= 0 {
		return ""
	}
	if toolkit.TextWidth(s) <= px {
		return s
	}
	r := []rune(s)
	// Start from a proportional guess and walk from there. One rune at a time
	// from the end would measure a thousand rows of two hundred characters two
	// hundred thousand times, every time the divider moves.
	n := min(len(r), len(r)*px/max(1, toolkit.TextWidth(s)))
	for n > 0 && toolkit.TextWidth(string(r[:n])+outlineEllipsis) > px {
		n--
	}
	for n < len(r) && toolkit.TextWidth(string(r[:n+1])+outlineEllipsis) <= px {
		n++
	}
	if n <= 0 {
		return ""
	}
	return string(r[:n]) + outlineEllipsis
}

// pressContents is what a row does when it is pressed.
//
// ⛔ A LINK OUT OF THE DOCUMENT IS SHOWN AND NOT FOLLOWED. Pressing it says
// where it would go, in the status line. The whole promise of this workbench is
// that nothing leaves this tab, and navigating the tab to somebody else's URL is
// the one gesture that would break it — silently, and in the one direction a
// reader cannot undo.
//
// ⛔ And a row whose destination this document cannot resolve keeps its row and
// does nothing. The row is still what the document says about itself, and
// dropping it would renumber everything under it; but a press that went nowhere
// would be a control that does not work rather than one that is off.
func (s *state) pressContents(m bookmark) {
	if m.uri != "" {
		s.note = "that link leaves the document: " + m.uri
		s.status = toolkit.NewStatusbar(s.statusLine())
		s.dirty = true
		return
	}
	if m.page == 0 {
		return
	}
	s.goTo(m.page)
}

// contentsMinH is the shortest the contents pane may be dragged to and still
// show a row; below it the pane says less than its own scrollbar costs. There
// is no maximum: the splitter's flexible pane keeps the minipages from being
// squeezed out, and a reader of a long table of contents may want most of the
// rail for it.
const (
	contentsMinH = 48
	// contentsLogicalH is how tall the pane starts: a handful of rows, enough
	// to see that there IS a table of contents and roughly how it is shaped,
	// with the minipages still filling the rest of the rail.
	contentsLogicalH = 160
)

// marksFor is the document's contents, remembered between repaints.
//
// ⛔ Walked ONCE per document, not once per frame. The scene repaints on a
// pointer move and on every frame the rail is still filling, and the walk
// follows up to a thousand indirect references through the file — cheap beside
// rendering a page, and not cheap sixty times a second for a document nobody
// is changing.
func (s *state) marksFor(src *reader.Document, stamp [2]uint64) []bookmark {
	if s.marksDoc == stamp && s.marks != nil {
		return s.marks
	}
	s.marksDoc = stamp
	// Never nil once read, so a document that genuinely has no contents is
	// remembered as having none rather than walked again on every frame.
	s.marks = append([]bookmark{}, s.bookmarks(src)...)
	return s.marks
}

// bookmarks is the document's contents, flattened in reading order with each
// row's depth kept. It is empty when the document carries no outline, which is
// most of them.
func (s *state) bookmarks(src *reader.Document) []bookmark {
	// ⛔ The errors are dropped rather than branched on, and that is the whole
	// of what this function has to say about them: it answers "the document's
	// contents, or none", and a catalog that cannot be read is none. A branch
	// that returned early would be a second way of saying the same thing, and
	// one no test could reach — a document that did not open never gets here,
	// and a document that did has a catalog.
	cat, _ := src.Catalog()
	root, ok := src.GetDict(cat, "Outlines")
	if !ok {
		return nil
	}
	pages := pageNumbers(src)
	var out []bookmark
	seen := map[reader.Ref]bool{}
	walkOutline(src, root, pages, seen, 0, &out)
	return out
}

// pageNumbers maps each page's object reference to its 1-based number, which is
// what turns a destination into somewhere to go. PageRef is the inverse of
// Page, and the only way back from the reference a /Dest carries.
func pageNumbers(src *reader.Document) map[reader.Ref]int {
	out := make(map[reader.Ref]int, src.PageCount())
	for i := 1; i <= src.PageCount(); i++ {
		if ref, ok := src.PageRef(i); ok {
			out[ref] = i
		}
	}
	return out
}

// walkOutline follows one level's /First … /Next chain, recursing into each
// item's own children.
//
// ⛔ seen is keyed on the REFERENCE, not on the item, and it is shared across
// the whole walk rather than per level. A file can point /Next at an item from
// another branch just as easily as at one from this one, and a per-level guard
// would follow that for ever.
func walkOutline(src *reader.Document, parent reader.Dict, pages map[reader.Ref]int,
	seen map[reader.Ref]bool, depth int, out *[]bookmark) {
	if depth >= maxOutlineDepth {
		return
	}
	ref, ok := refOf(parent.Get("First"))
	for ok && len(*out) < maxBookmarks {
		if seen[ref] {
			return // the chain has come back on itself
		}
		seen[ref] = true
		item, got := dictAt(src, ref)
		if !got {
			return
		}
		*out = append(*out, bookmark{
			title: outlineTitle(item),
			page:  destPage(src, item, pages),
			depth: depth,
		})
		walkOutline(src, item, pages, seen, depth+1, out)
		ref, ok = refOf(item.Get("Next"))
	}
}

// refOf is the object reference an outline link carries, if it carries one.
// ⛔ A link that is not a reference is not a link: the tree is built out of
// indirect references, and anything else in that slot is a file saying
// something this walk has no way to follow.
func refOf(o reader.Object) (reader.Ref, bool) {
	r, ok := o.(reader.Ref)
	return r, ok
}

// dictAt fetches the dictionary an outline link points at.
// A link to an object that is not there and a link to one that is not a
// dictionary are the same answer here, so there is one path for both: Get
// returns nothing for a missing object and ToDict refuses it.
func dictAt(src *reader.Document, ref reader.Ref) (reader.Dict, bool) {
	o, _ := src.Get(ref)
	return reader.ToDict(o)
}

// outlineTitle is what the row says. A title is a PDF text string, which is to
// say bytes a file chose: it is clipped, and an item with none is still a row,
// because a row missing from the contents would make every row after it point
// at the wrong line of a reader's own mental list.
func outlineTitle(item reader.Dict) string {
	s, ok := reader.TextString(item.Get("Title"))
	if !ok || s == "" {
		return outlineUntitled
	}
	// ⛔ Clipped by RUNES, not bytes. A PDF text string is commonly UTF-16BE,
	// so a title is as likely to be Cyrillic or Japanese as ASCII, and cutting
	// it at a byte offset would split a character in half — leaving the row to
	// draw a replacement glyph that the document never contained. Counting in
	// bytes also made the bound a lie: the first version of this cut at
	// outlineTitleMax-1 bytes and then appended a three-byte ellipsis, so a
	// "200" limit returned 202.
	r := []rune(s)
	if len(r) > outlineTitleMax {
		return string(r[:outlineTitleMax-1]) + outlineEllipsis
	}
	return s
}

// destPage is the page an item goes to, or 0 when it does not say or says
// something this document cannot resolve.
//
// Two spellings, because producers differ: /Dest carries the destination
// itself, and /A carries an action that carries it. go-pdfkit/pdfkit writes
// the first (document.go:569); most other producers write the second.
func destPage(src *reader.Document, item reader.Dict, pages map[reader.Ref]int) int {
	if n := destArrayPage(src, item.Get("Dest"), pages); n != 0 {
		return n
	}
	action, ok := src.GetDict(item, "A")
	if !ok {
		return 0
	}
	// Only /GoTo. An action that opens a URL or runs a file is not a place in
	// this document, and a row that silently did nothing would be worse than a
	// row that says where it goes and goes nowhere.
	if name, ok := reader.ToName(mustResolve(src, action.Get("S"))); !ok || name != "GoTo" {
		return 0
	}
	return destArrayPage(src, action.Get("D"), pages)
}

// destArrayPage reads an explicit destination: an array whose FIRST element is
// the page it refers to. A named destination (a name or a string, looked up in
// the catalog's /Dests) is not followed — it would be a second tree out of the
// same file, with the same cycle problem, for rows this reader has not met in
// practice.
func destArrayPage(src *reader.Document, o reader.Object, pages map[reader.Ref]int) int {
	arr, ok := reader.ToArray(o)
	if !ok || len(arr) == 0 {
		return 0
	}
	ref, ok := refOf(arr[0])
	if !ok {
		return 0
	}
	return pages[ref]
}
