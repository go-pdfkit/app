package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pdfkit/reader"
)

// check is the protocol: boot, look, open a file the way a person does, look
// again, save, and read what came out.
func check(ctx context.Context, c *conn, page, sample, shot, dl string) error {
	r, err := c.call(ctx, "Target.createTarget", map[string]any{"url": "about:blank"}, "")
	if err != nil {
		return err
	}
	var target struct {
		TargetID string `json:"targetId"`
	}
	json.Unmarshal(r, &target)
	r, err = c.call(ctx, "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true}, "")
	if err != nil {
		return err
	}
	var att struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(r, &att)
	sid := att.SessionID

	for _, m := range []string{"Page.enable", "Runtime.enable", "DOM.enable", "Log.enable"} {
		if _, err := c.call(ctx, m, nil, sid); err != nil {
			return err
		}
	}
	if _, err := c.call(ctx, "Page.setInterceptFileChooserDialog", map[string]any{"enabled": true}, sid); err != nil {
		return err
	}
	if _, err := c.call(ctx, "Browser.setDownloadBehavior", map[string]any{
		"behavior": "allowAndName", "downloadPath": dl, "eventsEnabled": true}, ""); err != nil {
		return err
	}
	if _, err := c.call(ctx, "Page.navigate", map[string]any{"url": page}, sid); err != nil {
		return err
	}

	// The shell only shows the canvas once the program is running, so waiting
	// for that class is waiting for the wasm to have started.
	if err := until(ctx, 60*time.Second, func() (bool, error) {
		// The page may not be there yet: Page.navigate returns when the
		// navigation begins, not when it has finished, so the first look is
		// often at the blank page the browser started on. A missing element
		// is "not ready yet", not a failure.
		v, err := eval(ctx, c, sid,
			`(() => { const s = document.getElementById('stage');
			          return !!s && s.classList.contains('ready'); })()`)
		return v == "true", err
	}); err != nil {
		return fmt.Errorf("the workbench never started: %w", err)
	}
	fmt.Println("the program is running in the tab")

	whole, err := look(ctx, c, sid)
	if err != nil {
		return err
	}
	fmt.Printf("empty workbench: %d pixels drawn, %d of them dark\n", whole.N, whole.Dark)
	if whole.N == 0 {
		return fmt.Errorf("the canvas is blank with nothing open")
	}

	// ⛔ THE CONTROLS ARE ASKED FOR BY NAME. This used to press along the strip
	// from x=0 to x=1000 and call whichever press opened a file picker "Open",
	// then sweep the panel downwards pressing everything until the document
	// changed. That read as a test of the workbench and was mostly a test of
	// the sweep: it could never say WHICH verb it had run, and a ribbon, whose
	// controls sit on several rows of several tabs, puts it out of reach
	// altogether. The workbench publishes where its controls are now.
	at, err := controls(ctx, c, sid)
	if err != nil {
		return err
	}
	fmt.Printf("the workbench says where %d controls are\n", len(at))
	empty, err := lookAt(ctx, c, sid, at[bandPage])
	if err != nil {
		return err
	}
	fmt.Printf("the page band holds %d pixels of ink with nothing open\n", empty.Ink)

	drain(c)
	if err := pressControl(ctx, c, sid, at, "open"); err != nil {
		return err
	}
	ev, ok := waitEvent(c, "Page.fileChooserOpened", 5*time.Second)
	if !ok {
		say(c)
		return fmt.Errorf("pressing Open where the workbench says it is asked for no file")
	}
	var p struct {
		BackendNodeID int `json:"backendNodeId"`
	}
	json.Unmarshal(ev.Params, &p)
	abs, _ := filepath.Abs(sample)
	if _, err := c.call(ctx, "DOM.setFileInputFiles", map[string]any{
		"files": []string{abs}, "backendNodeId": p.BackendNodeID}, sid); err != nil {
		return err
	}
	fmt.Printf("the file picker opened from Open, and was handed %s\n", filepath.Base(sample))

	// ⛔ Measured IN THE PAGE BAND, which the workbench says where to find.
	// The whole canvas was measured here before, and the comparison was "more
	// pixels differ from the top-left one than did before" -- which stopped
	// being about the document the moment the band above the page changed
	// height, since the top-left pixel it compares everything to is then a
	// pixel of the ribbon.
	//
	// Ink, as opposed to nearly black: a scanned page's ink is grey once it has
	// been scaled to fit, so counting nearly black pixels read a real scanned
	// document as never having arrived, on a page that was drawn correctly.
	if err := until(ctx, 30*time.Second, func() (bool, error) {
		where, err := controls(ctx, c, sid)
		if err != nil {
			return false, err
		}
		v, err := lookAt(ctx, c, sid, where[bandPage])
		if err != nil {
			return false, err
		}
		return where[bandPage].On && v.Hash != empty.Hash && v.Ink > empty.Ink, nil
	}); err != nil {
		say(c)
		return fmt.Errorf("the document never appeared where the page is drawn: %w", err)
	}
	at, err = controls(ctx, c, sid)
	if err != nil {
		return err
	}
	withDoc, _ := lookAt(ctx, c, sid, at[bandPage])
	fmt.Printf("document shown: %d pixels drawn in the page band, %d of them ink\n", withDoc.N, withDoc.Ink)

	if err := screenshot(ctx, c, sid, shot); err != nil {
		return err
	}
	fmt.Println("what the tab looks like is in", shot)

	// Save, which hands back the document as it now stands.
	drain(c)
	at, err = controls(ctx, c, sid)
	if err != nil {
		return err
	}
	if err := pressControl(ctx, c, sid, at, "save"); err != nil {
		return err
	}
	if _, ok := waitEvent(c, "Browser.downloadWillBegin", 10*time.Second); !ok {
		say(c)
		return fmt.Errorf("pressing Save where the workbench says it is handed nothing back")
	}

	var out []byte
	if err := until(ctx, 30*time.Second, func() (bool, error) {
		files, _ := os.ReadDir(dl)
		for _, f := range files {
			b, err := os.ReadFile(filepath.Join(dl, f.Name()))
			if err == nil && len(b) > 4 && string(b[:4]) == "%PDF" {
				out = b
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("nothing that looks like a PDF was downloaded: %w", err)
	}
	fmt.Printf("the tab handed back a %d byte PDF\n", len(out))
	if err := os.WriteFile(shot+".pdf", out, 0o644); err != nil {
		return err
	}

	// Home is not the whole workbench: the other tabs carry the twenty-five
	// verbs that need telling something, and the panel beside the page carries
	// what they are told. Both have to be driven here, because a tab that is
	// drawn and gets no events looks exactly like one that works — which is
	// what the panel was until the toolkit underneath learned to hand a press
	// to what is inside a scroll view.
	//
	// What counts as a verb having run is that the file the tab hands back is a
	// DIFFERENT file. Some tabs change what is on the screen without changing
	// the document at all — Read replaces the picture of the page with what the
	// page says — so a check that stopped at the first press to redraw the
	// canvas would call that a verb.
	if err := pressControl(ctx, c, sid, at, "Marks"); err != nil {
		return err
	}
	at, err = controls(ctx, c, sid)
	if err != nil {
		return err
	}
	// ⛔ The panel is only published when one is open, so this is the
	// workbench's own answer rather than a guess from the pixels in a band
	// that may have changed for some other reason.
	if _, ok := at[bandPanel]; !ok {
		return fmt.Errorf("choosing the Marks tab opened no panel of settings beside the page")
	}
	fmt.Println("the Marks tab opened its panel of settings beside the page")

	pageWas, err := lookAt(ctx, c, sid, at[bandPage])
	if err != nil {
		return err
	}
	drain(c)
	if err := pressControl(ctx, c, sid, at, "watermark"); err != nil {
		return err
	}
	if changed, err := changedIn(ctx, c, sid, at[bandPage], pageWas.Hash); err != nil {
		return err
	} else if !changed {
		say(c)
		return fmt.Errorf("the watermark verb redrew nothing where the page is")
	}
	// ⛔ SAVED FROM THE MARKS TAB, without going back to Home first. Open and
	// Save are on the ribbon's quick access bar and on no tab at all, so they
	// are reachable from wherever the reader is. This check had to walk back to
	// Home for Save before that bar existed, which was the clearest possible
	// statement that a reader would have had to as well.
	at, err = controls(ctx, c, sid)
	if err != nil {
		return err
	}
	if _, ok := at[bandPanel]; !ok {
		return fmt.Errorf("the Marks panel went away before the document was saved")
	}
	if err := pressControl(ctx, c, sid, at, "save"); err != nil {
		return err
	}
	again, err := waitForPDF(ctx, dl, out, 20*time.Second)
	if err != nil {
		say(c)
		return fmt.Errorf("the watermark verb changed the screen and not the document: %w", err)
	}
	fmt.Printf("the tab handed back a %d byte PDF after the watermark verb ran\n", len(again))
	doc, err := reader.Open(again)
	if err != nil {
		return fmt.Errorf("what the tab saved after the panel was used does not open: %w", err)
	}
	content, err := doc.PageContent(1)
	if err != nil {
		return fmt.Errorf("the first page of it cannot be read: %w", err)
	}
	if !bytes.Contains(content, []byte("(DRAFT) Tj")) {
		return fmt.Errorf("the mark the Marks tab wrote is not in the file the tab handed back")
	}
	fmt.Println("the mark written from the ribbon is in the saved file")

	return shellReaches(ctx, c, sid, page)
}

// The two builds the staleness check publishes, in order. They are only ever
// compared for equality, so what they say matters less than that they differ.
const (
	firstBuild  = "the build they first saw"
	secondBuild = "the build published afterwards"
)

// shellReaches asks the one question the rest of this program cannot: does a
// build published AFTER somebody's first visit reach them?
//
// ⛔ It is the check the shell did not have, and its absence cost five
// releases. The service worker kept a copy of every file it had ever served
// and answered from that copy forever: a worker only reinstalls when its own
// script changes byte for byte, the cache was named by a constant so the
// activate sweep never deleted anything, and nothing revalidated. Everyone who
// had opened the workbench once kept the build they first saw. It was reported
// as the layout regressing, because that is what it looks like from the
// outside.
//
// Driving the canvas cannot find this. Every check above runs in the first
// visit, where the cache and the server agree by construction. So this one
// changes a file UNDER the browser, reloads, and asks what the page gets.
//
// ⛔ It asserts that a published build REACHES the tab, not that any
// particular thing goes wrong when it does not. Run against the cache-first
// worker this replaced, it failed three times in three different ways -- the
// reload never finished, navigator.serviceWorker was gone when asked, and the
// fetch threw "Failed to fetch" -- and a check written to recognise the first
// of those would have passed the other two. A worker that cannot ship is a
// worker that cannot ship, however it declines to.
func shellReaches(ctx context.Context, c *conn, sid, page string) error {
	// The worker has to be in charge for any of this to be about the worker.
	// On the visit that registers it, the page is usually still uncontrolled:
	// it is the NEXT load the worker answers, which is exactly the returning
	// visitor this is about.
	if err := until(ctx, 30*time.Second, func() (bool, error) {
		v, err := eval(ctx, c, sid, inCharge)
		return v == "true", err
	}); err != nil {
		return fmt.Errorf("the offline worker never took charge of the tab, so nothing here is about it: %w", err)
	}

	got, err := fetched(ctx, c, sid, "build.txt")
	if err != nil {
		return err
	}
	if got != firstBuild {
		return fmt.Errorf("the tab reads %q from a server that is serving %q", got, firstBuild)
	}
	fmt.Println("the tab is served by the offline worker, and sees the build it arrived on")

	// Publish. Nothing else changes: same URL, same worker, same tab.
	buildStamp.Store(secondBuild)

	if _, err := c.call(ctx, "Page.navigate", map[string]any{"url": page}, sid); err != nil {
		return err
	}
	// ⛔ Waiting for the WORKBENCH to start here would be waiting for twenty
	// megabytes of wasm, and the first cut did exactly that. Under the
	// cache-first worker it timed out, so the check failed saying "the
	// workbench did not come back" -- true, and not the defect. What is being
	// asked is what the page is SERVED, which needs a document and a worker in
	// charge and nothing else.
	// ⛔ Waiting on the DOCUMENT, not on the worker being in charge. Requiring
	// a controller here made the check report "the tab never came back" under
	// the broken worker -- true, and not the thing being asked. A worker that
	// has stopped answering and a worker that answers with last week's file
	// are both "the shell cannot ship", and the question below distinguishes
	// them by itself.
	if err := until(ctx, 60*time.Second, func() (bool, error) {
		v, err := eval(ctx, c, sid, `(() => { try { return document.readyState !== 'loading'; } catch (e) { return false; } })()`)
		return v == "true", err
	}); err != nil {
		return fmt.Errorf("the tab never came back after the reload: %w", err)
	}
	who, err := eval(ctx, c, sid, inCharge)
	if err != nil {
		return err
	}

	got, err = fetched(ctx, c, sid, "build.txt")
	if err != nil {
		return err
	}
	if got != secondBuild {
		return fmt.Errorf("a returning visitor is pinned to an old build: the server publishes %q "+
			"and the tab is still served %q (a worker in charge of the tab: %s), so nothing released "+
			"after somebody's first visit reaches them", secondBuild, got, who)
	}
	fmt.Println("a build published after the first visit reaches the tab on the next load")
	return nil
}

// inCharge asks whether there is a document and a worker answering for it.
//
// ⛔ TOTAL on purpose. The first cut read `navigator.serviceWorker.controller`
// straight, and right after a navigate there may be no navigator.serviceWorker
// at all -- the tab is still between documents. That throws, and a throw out
// of a poll is not "not yet", it is a failed check: the run against the broken
// worker failed on a TypeError instead of on the staleness, and the run
// against the fixed one passed on the timing rather than on the answer.
// Neither told me anything.
const inCharge = `(() => {
	try {
		return document.readyState !== 'loading' &&
			!!(navigator.serviceWorker && navigator.serviceWorker.controller);
	} catch (e) { return false; }
})()`

// fetched is what the page gets for a URL, which is what the worker decides.
//
// eval hands back the raw JSON of what the snippet evaluated to, so a string
// arrives quoted and has to be read back as one.
func fetched(ctx context.Context, c *conn, sid, url string) (string, error) {
	// The throw is caught IN THE TAB and handed back as text, because a worker
	// that will not serve the file makes fetch reject -- and a rejection
	// arriving as a page of DevTools JSON says what the exception object looks
	// like rather than what happened.
	v, err := eval(ctx, c, sid, fmt.Sprintf(`(async () => {
		try { return await (await fetch(%q, {cache: 'no-store'})).text(); }
		catch (e) { return 'nothing: the tab could not fetch it at all (' + e + ')'; }
	})()`, url))
	if err != nil {
		return "", fmt.Errorf("asking the tab for %s: %w", url, err)
	}
	var s string
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return "", fmt.Errorf("the tab answered %s for %s, which is not a string: %w", v, url, err)
	}
	return s, nil
}

// The two bands the workbench publishes alongside its controls: where the
// document is drawn, and where the settings the verbs read are. They are named
// rather than measured, because a band worked out here from fractions of the
// canvas goes stale, silently, the moment the layout above it changes.
const (
	bandPage  = "#page"
	bandPanel = "#panel"
)

// A spot is one named control and where the workbench says it is, in the
// canvas's own pixels. It is the same shape the workbench publishes.
type spot struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	On    bool   `json:"on"`
}

// controls asks the workbench where its named controls are.
//
// ⛔ ASKED EVERY TIME rather than kept. A tab holds its own verbs, so the
// answer changes the moment one is chosen; a map read once would have the
// check pressing where the last tab's controls used to be.
func controls(ctx context.Context, c *conn, sid string) (map[string]spot, error) {
	v, err := eval(ctx, c, sid, `(() => {
		if (typeof window.pdfkitControls !== 'function') return '[]';
		return window.pdfkitControls();
	})()`)
	if err != nil {
		return nil, fmt.Errorf("asking the workbench where its controls are: %w", err)
	}
	var quoted string
	if json.Unmarshal([]byte(v), &quoted) == nil {
		v = quoted
	}
	var list []spot
	if err := json.Unmarshal([]byte(v), &list); err != nil {
		return nil, fmt.Errorf("the workbench answered %s, which is not a list of controls: %w", v, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("the workbench published no controls at all")
	}
	out := map[string]spot{}
	for _, sp := range list {
		out[sp.Label] = sp
	}
	return out, nil
}

// pressControl presses the named control where the workbench says it is.
func pressControl(ctx context.Context, c *conn, sid string, at map[string]spot, name string) error {
	sp, ok := at[name]
	if !ok {
		return fmt.Errorf("the workbench is not showing a control called %q", name)
	}
	if !sp.On {
		// ⛔ Said rather than pressed. A greyed verb does nothing, and a check
		// that pressed one and then waited for the document to change would
		// wait out its whole timeout and report the verb as broken.
		return fmt.Errorf("%q is there but greyed, so pressing it would do nothing", name)
	}
	return click(ctx, c, sid, sp.X+sp.W/2, sp.Y+sp.H/2)
}

// changedIn waits a moment for a band to catch up and reports whether it
// changed.
//
// The wait is the point: the canvas is repainted on an animation frame rather
// than on the press itself, so a look taken straight after a click is a look
// at what was on the screen before it.
func changedIn(ctx context.Context, c *conn, sid string, at spot, was int) (bool, error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		v, err := lookAt(ctx, c, sid, at)
		if err != nil {
			return false, err
		}
		if v.Hash != was {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
	}
}

// waitForPDF waits for a PDF to land in the download directory that is not the
// one already seen.
func waitForPDF(ctx context.Context, dl string, notThis []byte, patience time.Duration) ([]byte, error) {
	var out []byte
	err := until(ctx, patience, func() (bool, error) {
		files, _ := os.ReadDir(dl)
		for _, f := range files {
			b, err := os.ReadFile(filepath.Join(dl, f.Name()))
			if err == nil && len(b) > 4 && string(b[:4]) == "%PDF" && !bytes.Equal(b, notThis) {
				out = b
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("no second PDF was handed back: %w", err)
	}
	return out, nil
}

// say prints whatever the tab said for itself, which is where a program that
// fell over on its own leaves the reason.
func say(c *conn) {
	for {
		select {
		case ev := <-c.events:
			switch ev.Method {
			case "Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Log.entryAdded":
				fmt.Println("tab:", ev.Method, string(ev.Params))
			}
		default:
			return
		}
	}
}

// canvas is what the tab actually has on its canvas.
type canvas struct {
	N int `json:"n"`
	// Dark is how many pixels are nearly black. It tells a lit control from an
	// unlit one, which is what it is for.
	Dark int `json:"dark"`
	// Ink is how many pixels are dark the way text is dark. A scanned page
	// scaled to fit a window is grey, not black: the same page counts 1805
	// pixels this way and 9 the other.
	Ink  int `json:"ink"`
	Hash int `json:"hash"`
}

// look reads the pixels out of the whole canvas in the tab.
func look(ctx context.Context, c *conn, sid string) (canvas, error) {
	return lookAt(ctx, c, sid, spot{W: -1})
}

// lookAt reads the pixels out of one band of the canvas — the page, or the
// panel of settings beside it — AS THE WORKBENCH PUBLISHES IT. A W of -1 is
// the whole canvas.
//
// ⛔ The band used to be written here as fractions of the canvas: from 7% down
// to 95%, left of 66%. Those were right for a strip of buttons along the top
// and wrong the moment the band above the page grew — a baseline pixel taken
// inside the ribbon measures the ribbon, and the check then reports that the
// document never arrived on a canvas it is drawn on perfectly well.
func lookAt(ctx context.Context, c *conn, sid string, at spot) (canvas, error) {
	box := fmt.Sprintf("{x: %d, y: %d, w: %d, h: %d}", at.X, at.Y, at.W, at.H)
	if at.W < 0 {
		box = "null"
	}
	js := fmt.Sprintf(`(() => {
	  const c = document.getElementById('screen');
	  const b = %s;
	  const x0 = b ? b.x : 0, y0 = b ? b.y : 0;
	  const x1 = b ? b.x + b.w : c.width, y1 = b ? b.y + b.h : c.height;
	  const d = c.getContext('2d').getImageData(x0, y0, x1 - x0, y1 - y0).data;
	  const r0 = d[0], g0 = d[1], b0 = d[2];
	  let n = 0, dark = 0, ink = 0, hash = 0;
	  for (let i = 0; i < d.length; i += 4) {
	    if (d[i] !== r0 || d[i+1] !== g0 || d[i+2] !== b0) n++;
	    if (d[i] + d[i+1] + d[i+2] < 240) dark++;
	    // Ink, as opposed to nearly black. Scanned text that has been scaled
	    // down to fit a window is grey: the same page counts 1805 pixels this
	    // way and 9 the other, so which threshold is used decides whether a
	    // scanned document is seen to arrive at all.
	    if ((d[i] * 299 + d[i+1] * 587 + d[i+2] * 114) / 1000 < 128) ink++;
	    hash = (hash * 31 + d[i] + d[i+1] * 3 + d[i+2] * 7) | 0;
	  }
	  return JSON.stringify({n, dark, ink, hash});
	})()`, box)
	var out canvas
	s, err := eval(ctx, c, sid, js)
	if err != nil {
		return out, err
	}
	var quoted string
	if json.Unmarshal([]byte(s), &quoted) == nil {
		s = quoted
	}
	return out, json.Unmarshal([]byte(s), &out)
}

// click presses the canvas at a point in its own pixels, which is not the same
// as a point on the page: the canvas is drawn at whatever width the layout
// gives it.
func click(ctx context.Context, c *conn, sid string, x, y int) error {
	s, err := eval(ctx, c, sid, fmt.Sprintf(`(() => {
	  const c = document.getElementById('screen');
	  const r = c.getBoundingClientRect();
	  return JSON.stringify({x: r.left + %d * r.width / c.width, y: r.top + %d * r.height / c.height});
	})()`, x, y))
	if err != nil {
		return err
	}
	var quoted string
	if json.Unmarshal([]byte(s), &quoted) == nil {
		s = quoted
	}
	var at struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(s), &at); err != nil {
		return err
	}
	for _, kind := range []string{"mousePressed", "mouseReleased"} {
		if _, err := c.call(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": kind, "x": at.X, "y": at.Y, "button": "left", "clickCount": 1,
		}, sid); err != nil {
			return err
		}
	}
	return nil
}

// eval runs a snippet in the tab and gives back what it evaluated to, as JSON.
func eval(ctx context.Context, c *conn, sid, js string) (string, error) {
	r, err := c.call(ctx, "Runtime.evaluate", map[string]any{
		"expression": js, "returnByValue": true, "awaitPromise": true}, sid)
	if err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(r, &out); err != nil {
		return "", err
	}
	if out.Exception != nil {
		return "", fmt.Errorf("the tab threw: %s", out.Exception)
	}
	return string(out.Result.Value), nil
}

// screenshot saves what the tab looks like.
func screenshot(ctx context.Context, c *conn, sid, path string) error {
	r, err := c.call(ctx, "Page.captureScreenshot", map[string]any{"format": "png"}, sid)
	if err != nil {
		return err
	}
	png, err := decodeShot(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, png, 0o644)
}

// until runs a condition until it holds or the time runs out.
func until(ctx context.Context, d time.Duration, cond func() (bool, error)) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(60 * time.Millisecond):
		}
	}
	return fmt.Errorf("waited %s and it never happened", d)
}

// waitEvent waits a little while for one kind of event.
func waitEvent(c *conn, method string, d time.Duration) (event, bool) {
	deadline := time.After(d)
	for {
		select {
		case ev := <-c.events:
			if ev.Method == method {
				return ev, true
			}
		case <-deadline:
			return event{}, false
		}
	}
}

// drain throws away whatever happened before now.
func drain(c *conn) {
	for {
		select {
		case <-c.events:
		default:
			return
		}
	}
}
