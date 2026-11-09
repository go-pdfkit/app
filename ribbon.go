// The ribbon: every verb the workbench offers, named, grouped and on a tab.
//
// It replaces a strip of thirteen buttons. That strip had reached two thirds of
// the way across the window and could not grow: six of its thirteen controls
// were not verbs at all but switches that opened a panel of verbs beside the
// page, so the twenty-five things this library actually does were two presses
// away and named nowhere a reader could see them.
//
// ⛔ THE RIBBON CARRIES THE VERBS; THE PANEL CARRIES WHAT THEY ARE TOLD.
// That is the whole rule, and it is what decides where a control goes. "Turn
// them" is a verb and sits on the ribbon; "a quarter / a half / three quarters"
// is what it is told and sits in the panel beside the page, under the tab that
// commands it. Before this, a panel held both -- a caption, a box, and a button
// under it -- which is why every panel was a scrolling column that ran off its
// own bottom edge, and why TestNoPanelOverflowsTheRoomItIsGiven had to exist.
//
// A tab IS the panel: choosing "Marks" opens the Marks panel beside the page,
// because a group of verbs and the settings those verbs read are the same
// subject seen twice. Nothing switches a panel on any more.

package main

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// A verb is one command: what it is called, what it does, when it may run, and
// the mark it carries.
//
// ⛔ A TABLE AND NOT A TREE OF CALLS. Every verb is declared in one place, so
// the ribbon, the test that presses all of them and the test that checks every
// mark is one the pack has are all reading the same list. The strip's icons
// used to live in a map keyed by the button's caption, two hundred lines away
// from the button -- renaming a control silently dropped its mark.
type verb struct {
	// label is what the control says, and what Ribbon.ControlRect finds it by.
	label string
	// tip is the sentence under the name in the tooltip. It says what the verb
	// does to the document, not what it is called again.
	tip string
	// icon is an Iconoir stem. TestEveryMarkOnTheRibbonIsOneThePackHas is the
	// control: an unknown name draws NOTHING, silently.
	icon string
	// large asks for the big form, icon above text. One per group: the group's
	// primary command, which is the Windows ribbon's own rule.
	large bool
	// run is the command body.
	run func(*state)
	// when says whether it may run at all; nil means "whenever a document is
	// open", which is true of all but three of them.
	when func(*state) bool
}

// The tabs, in the order they are offered. The first is the document itself;
// the rest are the groups of verbs, and each one opens its panel beside the
// page when it is chosen.
//
// ⛔ The group tabs are named by groupNames and IN THAT ORDER, because
// tabGroup turns a tab index into a panel name by indexing it. A test asserts
// the two lists agree rather than a comment asking the next reader to remember.
const tabHome = "Home"

