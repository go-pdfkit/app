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

	empty, err := look(ctx, c, sid)
	if err != nil {
		return err
	}
	fmt.Printf("empty workbench: %d pixels drawn, %d of them dark\n", empty.N, empty.Dark)
	if empty.N == 0 {
		return fmt.Errorf("the canvas is blank with nothing open")
	}

	// Press along the strip until the file picker is asked for: that press is
	// the Open control, and nothing else on the strip asks for a file.
	openedAt, err := sweep(ctx, c, sid, 0, func() (bool, error) {
		ev, ok := waitEvent(c, "Page.fileChooserOpened", 250*time.Millisecond)
		if !ok {
			return false, nil
		}
		var p struct {
			BackendNodeID int `json:"backendNodeId"`
		}
		json.Unmarshal(ev.Params, &p)
		abs, _ := filepath.Abs(sample)
		_, err := c.call(ctx, "DOM.setFileInputFiles", map[string]any{
			"files": []string{abs}, "backendNodeId": p.BackendNodeID}, sid)
		return true, err
	})
	if err != nil {
		return fmt.Errorf("no press on the strip asked for a file: %w", err)
	}
	fmt.Printf("the file picker opened from x=%d, and was handed %s\n", openedAt, filepath.Base(sample))

	if err := until(ctx, 30*time.Second, func() (bool, error) {
		v, err := look(ctx, c, sid)
		if err != nil {
			return false, err
		}
		// The canvas changed, more of it is drawn on than before, and there is
		// more ink than the empty workbench's own furniture.
		//
		// It used to ask for more NEARLY BLACK pixels, and a scanned page has
		// almost none: its ink is grey once it has been scaled to fit, and it
		// covers the strip and the borders that were the dark pixels being
		// counted. A real scanned document therefore read as never having
		// arrived, on a page that was drawn correctly.
		return v.Hash != empty.Hash && v.N > empty.N && v.Ink > empty.Ink, nil
	}); err != nil {
		say(c)
		return fmt.Errorf("the document never appeared on the canvas: %w", err)
	}
	withDoc, _ := look(ctx, c, sid)
	fmt.Printf("document shown: %d pixels drawn, %d of them ink\n", withDoc.N, withDoc.Ink)

	if err := screenshot(ctx, c, sid, shot); err != nil {
		return err
	}
	fmt.Println("what the tab looks like is in", shot)

	// Now press on until something is downloaded: that is the Save control,
	// and what lands on disk is the document as it now stands.
	drain(c)
	savedAt, err := sweep(ctx, c, sid, openedAt+4, func() (bool, error) {
		_, ok := waitEvent(c, "Browser.downloadWillBegin", 250*time.Millisecond)
		return ok, nil
	})
	if err != nil {
		return fmt.Errorf("no press on the strip saved anything: %w", err)
	}
	fmt.Printf("a download began from x=%d\n", savedAt)

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

	// The strip is no longer the whole workbench: most of the verbs now live
	// in a panel that a group opens beside the page. That panel has to be
	// driven here too, because a panel that is drawn and gets no events looks
	// exactly like one that works — which is what it was until the toolkit
	// underneath learned to hand a press to what is inside a scroll view.
	// What counts as a verb having run is that the file the tab hands back is
	// a DIFFERENT file. Some of the panels change what is on the screen
	// without changing the document at all — the Read group replaces the
	// picture of the page with what the page says — and a check that stopped
	// at the first press to redraw the canvas would call that a verb.
	var again []byte
	marked, err := drivePanel(ctx, c, sid, func() (bool, error) {
		drain(c)
		if err := click(ctx, c, sid, savedAt, stripY); err != nil {
			return false, err
		}
		got, err := waitForPDF(ctx, dl, out, 6*time.Second)
		if err != nil {
			return false, nil // the same file back: nothing was changed
		}
		again = got
		return true, nil
	})
	if err != nil {
		say(c)
		return err
	}
	fmt.Println("a verb pressed in the panel changed the document:", marked)
	fmt.Printf("the tab handed back a %d byte PDF after the panel was used\n", len(again))
	doc, err := reader.Open(again)
	if err != nil {
		return fmt.Errorf("what the tab saved after the panel was used does not open: %w", err)
	}
	content, err := doc.PageContent(1)
	if err != nil {
		return fmt.Errorf("the first page of it cannot be read: %w", err)
	}
	if !bytes.Contains(content, []byte("(DRAFT) Tj")) {
		return fmt.Errorf("the mark the panel wrote is not in the file the tab handed back")
	}
	fmt.Println("the mark written from the panel is in the saved file")
	return nil
}

// Where the panel and the page are on the canvas, and how far apart the
// presses that look for them are. The panel is three hundred of the canvas's
// thousand pixels wide, at the right hand end.
const (
	pageBand  = 0.66
	panelMid  = 840
	panelTop  = 55
	panelFoot = 690
	stripY    = 8 + 15
	// The leftmost control worth pressing from this end: everything to the
	// left of it is Open, Save, the arrows and the two that change the
	// document without being asked anything, and none of those opens a panel.
	stripStop = 306
)

