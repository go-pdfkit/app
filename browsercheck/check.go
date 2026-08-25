package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
		v, err := eval(ctx, c, sid, `document.getElementById('stage').classList.contains('ready')`)
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
		return v.Hash != empty.Hash && v.Dark > empty.Dark, nil
	}); err != nil {
		say(c)
		return fmt.Errorf("the document never appeared on the canvas: %w", err)
	}
	withDoc, _ := look(ctx, c, sid)
	fmt.Printf("document shown: %d pixels drawn, %d of them dark\n", withDoc.N, withDoc.Dark)

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
	return os.WriteFile(shot+".pdf", out, 0o644)
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
	N    int `json:"n"`
	Dark int `json:"dark"`
	Hash int `json:"hash"`
}

// look reads the pixels out of the canvas in the tab.
func look(ctx context.Context, c *conn, sid string) (canvas, error) {
	const js = `(() => {
	  const c = document.getElementById('screen');
	  const d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data;
	  const r0 = d[0], g0 = d[1], b0 = d[2];
	  let n = 0, dark = 0, hash = 0;
	  for (let i = 0; i < d.length; i += 4) {
	    if (d[i] !== r0 || d[i+1] !== g0 || d[i+2] !== b0) n++;
	    if (d[i] + d[i+1] + d[i+2] < 240) dark++;
	    hash = (hash * 31 + d[i] + d[i+1] * 3 + d[i+2] * 7) | 0;
	  }
	  return JSON.stringify({n, dark, hash});
	})()`
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