// ribbonTabs is the shape of the ribbon: which verbs sit in which group of
// which tab.
var ribbonTabs = []struct {
	label  string
	groups []verbGroup
}{
	{tabHome, []verbGroup{
		{"Page", []string{"rotate", "previous", "next", "delete"}},
		{"Form", []string{"fill"}},
	}},
	{groupPages, []verbGroup{
		{"Which pages", []string{"keep", "drop"}},
		{"Turn", []string{"turn", "reverse"}},
		{"Order", []string{"move"}},
		{"Shape", []string{"crop", "insert", "dropBlank", "deskew", "resize"}},
		{"Ink", []string{"recolour"}},
		{"Split", []string{"split"}},
	}},
	{groupSheet, []verbGroup{
		{"To a sheet", []string{"nup", "booklet"}},
		{"From a sheet", []string{"divide", "onepage"}},
		{"Larger than the paper", []string{"poster"}},
		{"Another file", []string{"merge", "overlay", "interleave"}},
	}},
	{groupMarks, []verbGroup{
		{"Watermark", []string{"watermark"}},
		{"Numbers", []string{"number", "bates"}},
	}},
	// ⛔⛔ THE STAMP, THE HEADER AND FOOTER AND THE PICTURE MOVED OFF MARKS
	// because its PANEL ran out of room: adding the two boxes a header and a
	// footer need took it 109 pixels past the room it is given, and a control
	// that can only be scrolled to is one a reader does not know is there. The
	// line is the question each verb asks — Marks writes the same thing across
	// every page by a rule, these put something where you CHOOSE — and the
	// chooser that answers it is on this panel with them.
	//
	// ⛔ The group may not be called "Stamp" here: that is this tab's own name,
	// and a group named like its tab reports no rectangle at all once the strip
	// compacts. See scene_test.go for what that cost twice.
	{groupStamp, []verbGroup{
		{"A line of text", []string{"stamp", "headFoot"}},
		{"Picture", []string{"placePicture"}},
	}},
	{groupFile, []verbGroup{
		{"Clean", []string{"sanitize", "flatten", "dropAnnots"}},
		{"Files inside", []string{"attach", "showCarried"}},
		{"Size", []string{"compress"}},
		{"About it", []string{"setInfo", "showInfo", "clearInfo"}},
	}},
	{groupContents, []verbGroup{
		// ⛔ `dropOutlines` CAME HERE OUT OF CLEAN, where it was the only thing
		// the workbench could do to a table of contents. Clean is what a file is
		// stripped of before it is sent; a reader deciding what the bookmarks
		// should SAY is asking a different question, and the three answers to it
		// belong together — the ribbon's own rule is one command on one tab.
		{"Bookmarks", []string{"bookmarkPage", "bookmarksFromFile", "dropOutlines"}},
		// ⛔ Its own group, and not a fourth control under Bookmarks: those three
		// change the tree a reader NAVIGATES by, this one writes a page of the
		// document that gets printed. Same subject, different act — and the one
		// that adds a page belongs where that is plain.
		{"Printed", []string{"contentsPage"}},
	}},
	{groupRedraw, []verbGroup{
		// ⛔ "The pixels", not "Redraw": a group may not be named like its own
		// tab — see the gate in scene_test.go and what it cost once.
		{"The pixels", []string{"invert", "greyscale", "onGround", "adjust", "scanned"}},
	}},
	{groupProtect, []verbGroup{
		{"Password", []string{"encrypt", "decrypt"}},
		{"Check", []string{"protection"}},
	}},
	{groupRead, []verbGroup{
		// ⛔ "Readings" and not "Read": a group whose label is its own TAB's
		// label cannot be told from the tab's button, and the reachability gate
		// excused this group for exactly that reason while its four controls
		// were not drawn at all. See the gate for what it now refuses.
		// ⛔⛔ "Readings" AND NOT "Read", WHICH IS THIS TAB'S OWN NAME. A group
		// whose label is its tab's label is ambiguous to the ribbon: when the
		// strip grew crowded enough to compact, these four controls reported NO
		// RECTANGLE AT ALL — the fold that holds them is a button called "Read",
		// which cannot be told from the tab called "Read". Renaming the group
		// put all four back at full size with nothing else changed, measured
		// twice in both directions. Width was only what triggered the
		// compaction; the name is what made it a disappearance.
		{"Readings", []string{"readText", "readImages", "readPage", "readTables"}},
		{"Search", []string{"find", "findNext"}},
		{"Selected", []string{"copyText"}},
	}},
	{groupMarkUp, []verbGroup{
		// ⛔ "Text", not "Mark up": a group whose label is its own TAB's label
		// is ambiguous to the ribbon, and its controls report no rectangle once
		// the strip compacts. That cost four readings an afternoon ago; the gate
		// in scene_test.go now refuses the shape outright.
		{"Text", []string{"highlight", "underline", "strikeOut", "squiggle"}},
		// ⛔ Apart from the four above, which write TEXT markup: those follow
		// the words line by line and a reader asks for them by saying "this
		// sentence". These three put a SHAPE over a region, asked for by saying
		// "this bit of the page". Same gesture, different question.
		{"Shapes", []string{"boxWords", "ringWords", "arrowAtWords"}},
		{"Comments", []string{"noteOn", "marginComment"}},
		// ⛔ The two verbs that change what the document SAYS, together and
		// apart from the marks and the comments: one takes words out, the other
		// puts different words in. A reader deciding between them is deciding
		// the same question, so they sit in one group.
		{"Change it", []string{"redactText", "replaceWords"}},
	}},
	{groupHandOver, []verbGroup{
		{"This page", []string{"pagePicture", "textFile", "jsonFile"}},
		{"Every page", []string{"zipped", "cbz"}},
		{"The grids", []string{"csvFile"}},
	}},
}

type verbGroup struct {
	label string
	ids   []string
}

// quickIDs are the verbs on the quick access bar: the two that are wanted
// whatever tab is open.
//
// ⛔ AND THEY ARE ON NO TAB. A ribbon puts each command on one tab, so Open
// and Save on the Home tab meant leaving whatever tab was being worked in to
// save — the browser check had to walk back to Home for it, which is the
// clearest possible statement that a reader would have to as well. They live
// on the bar instead, which the toolkit keeps across every tab and across a
// collapsed ribbon; the reference allows exactly this, a command that is on
// the bar and "not available elsewhere in the ribbon UI".
var quickIDs = []string{"open", "save"}