// drivePanel opens a group of verbs from the strip and presses the verbs in it
// until one of them changes the page, which is what says the panel is wired to
// the document rather than merely painted next to it.
//
// It works from the right hand end of the strip, so that the sweep never
// presses the controls that drop a page.
func drivePanel(ctx context.Context, c *conn, sid string, confirm func() (bool, error)) (string, error) {
	quiet, err := lookIn(ctx, c, sid, pageBand, 1)
	if err != nil {
		return "", err
	}
	// A control is wider than the step, so the same group opens several times
	// running. Its panel is told apart by what it looks like, and one that has
	// already been pressed all the way down is not pressed again.
	tried := map[int]bool{}
	for x := 996; x > stripStop; x -= 8 {
		if err := click(ctx, c, sid, x, stripY); err != nil {
			return "", err
		}
		opened, err := changedIn(ctx, c, sid, pageBand, 1, quiet.Hash)
		if err != nil {
			return "", err
		}
		if !opened {
			continue
		}
		panel, err := lookIn(ctx, c, sid, pageBand, 1)
		if err != nil {
			return "", err
		}
		if tried[panel.Hash] {
			if err := click(ctx, c, sid, x, stripY); err != nil {
				return "", err
			}
			if _, err := changedIn(ctx, c, sid, pageBand, 1, quiet.Hash); err != nil {
				return "", err
			}
			continue
		}
		tried[panel.Hash] = true
		fmt.Printf("a panel opened from x=%d\n", x)
		hit, err := pressDownPanel(ctx, c, sid, confirm)
		if err != nil {
			return "", err
		}
		if hit != "" {
			return hit, nil
		}
		// Nothing in this group changes what is drawn. Put it away and carry
		// on along the strip.
		if err := click(ctx, c, sid, x, stripY); err != nil {
			return "", err
		}
		if _, err := changedIn(ctx, c, sid, pageBand, 1, quiet.Hash); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("no group on the strip opened a panel with a verb in it")
}

// pressDownPanel presses down the open panel until a press both changes the
// page beside it and changes the document, and says where that press was.
func pressDownPanel(ctx context.Context, c *conn, sid string, confirm func() (bool, error)) (string, error) {
	page, err := lookIn(ctx, c, sid, 0, pageBand)
	if err != nil {
		return "", err
	}
	for y := panelTop; y < panelFoot; y += 16 {
		if err := click(ctx, c, sid, panelMid, y); err != nil {
			return "", err
		}
		changed, err := changedIn(ctx, c, sid, 0, pageBand, page.Hash)
		if err != nil {
			return "", err
		}
		if !changed {
			continue
		}
		did, err := confirm()
		if err != nil {
			return "", err
		}
		if did {
			return fmt.Sprintf("pressed at y=%d", y), nil
		}
		// It redrew and changed nothing that would be saved. Take the page as
		// it now stands as the new baseline and carry on down the panel.
		page, err = lookIn(ctx, c, sid, 0, pageBand)
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

// changedIn waits a moment for the canvas to catch up and reports whether the
// band changed.
//
// The wait is the point: the canvas is repainted on an animation frame rather
// than on the press itself, so a look taken straight after a click is a look
// at what was on the screen before it.
func changedIn(ctx context.Context, c *conn, sid string, from, to float64, was int) (bool, error) {
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		v, err := lookIn(ctx, c, sid, from, to)
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
	return lookIn(ctx, c, sid, 0, 1)
}

// lookIn reads the pixels out of a band of the canvas, given as fractions of
// its width. It is how the two halves of the workbench are told apart with
// nothing eyeballed: opening a group of verbs lights up the right hand band,
// and pressing one of them changes the left hand one, where the page is.
func lookIn(ctx context.Context, c *conn, sid string, from, to float64) (canvas, error) {
	// The band runs from under the strip to above the status line. Both of
	// those change for reasons that are not what is being asked about — a
	// control lights up under the pointer, a message is written at the bottom
	// — and counting them would answer a different question.
	js := fmt.Sprintf(`(() => {
	  const c = document.getElementById('screen');
	  const x0 = Math.floor(c.width * %g), x1 = Math.ceil(c.width * %g);
	  const y0 = Math.floor(c.height * 0.07), y1 = Math.floor(c.height * 0.95);
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
	})()`, from, to)
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

// sweep presses along the toolbar from left to right until what is pressed
// does the thing being waited for.
func sweep(ctx context.Context, c *conn, sid string, from int, happened func() (bool, error)) (int, error) {
	for x := from; x < 1000; x += 4 {
		if err := click(ctx, c, sid, x, 8+15); err != nil {
			return 0, err
		}
		ok, err := happened()
		if err != nil {
			return 0, err
		}
		if ok {
			return x, nil
		}
	}
	return 0, fmt.Errorf("pressed the whole strip and nothing happened")
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
