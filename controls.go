// Where the named controls are, for anything driving the workbench from
// outside it.
//
// ⛔ A CANVAS HAS NO DOM. Everything here is painted into one framebuffer, so
// a harness, a script or an assistive bridge has nothing to find by name: it
// can only press at coordinates it guessed. The browser check in this repo did
// exactly that -- it swept the old strip from x=0 to x=1000 at a fixed y,
// decided that whichever press opened a file picker was Open, and then swept
// the panel downwards pressing everything until the document changed. That
// read as a test of the workbench and was mostly a test of the sweep: it could
// not say WHICH verb it had run, and a ribbon, whose controls sit on several
// rows and several tabs, puts it out of reach entirely.
//
// So the workbench says where its controls are. It is the same answer
// Ribbon.ControlRect gives, published for a caller that is not in this
// process.

package main

import (
	"encoding/json"

	"github.com/go-widgets/toolkit"
)

// A spot is one named control and where it is on the canvas, in the canvas's
// own pixels.
type spot struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	// On says the control can be used. A harness that presses a greyed verb
	// and waits for the document to change would wait for ever, and report it
	// as the verb being broken.
	On bool `json:"on"`
}

// controls is every tab and every control the ribbon is showing, by the name
// it shows. The tab strip is included because choosing a tab is how a group of
// verbs — and the panel of settings those verbs read — is reached at all.
func (s *state) controls() []spot {
	var out []spot
	for i, t := range ribbonTabs {
		// The index comes from the table the ribbon's tabs were built from, so
		// there is always one there; TestTheTabsAfterHomeAreTheGroups is what
		// holds the two to each other.
		r, _ := s.ribbon.TabRect(i)
		out = append(out, spot{Label: t.label, X: r.X, Y: r.Y, W: r.W, H: r.H, On: true})
	}
	for id, a := range s.acts {
		r, ok := s.ribbon.ControlRect(a.Label)
		if !ok {
			// Not on the tab being shown — or on no tab at all, which is where
			// the quick access bar's own commands live.
			if r, ok = s.ribbon.QuickRect(a.Label); !ok {
				continue
			}
		}
		out = append(out, spot{Label: id, X: r.X, Y: r.Y, W: r.W, H: r.H, On: a.IsEnabled()})
	}
	// ⛔ And the two bands a caller reads PIXELS out of, which it cannot work
	// out for itself any more. The browser check used to count the ink in a
	// band written as fractions of the canvas -- "from 7% down to 95%, left of
	// 66%" -- numbers that were right for a strip of buttons and wrong the
	// moment the band above the page grew. A check whose baseline pixel is
	// inside the ribbon measures the ribbon.
	// On says the band is showing a drawn page. It reads the VIEWER rather
	// than a picture field: the band now holds a viewer at all times, and what
	// a caller is asking is whether there is a page in it.
	out = append(out, spot{Label: bandPage,
		X: margin, Y: s.viewTop(), W: s.pageW(), H: s.viewH(), On: s.pv.PageCount() > 0})
	if s.tools.open != "" {
		out = append(out, spot{Label: bandPanel,
			X: s.w - margin - panelW, Y: s.viewTop(), W: panelW, H: s.viewH(), On: true})
	}
	// ⛔ AND THE DIVIDER, because everything else here is PRESSED and the
	// divider is the one control that only answers to a gesture: press, move,
	// release. The browser check could not perform one, so it was green before
	// and after a divider that moved nothing at all -- a defect reported from
	// outside as "the minipage resize bar does not work", which no check in this
	// repository saw. A press needs one point; a drag needs a point to start
	// from and something whose width is supposed to change, so both are
	// published: the handle to grab, and the band whose width answers.
	//
	// ⛔ Read out of the SPLITTER ON THE SCREEN, not out of railView: moving the
	// divider throws the minipages away so they can be drawn again at the new
	// width, so railView is nil for the whole of the resize. A caller asking
	// "did the drag work" would have been told the rail had gone.
	//
	// Pane 0 and handle 0 are the rail's whenever there is a rail pane, because
	// arrange puts it first; railPane is that arrangement, recorded rather than
	// assumed, so a harness told to widen the minipages cannot end up widening
	// the settings.
	if sp := splitterIn(s.view); sp != nil && s.railPane {
		b := sp.Bounds()
		out = append(out, spot{Label: bandRail,
			X: b.X, Y: b.Y, W: sp.PaneSize(0), H: b.H, On: s.railView != nil})
		if h, ok := sp.HandleRect(0); ok && h.W > 0 && h.H > 0 {
			out = append(out, spot{Label: ctlDivider, X: h.X, Y: h.Y, W: h.W, H: h.H, On: true})
		}
	}
	return out
}

