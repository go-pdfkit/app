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

import "encoding/json"

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
	out = append(out, spot{Label: bandPage,
		X: margin, Y: s.viewTop(), W: s.pageW(), H: s.viewH(), On: s.page != nil})
	if s.tools.open != "" {
		out = append(out, spot{Label: bandPanel,
			X: s.w - margin - panelW, Y: s.viewTop(), W: panelW, H: s.viewH(), On: true})
	}
	return out
}

// The two bands, named rather than numbered. A panel is only published when
// one is open, which is itself the answer to "did choosing that tab open its
// settings".
const (
	bandPage  = "#page"
	bandPanel = "#panel"
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

// marshal is a variable so a test can watch what is handed back when writing
// the answer goes wrong, the way docBytes and drawPage are.
var marshal = json.Marshal