// verbs is every command, by id. The ids are what ribbonTabs names.
var verbs = map[string]verb{
	// The document itself.
	"open": {
		label: "Open", tip: "Choose a PDF. It is read in this tab and never uploaded.",
		icon: "folder", large: true,
		run: (*state).open, when: func(*state) bool { return true },
	},
	"save": {
		label: "Save", tip: "Hand the document back as it now stands.",
		icon: "download",
		run:  (*state).save,
	},
	"previous": {
		label: "Previous", tip: "The page before this one.",
		icon: "nav-arrow-left",
		run:  func(s *state) { s.step(-1) },
		when: func(s *state) bool { return s.doc != nil && s.at > 1 },
	},
	"next": {
		label: "Next", tip: "The page after this one.",
		icon: "nav-arrow-right",
		run:  func(s *state) { s.step(1) },
		when: func(s *state) bool { return s.doc != nil && s.at < s.doc.PageCount() },
	},
	"rotate": {
		label: "Rotate", tip: "Turn the page on the screen a quarter clockwise.",
		icon: "crop-rotate-tr", large: true,
		run: (*state).rotate,
	},
	"delete": {
		label: "Delete", tip: "Drop the page on the screen.",
		icon: "trash",
		run:  (*state).deletePage,
		// A document of one page is not one a page can be dropped from, which
		// the verb refuses and the control can say first.
		when: func(s *state) bool { return s.doc != nil && s.doc.PageCount() > 1 },
	},
	"fill": {
		label: "Fill in", tip: "Show the form this document asks to have filled in.",
		icon: "input-field", large: true,
		run:  (*state).showForm,
		when: func(s *state) bool { return s.form != nil },
	},

	// Pages.
	"keep": {
		label: "Keep only these", tip: "Drop every page the range does not name.",
		icon: "check", large: true,
		run: (*state).selectPages,
	},
	"drop": {
		label: "Delete these", tip: "Drop the pages the range names.",
		icon: "trash",
		run:  (*state).deleteRange,
	},
	"turn": {
		label: "Turn them", tip: "Turn the pages the range names by the quarter chosen beside it.",
		icon: "crop-rotate-tr", large: true,
		run: (*state).turnRange,
	},
	"reverse": {
		label: "Reverse the order", tip: "Put the pages in the opposite order.",
		icon: "sort",
		run:  (*state).reverse,
	},
	"move": {
		label: "Move it there", tip: "Take the page on the screen to the place chosen beside it, and follow it.",
		icon: "drag", large: true,
		run: (*state).movePage,
	},
	"crop": {
		label: "Crop them", tip: "Cut the pages the range names down to the box beside it.",
		icon: "crop", large: true,
		run: (*state).crop,
	},
	"insert": {
		label: "Insert a blank page", tip: "Put an empty page of the same size before the one chosen beside it.",
		icon: "page-plus",
		run:  (*state).insertBlank,
	},
	"dropBlank": {
		label: "Drop the blank pages", tip: "Drop every page with nothing drawn on it.",
		icon: "trash",
		run:  (*state).dropBlank,
	},
	"split": {
		label: "Split and hand them over", tip: "Hand the pieces over in one zip, or as a PDF when there is only one piece. What is open is not changed.",
		icon: "scissor", large: true,
		run: (*state).split,
	},

	// Sheet.
	"nup": {
		label: "Lay them out", tip: "Put as many pages on one sheet as is chosen beside it.",
		icon: "view-grid", large: true,
		run: (*state).nUp,
	},
	"booklet": {
		label: "Fold it into a booklet", tip: "Order the pages so the printed sheets fold into a booklet.",
		icon: "book",
		run:  (*state).booklet,
	},
	"interleave": {
		label: "Mix another file in", tip: "Choose another PDF and take one page from each in turn — which is how two single-sided scans of the same document become one.",
		icon: "vertical-split",
		run:  (*state).interleave,
	},
	// ⛔ A DOCUMENT CAN CARRY OTHER FILES and the workbench never said so, so
	// a PDF arriving with a spreadsheet inside it looked like a PDF with
	// nothing inside it.
	"attach": {
		label: "Put a file inside it", tip: "Choose a file and carry it inside this document. A name already used is refused rather than replacing what is there.",
		icon: "attachment", large: true,
		run: (*state).attach,
	},
	"showCarried": {
		label: "Show the files inside", tip: "Put the files travelling inside this document where the page was, with a way of taking each one out or dropping it. Press it again for the page.",
		icon: "multiple-pages",
		run:  (*state).readCarried,
		when: func(s *state) bool { return s.doc != nil },
	},
	"divide": {
		label: "Cut each page up", tip: "Cut every page into the grid of smaller pages chosen beside it, each holding its own part of the original.",
		icon: "split-area", large: true,
		run: (*state).divide,
	},
	"onepage": {
		label: "Put them on one page", tip: "Join every page into a single tall page.",
		icon: "vertical-merge",
		run:  (*state).onePage,
	},
	// ⛔ A SEPARATE GROUP FROM `divide`, AND SEPARATE NUMBERS, because the two
	// take the same "across and down" and mean opposite things: one cuts a page
	// into smaller pages, the other spreads one page over several sheets to be
	// printed larger and taped together.
	"poster": {
		label: "Print it larger than the paper", tip: "Spread each page over the grid of sheets chosen beside it, to print it larger and join it up afterwards.",
		icon: "scale-frame-enlarge", large: true,
		run: (*state).poster,
	},
	"deskew": {
		label: "Straighten the scan", tip: "Measure how far each page is tilted and turn the ones that are crooked. It says how many it measured and how many it straightened.",
		icon: "crop-rotate-tl",
		run:  (*state).deskew,
	},
	// ⛔ A DIFFERENT THING FROM `crop`, and they sit in the same group because
	// a reader will reach for the wrong one: cropping cuts the page down to
	// part of itself, this sets the sheet it is on. A page cropped to a
	// postcard on A4 paper is still on A4.
	"resize": {
		label: "Set the paper", tip: "Put the pages the range names on the sheet size chosen beside it. It changes the paper, not what is drawn on it.",
		icon: "ruler-combine",
		run:  (*state).setPaper,
	},
	"recolour": {
		label: "Paint the text", tip: "Paint the text of the pages the range names in the colour chosen beside it, leaving everything else alone. It says how much it could not reach, because text inside a form is shared with other pages.",
		icon: "fill-color", large: true,
		run: (*state).recolourText,
	},
	// ⛔⛔ EVERY VERB IN THIS GROUP REPLACES THE PAGES WITH PICTURES OF THEM,
	// and every tip says so. Turning light into dark is a pixel operation and
	// there is no other way to do it — but a reader who inverts a contract to
	// read it at night and then saves has thrown away everything that made it
	// a document, so the warning belongs on the control and not only in the
	// release notes.
	"invert": {
		label: "Turn them inside out", tip: "Draw every page with light and dark swapped, for reading at night. ⚠ The pages become pictures: their text can no longer be selected, searched or read out.",
		icon: "half-moon", large: true,
		run: (*state).invert,
	},
	"greyscale": {
		label: "Drop the colour", tip: "Draw every page in grey. ⚠ The pages become pictures: their text can no longer be selected, searched or read out.",
		icon: "droplet-half",
		run:  (*state).greyscale,
	},
	"onGround": {
		label: "Draw them on a colour", tip: "Draw every page on the colour chosen beside it — cream for reading, grey for proofing, black for the night. ⚠ The pages become pictures.",
		icon: "color-filter",
		run:  (*state).onGround,
	},
	"adjust": {
		label: "Brighten or deepen them", tip: "Draw every page at the brightness and contrast chosen beside it. ⚠ The pages become pictures.",
		icon: "sun-light",
		run:  (*state).adjust,
	},
	"scanned": {
		label: "Make them look scanned", tip: "Draw every page as a flatbed would: a little off square, a little faded, with grain. The same file always comes out the same way. ⚠ The pages become pictures.",
		icon: "printing-page",
		run:  (*state).scanned,
	},
	"merge": {
		label: "Add a file after this one", tip: "Choose another PDF and put its pages after these.",
		icon: "page-plus", large: true,
		run: (*state).merge,
	},
	"overlay": {
		label: "Lay a file over this one", tip: "Choose another PDF and draw its pages on top of these.",
		icon: "copy",
		run:  (*state).overlay,
	},

	// Marks.
	"watermark": {
		label: "Write it across them", tip: "Write the watermark diagonally across the pages the range names.",
		icon: "droplet", large: true,
		run: (*state).watermark,
	},
	"number": {
		label: "Number them", tip: "Write the page number at the foot of the pages the range names.",
		icon: "numbered-list-left", large: true,
		run: (*state).number,
	},
	"bates": {
		label: "Stamp the numbers on", tip: "Write a Bates number on each page, counting up from the first.",
		icon: "hashtag",
		run:  (*state).bates,
	},
	"stamp": {
		label: "Stamp them", tip: "Write the stamp where it is placed beside it.",
		icon: "label", large: true,
		run: (*state).stamp,
	},
	// ⛔ One press for both. `Stamp them` above could already write either — the
	// nine places include the top and the foot, and the tokens are filled in
	// wherever a stamp goes — so this verb buys the GESTURE: a header and a
	// footer are asked for together, and the second press is the one people
	// forget.
	"headFoot": {
		label: "Head and foot them", tip: "Write the header at the top of the pages and the footer at their foot, in one go. {page} and {pages} are filled in; a box left empty writes nothing there.",
		icon: "text-box",
		run:  (*state).headFoot,
	},
	"placePicture": {
		label: "Draw a picture on them", tip: "Choose a picture and draw it where it is placed beside it, at the width beside it — a signature, a logo, a seal.",
		icon: "media-image", large: true,
		run: (*state).placePicture,
	},

	// File.
	"sanitize": {
		label: "Sanitize — drop what runs", tip: "Strip whatever in the file runs rather than shows.",
		icon: "shield-check", large: true,
		run: (*state).sanitize,
	},
	"flatten": {
		label: "Flatten the annotations in", tip: "Draw the annotations into the pages, so they are part of them.",
		icon: "combine",
		run:  (*state).flatten,
	},
	"dropAnnots": {
		label: "Drop every annotation", tip: "Remove every note, highlight and form field.",
		icon: "trash",
		run:  (*state).dropAnnots,
	},
	// ⛔ THE GROUP LEADS WITH THE ONE THAT WRITES, not with the one that drops.
	// `Drop the bookmarks` was the only thing this workbench could do to a table
	// of contents for twelve versions: the rail showed one and the ribbon could
	// delete it. A group whose large control is the destructive one says that is
	// the main thing to do here, and it is not.
	"bookmarkPage": {
		label: "Bookmark this page", tip: "Add a bookmark for the page on the screen, keeping the ones the document already has. Select a passage first and it is titled with those words and points at the page they are on.",
		icon: "bookmark-circle", large: true,
		run: (*state).bookmarkPage,
	},
	// ⛔ THE PRINTED KIND, beside the kind a reader navigates by, because they
	// answer the same question and a reader deciding between them is deciding
	// one thing. The rail's bookmarks are what this is built from, so a document
	// with none is told to make some rather than handed a blank page.
	"contentsPage": {
		label: "Write a contents page", tip: "Write the bookmarks out as a printed table of contents: a page at the front with a line for each entry, its page number at the right and dots leading to it. The numbers account for the page it adds.",
		icon: "list", large: true,
		run: (*state).contentsPage,
	},
	"bookmarksFromFile": {
		label: "Bookmarks from a file", tip: "Replace the table of contents with one written as text: a title, a tab and a page number a line, two spaces of indent for each level under another — the file `pdfops outline -from` takes.",
		icon: "numbered-list-left",
		run:  (*state).bookmarksFromFile,
		when: func(s *state) bool { return s.doc != nil },
	},
	"dropOutlines": {
		label: "Drop the bookmarks", tip: "Remove the table of contents a reader navigates by.",
		icon: "bookmark",
		run:  (*state).dropOutlines,
	},
	"compress": {
		label: "Pack it smaller", tip: "Rewrite the file's streams so it takes less room.",
		icon: "archive", large: true,
		run: (*state).compress,
	},
	// ⛔ THE GROUP LEADS WITH ITS LARGE CONTROL, which is why this sits after
	// `setInfo` and not before it, however much the reading deserves to come
	// first: for a long time only the writing was here, so a reader could give
	// the document a title and not see the title it had, and anybody typing
	// into those boxes was overwriting a value the workbench never showed them.
	// The ribbon's own rule won the argument — a group whose first control is
	// small reads as a wall of equal buttons, and a test says so.
	"showInfo": {
		label: "Show what it says", tip: "Put what the file says about itself where the page was — its title, author, producer and dates, how big each page is, and whether its structure had to be rebuilt to read it. Press it again for the page.",
		icon: "page-search",
		run:  (*state).readAbout,
		when: func(s *state) bool { return s.doc != nil },
	},
	"setInfo": {
		label: "Say so in the file", tip: "Write what is typed beside it into the document: its title, author, subject, keywords, creator and producer. A box left empty removes that entry.",
		icon: "edit-pencil", large: true,
		run: (*state).setInfo,
	},
	// ⛔ ON THE RIBBON RATHER THAN ON Ctrl+C, because the workbench is handed a
	// key and nothing else: handleKeyDown takes a string, and no modifier ever
	// reaches it. A verb that quietly needed a chord the app cannot see would
	// be a verb nobody could press. The chord is worth adding; it is a change
	// to the host, not to this table.
	// ⛔⛔ A MATCH BECOMES A SELECTION, which is why these two sit next to `Copy
	// the words` and before the four marks. Finding a phrase lights it exactly
	// as a drag would have, so Copy, Highlight, Underline, Strike out, Squiggle
	// and Redact all act on what was found without knowing a search happened.
	"find": {
		label: "Find", tip: "Look for the word or phrase typed beside it, turn to the first page carrying it, and light it as though you had dragged across it.",
		icon: "text-magnifying-glass", large: true,
		run: (*state).find,
	},
	"findNext": {
		label: "Find the next one", tip: "Show the match after this one, and start again at the first after the last.",
		icon: "skip-next",
		run:  (*state).findNext,
		when: func(s *state) bool { return len(s.hits) > 1 },
	},
	// ⛔⛔ THE ONLY VERB THAT CHANGES WHAT THE DOCUMENT SAYS. Every other mark
	// on this tab is a layer that comes off or a removal that announces itself;
	// this puts words in the author's mouth, so its tip says what it is and its
	// status line says both the old words and the new ones.
	"replaceWords": {
		label: "Replace the words", tip: "Put the text typed beside it in place of the words selected on the page. ⚠ This changes what the document says, and the page keeps its layout: whatever follows on the line stays where it was.",
		icon: "edit-pencil",
		run:  (*state).replaceWords,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"copyText": {
		label: "Copy the words", tip: "Put the text selected on the page on the clipboard. Drag across a page to select some.",
		icon: "copy", large: true,
		run:  (*state).copySelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	// ⛔ THESE THREE WRITE ANNOTATIONS, NOT INK. The words under a mark stay
	// selectable, searchable and extractable, and the mark can be deleted
	// again, because that is what ISO 32000-1 §12.5.6.10 says a text-markup
	// annotation is. Burning the colour into the page's content stream would
	// look the same and be a different, irreversible act.
	"highlight": {
		label: "Highlight", tip: "Write a yellow highlight over the words selected on the page. Drag across a page to select some.",
		icon: "fill-color", large: true,
		run:  (*state).highlightSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"underline": {
		label: "Underline", tip: "Underline the words selected on the page.",
		icon: "underline",
		run:  (*state).underlineSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"strikeOut": {
		label: "Strike out", tip: "Strike through the words selected on the page. The text itself is untouched.",
		icon: "strikethrough",
		run:  (*state).strikeSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	// ⛔ THESE THREE DRAW RATHER THAN WRITE, and each leaves the words legible:
	// a box and a ring go ROUND the passage with paper between, and an arrow
	// comes in from the margin and stops short of the glyphs. A line drawn
	// through a passage would be a strike-out, and there is already a verb for
	// that two controls along.
	"boxWords": {
		label: "Box them", tip: "Draw a red rectangle round the words selected on the page, with a little paper between the border and the glyphs.",
		icon: "square-dashed", large: true,
		run:  (*state).boxSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"ringWords": {
		label: "Ring them", tip: "Draw a red ellipse round the words selected on the page, the way somebody circles a word on paper.",
		icon: "circle",
		run:  (*state).ringSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"arrowAtWords": {
		label: "Point at them", tip: "Draw a red arrow from the margin to the words selected on the page, stopping just short of them.",
		icon: "arrow-right-circle",
		run:  (*state).arrowToSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	// ⛔ THESE TWO WRITE SOMETHING OF THEIR OWN, which is what separates them
	// from the four marks above: a mark says "look here" about words the author
	// wrote, a note and a margin comment say something nobody wrote yet. Both
	// still leave the page itself untouched.
	"noteOn": {
		label: "Put a note on it", tip: "Attach a sticky note to the words selected on the page, saying what is typed beside it. The note sits in the margin, so it does not cover them.",
		icon: "chat-bubble", large: true,
		run:  (*state).noteOnSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"marginComment": {
		label: "Write in the margin", tip: "Write what is typed beside it in the margin next to the words selected on the page, where a reader would pencil a comment.",
		icon: "notes",
		run:  (*state).commentInMargin,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"squiggle": {
		label: "Squiggle under", tip: "Draw a wavy line under the words selected on the page — the mark that means look at this again. The text itself is untouched.",
		icon: "sine-wave",
		run:  (*state).squiggleSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	// ⛔⛔ THE ONE VERB ON THIS TAB THAT DESTROYS SOMETHING, and its tip says so
	// in the words that matter: the text is REMOVED, not covered. A reader who
	// thinks a redaction is a black rectangle is the person this feature is for,
	// and the label is where they find out otherwise.
	"redactText": {
		label: "Redact", tip: "Remove the words selected on the page from the file itself — not a black box over them, which anybody could copy the text out from under. It refuses, and says why, where it cannot remove them completely.",
		icon: "erase", large: true,
		run:  (*state).redactSelection,
		when: func(s *state) bool { return !s.sel.IsEmpty() },
	},
	"clearInfo": {
		label: "Drop what it says about itself", tip: "Remove the title, author, producer and dates.",
		icon: "erase",
		run:  (*state).clearInfo,
	},

	// Protect.
	"encrypt": {
		label: "Protect it", tip: "Put the passwords typed beside it on the file, allowing what is ticked.",
		icon: "lock", large: true,
		run: (*state).encrypt,
	},
	"decrypt": {
		label: "Take the protection off", tip: "Write the document out with no password on it.",
		icon: "lock-slash",
		run:  (*state).decrypt,
	},
	"protection": {
		label: "What is it protected with?", tip: "Say which passwords the file carries and what it allows.",
		icon: "info-circle", large: true,
		run: (*state).protection,
	},

	// Read.
	"readText": {
		label: "What this page says", tip: "Show the page's text where the picture of it was.",
		icon: "text-magnifying-glass", large: true,
		run: func(s *state) { s.read(readingText) },
	},
	"readImages": {
		label: "What this page carries", tip: "List the pictures drawn on the page.",
		icon: "media-image",
		run:  func(s *state) { s.read(readingImages) },
	},
	"readPage": {
		label: "Show the page again", tip: "Put the picture of the page back.",
		icon: "page",
		run:  func(s *state) { s.read("") },
	},
	"pagePicture": {
		label: "Hand over this page", tip: "Draw the page and hand it over in the format chosen beside it.",
		icon: "download", large: true,
		run: (*state).pageAsPicture,
	},
	"zipped": {
		label: "Hand over every page, zipped", tip: "Draw every page and hand them over in one zip.",
		// Large because it now LEADS a group: every group on the strip leads
		// with exactly one large control, and `Every page` is the group of the
		// two verbs that hand over all of them at once.
		icon: "folder", large: true,
		run: (*state).everyPageZipped,
	},
	// ⛔ A REAL .cbz, which the zip beside it is not. Both hold one picture per
	// page; a comic reader opens the one named for the format. The same bytes
	// under the right name is the whole difference between a file somebody can
	// use and one they have to rename first.
	"cbz": {
		label: "Hand it over as a comic", tip: "Draw every page and hand them over as a .cbz — the archive a comic reader opens as a book.",
		icon: "book",
		run:  (*state).asArchive,
	},
	"textFile": {
		label: "Hand over what this page says", tip: "Hand the page's text over as a file.",
		icon: "page",
		run:  (*state).textAsFile,
	},
	// ⛔ "the grids its words line up into", not "its tables". A PDF page holds
	// no tables: it holds text put at places, and a table is what a reader sees
	// when those places line up. The wording says inference because that is
	// what it is, and a verb promising tables would be promising something the
	// file cannot declare.
	"readTables": {
		label: "What lines up into a grid", tip: "Put the grids this page's words line up into where the page was. A PDF holds no tables — it holds text at places — so these are read off the alignment. Press it again for the page.",
		icon: "table-2-columns",
		run:  (*state).readTables,
		when: func(s *state) bool { return s.doc != nil },
	},
	"csvFile": {
		// ⛔ The tip SAID "with a blank line between them", which is what this
		// verb deliberately does not do: a CSV reader skips blank lines, so a
		// file separated that way comes back as one grid with the next one's
		// rows stuck to it (go-pdfkit/ops#71). The verb writes a one-cell
		// marker row naming each grid, and the tip now says that — a tip that
		// promises the behaviour we rejected is the defect described to the
		// person instead of done to them.
		label: "Hand over the grids", tip: "Hand this page's grids over as one comma-separated file, with a row naming each grid in front of it.",
		icon: "table-rows", large: true,
		run: (*state).tablesAsCSV,
	},
	"jsonFile": {
		label: "Hand over the page as data", tip: "Hand this page over as JSON: every piece of text with where it sits, and the grids its words line up into.",
		icon: "code-brackets-square",
		run:  (*state).pageAsJSON,
	},
}

// buildRibbon makes the ribbon and the actions behind it, once.
//
// ⛔ ONE ACTION PER VERB, AND THE ACTION IS WHAT SAYS IT MAY RUN. Action is
// the toolkit's single source of truth for a command, so a verb that cannot run
// greys wherever it is shown. reconsider sets them all after every change,
// which is what makes the arrows grey at the ends of a document and Delete grey
// on a document of one page -- told beforehand rather than refused afterwards.
func (s *state) buildRibbon() *toolkit.Ribbon {
	s.acts, s.rules = map[string]*toolkit.Action{}, map[string]func(*state) bool{}
	var tabs []*toolkit.RibbonTab
	for _, t := range ribbonTabs {
		tab := &toolkit.RibbonTab{Label: t.label}
		for gi, g := range t.groups {
			rg := &toolkit.RibbonGroup{
				Label: g.label,
				// The leftmost groups are the ones reached for most, so they are
				// the last to shrink: Priority rises to the right.
				Priority: gi,
			}
			for _, id := range g.ids {
				v := verbs[id]
				a := s.action(id, v)
				rg.Controls = append(rg.Controls, &toolkit.RibbonControl{
					Kind:   toolkit.RibbonButton,
					Action: a,
					Tip:    v.tip,
					Large:  v.large,
				})
			}
			tab.Groups = append(tab.Groups, rg)
		}
		tabs = append(tabs, tab)
	}
	r := toolkit.NewRibbon(tabs...)
	for _, id := range quickIDs {
		v := verbs[id]
		r.Quick = append(r.Quick, &toolkit.RibbonControl{
			Kind:   toolkit.RibbonButton,
			Action: s.action(id, v),
			Tip:    v.tip,
		})
	}
	r.Quick = append(r.Quick, s.contentsControl(), s.themeControl())
	return r
}

// contentsControl switches the rail between the minipages and the document's
// own table of contents.
//
// ⛔ Not in the verbs table, for the same reason the theme switch is not: it
// changes what the workbench SHOWS and not what the document says. It is
// greyed when no document is open, though, which the theme switch is not —
// there is no rail to switch without one.
func (s *state) contentsControl() *toolkit.RibbonControl {
	a := &toolkit.Action{
		ID: "contents",
		// One label, as with the theme switch: Ribbon.ControlRect finds a
		// control by it, so a label that moved with the state would move the
		// control out from under the press that moved it.
		Label: "Contents",
		Icon: func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
			toolkit.DrawIconoir(p, r, s.contentsIcon(), ink)
		},
		Run:     s.toggleContents,
		Enabled: mvvm.NewObservable(false),
		Visible: mvvm.NewObservable(true),
	}
	s.acts["contents"] = a
	// ⛔ Offered for ANY open document, not only one carrying an outline. The
	// first version greyed it for a document with no /Outlines, which is most
	// of them — so the view shipped and was reported missing, by somebody
	// running the build that added it. A document with no outline gets a list
	// of its pages, and the tooltip says so before the press rather than after.
	s.rules["contents"] = func(s *state) bool { return s.opened() }
	s.contentsCtl = &toolkit.RibbonControl{Kind: toolkit.RibbonButton, Action: a, Tip: s.contentsTip()}
	return s.contentsCtl
}

// themeControl is the light / dark / system switch.
//
// ⛔ IT IS NOT IN THE VERBS TABLE, and that is deliberate rather than an
// oversight. Everything in that table is something done TO THE DOCUMENT: it
// has a rule about whether a document is open, it marks the file dirty, and
// the test that presses all of them expects a document to press them against.
// A colour scheme is none of those — it is as available with nothing open as
// with a hundred pages, and pressing it changes no byte that would be saved.
//
// It sits on the quick bar for the same reason Open and Save do: it is wanted
// whatever tab is being worked in.
func (s *state) themeControl() *toolkit.RibbonControl {
	a := &toolkit.Action{
		ID: "theme",
		// ⛔ The label does NOT cycle with the mode, though the mode is what the
		// button is for. Ribbon.ControlRect finds a control BY ITS LABEL, so a
		// label that changed with the state would move the control out from
		// under every test and every browser check that reaches for it — and it
		// would be the press itself that moved it. The mark carries the state
		// instead, and the tooltip says it in words.
		Label: "Theme",
		Icon: func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
			toolkit.DrawIconoir(p, r, s.themeMode.icon(), ink)
		},
		Run:     s.cycleTheme,
		Enabled: mvvm.NewObservable(true),
		Visible: mvvm.NewObservable(true),
	}
	s.acts["theme"] = a
	// No rule: reconsider greys a verb when no document is open, and this is
	// not a verb. Registering nil would make it grey with the rest.
	s.rules["theme"] = func(*state) bool { return true }
	s.themeCtl = &toolkit.RibbonControl{Kind: toolkit.RibbonButton, Action: a, Tip: s.themeMode.tip()}
	return s.themeCtl
}

// action builds one verb's Action, bound to this workbench.
//
// ⛔ The rule is KEPT BESIDE THE ACTION rather than looked up in the verbs
// table when it is needed. Go would refuse the package otherwise: reconsider
// would read verbs, and the verbs run handlers that end in refresh, which calls
// reconsider -- an initialisation cycle through a table that is only ever read.
// Carrying the rule here breaks it, and reads better as well: an Action and the
// question of whether it may run are one thing.
func (s *state) action(id string, v verb) *toolkit.Action {
	run := v.run
	a := &toolkit.Action{
		ID:      id,
		Label:   v.label,
		Icon:    iconoirGlyph(v.icon),
		Run:     func() { run(s) },
		Enabled: mvvm.NewObservable(s.mayRun(v.when)),
		Visible: mvvm.NewObservable(true),
	}
	s.acts[id] = a
	s.rules[id] = v.when
	return a
}

// mayRun is a verb's rule: its own, or "a document is open" when it has none.
func (s *state) mayRun(when func(*state) bool) bool {
	if when != nil {
		return when(s)
	}
	return s.opened()
}

// iconoirGlyph draws one icon from the pack at whatever box it is given.
//
// ⛔ Through the toolkit, from a pack, never drawn here. toolkit.DrawIconoir
// rasterises one of the SVGs that go-icons/iconoir embeds, recoloured to the
// ink the control is using. A glyph outlined by hand would be a per-app
// duplicate of a layer that exists to be shared.
func iconoirGlyph(name string) toolkit.IconFunc {
	return func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
		toolkit.DrawIconoir(p, r, name, ink)
	}
}

// tabGroup is the panel a tab opens: none for Home, else the group of the same
// name. The tabs after the first are groupNames in order, which
// TestTheTabsAfterHomeAreTheGroups holds them to.
func tabGroup(i int) string {
	if i <= 0 || i > len(groupNames) {
		return ""
	}
	return groupNames[i-1]
}

// followTab opens the panel the chosen tab commands.
//
// A tab and its panel are the same subject: the verbs of a group, and what
// those verbs are told. Choosing "Marks" therefore opens the Marks panel beside
// the page, and going back to Home puts it away -- there is nothing left to
// switch a panel on with, and nothing that could leave the two disagreeing.
func (s *state) followTab(i int) {
	s.tools.open = tabGroup(i)
	name := s.tools.open
	if name != "" {
		s.showingForm = false
	}
	s.settle()
	s.note = ""
	s.refresh()
}