// splitterIn finds the splitter a band is laid out with, which is what a reader
// drags. ⛔ Found by WALKING the band rather than kept in a field: a field would
// agree with a workbench that built a splitter and never put it on the screen,
// which is the half of this feature that shipped broken once already.
func splitterIn(w toolkit.Widget) *toolkit.Splitter {
	if sp, ok := w.(*toolkit.Splitter); ok {
		return sp
	}
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, c := range p.Children() {
			if sp := splitterIn(c); sp != nil {
				return sp
			}
		}
	}
	return nil
}

// The two bands, named rather than numbered. A panel is only published when
// one is open, which is itself the answer to "did choosing that tab open its
// settings".
const (
	bandPage  = "#page"
	bandPanel = "#panel"
	// bandRail is the minipages, whose WIDTH is the answer to whether the
	// divider moved. ctlDivider is what is grabbed to move it.
	bandRail   = "#rail"
	ctlDivider = "#divider"
)

// controlsJSON is the same, as the text a caller outside this process reads.
//
// ⛔ Laid out first. ControlRect answers out of the last layout, and a caller
// that asks before the first frame — which is exactly when a harness asks —
// would otherwise be told every control is at the origin.
func (s *state) controlsJSON() string {
	s.layOutRibbon()
	b, err := marshal(s.controls())
	if err != nil {
		// Marshalling a slice of strings and ints cannot fail. Saying so beats
		// handing back something that parses as "no controls at all", which a
		// caller would read as a workbench that has none.
		return `["cannot be said: ` + err.Error() + `"]`
	}
	return string(b)
}

// A contentsRowSaid is one row of the contents pane as a caller outside this
// process reads it: what it says and where it leads.
//
// Page is this document's page number, 0 for a row that goes nowhere; URI is
// the address of a row that leaves the document, "" for one that does not.
type contentsRowSaid struct {
	Title string `json:"title"`
	Page  int    `json:"page"`
	URI   string `json:"uri"`
	Depth int    `json:"depth"`
}

// contentsJSON is what the contents pane is listing, as text.
//
// ⛔ WORDS, NOT COORDINATES. Everything else published here is geometry, and
// geometry is enough to press a control and watch what happens. A contents row
// is named by the DOCUMENT — a bookmark's title, or the text a link sits on —
// so a harness reading only rectangles cannot tell a pane listing the right
// rows from one listing the wrong ones. It is empty when the pane is not
// showing, which is itself the answer to "is it up".
func (s *state) contentsJSON() string {
	var rows []contentsRowSaid
	if s.railContents && s.src != nil {
		for _, m := range s.contentsOf(s.src, s.marksFor(s.src, s.thumbs.stamp)) {
			rows = append(rows, contentsRowSaid{
				Title: m.title, Page: m.page, URI: m.uri, Depth: m.depth,
			})
		}
	}
	b, err := marshal(rows)
	if err != nil {
		return `[{"title": "cannot be said: ` + err.Error() + `"}]`
	}
	return string(b)
}

// marshal is a variable so a test can watch what is handed back when writing
// the answer goes wrong, the way docBytes and drawPage are.
var marshal = json.Marshal
