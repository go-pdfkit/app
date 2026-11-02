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
		{"File", []string{"open", "save"}},
		{"Page", []string{"rotate", "previous", "next", "delete"}},
		{"Form", []string{"fill"}},
	}},
	{groupPages, []verbGroup{
		{"Which pages", []string{"keep", "drop"}},
		{"Turn", []string{"turn", "reverse"}},
		{"Order", []string{"move"}},
		{"Shape", []string{"crop", "insert", "dropBlank"}},
		{"Split", []string{"split"}},
	}},
	{groupSheet, []verbGroup{
		{"To a sheet", []string{"nup", "booklet"}},
		{"Another file", []string{"merge", "overlay"}},
	}},
	{groupMarks, []verbGroup{
		{"Watermark", []string{"watermark"}},
		{"Numbers", []string{"number", "bates"}},
		{"Stamp", []string{"stamp"}},
	}},
	{groupFile, []verbGroup{
		{"Clean", []string{"sanitize", "flatten", "dropAnnots", "dropOutlines"}},
		{"Size", []string{"compress"}},
		{"About it", []string{"setInfo", "clearInfo"}},
	}},
	{groupProtect, []verbGroup{
		{"Password", []string{"encrypt", "decrypt"}},
		{"Check", []string{"protection"}},
	}},
	{groupRead, []verbGroup{
		{"Read", []string{"readText", "readImages", "readPage"}},
		{"Hand over", []string{"pagePicture", "zipped", "textFile"}},
	}},
}

type verbGroup struct {
	label string
	ids   []string
}

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
		label: "Split and hand them over", tip: "Hand over one file per piece. What is open is not changed.",
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
	"setInfo": {
		label: "Say so in the file", tip: "Write the title and author typed beside it into the document.",
		icon: "edit-pencil", large: true,
		run: (*state).setInfo,
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
		icon: "folder",
		run:  (*state).everyPageZipped,
	},
	"textFile": {
		label: "Hand over what this page says", tip: "Hand the page's text over as a file.",
		icon: "page",
		run:  (*state).textAsFile,
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
	return toolkit.NewRibbon(tabs...)
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
