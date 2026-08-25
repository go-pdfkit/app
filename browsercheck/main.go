// browsercheck drives the real browser over the DevTools protocol: it serves
// the offline shell, opens it in Chrome with no window, hands the file input a
// PDF the way a person would, presses the controls on the strip, and looks at
// the pixels that come back. Nothing here is eyeballed.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/coder/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS")
}

// chromePaths is where a headless browser is looked for, in order. The first
// one that is there is used; CHROME names one outright.
var chromePaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/usr/bin/google-chrome",
	"/usr/bin/chromium-browser",
	"/usr/bin/chromium",
	"/opt/google/chrome/chrome",
}

// findChrome is the browser to drive, or the reason there is none.
func findChrome() (string, error) {
	if p := os.Getenv("CHROME"); p != "" {
		return p, nil
	}
	for _, p := range chromePaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	for _, name := range []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no browser found; set CHROME to one")
}

func run() error {
	root := os.Args[1]
	sample := ""
	if len(os.Args) > 2 {
		sample = os.Args[2]
	}
	shot := "/tmp/browsercheck.png"
	if len(os.Args) > 3 {
		shot = os.Args[3]
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: noStore(http.FileServer(http.Dir(root)))}
	go srv.Serve(ln)
	defer srv.Close()
	page := fmt.Sprintf("http://%s/index.html", ln.Addr())
	fmt.Println("serving", root, "at", page)

	profile, err := os.MkdirTemp("", "chromeprofile")
	if err != nil {
		return err
	}
	defer os.RemoveAll(profile)
	if sample == "" {
		if sample, err = writeSample(profile); err != nil {
			return err
		}
	}
	if _, err := os.Stat(sample); err != nil {
		return fmt.Errorf("the sample to hand the tab is not there: %w", err)
	}
	dl := filepath.Join(profile, "downloads")
	os.MkdirAll(dl, 0o755)

	chrome, err := findChrome()
	if err != nil {
		return err
	}
	args := []string{
		"--headless=new", "--remote-debugging-port=0", "--no-first-run",
		"--no-default-browser-check", "--disable-gpu",
		"--user-data-dir=" + profile,
		"--window-size=1060,1040",
	}
	if os.Getenv("CHROME_NO_SANDBOX") != "" {
		// A build runner has no user namespaces to put the renderer in, so
		// Chrome refuses to start at all. Turning its sandbox off is safe
		// only because the only page it will ever load is the one served
		// three lines up, from this machine, by this program.
		args = append(args, "--no-sandbox", "--disable-dev-shm-usage")
	}
	cmd := exec.Command(chrome, append(args, "about:blank")...)
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Process.Kill()

	wsURL, err := devtoolsURL(stderr)
	if err != nil {
		return err
	}
	fmt.Println("devtools at", wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	c, err := dial(ctx, wsURL)
	if err != nil {
		return err
	}
	defer c.close()

	return check(ctx, c, page, sample, shot, dl)
}

// noStore keeps the browser from serving a stale build out of its own cache.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// devtoolsURL reads the line Chrome prints when it opens its debugging port.
func devtoolsURL(stderr io.Reader) (string, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 512)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		n, err := stderr.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if i := indexOf(string(buf), "ws://"); i >= 0 {
			s := string(buf[i:])
			for j := 0; j < len(s); j++ {
				if s[j] == '\n' || s[j] == '\r' {
					return s[:j], nil
				}
			}
		}
		if err != nil {
			return "", fmt.Errorf("chrome said: %s", buf)
		}
	}
	return "", fmt.Errorf("chrome never printed a debugging url: %s", buf)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// A conn is one DevTools session: requests numbered, replies matched by number,
// events kept in a list so a test can wait for one.
type conn struct {
	ws      *websocket.Conn
	id      int
	replies map[int]chan json.RawMessage
	events  chan event
}

type event struct {
	Method string
	Params json.RawMessage
}

func dial(ctx context.Context, url string) (*conn, error) {
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(64 << 20)
	c := &conn{ws: ws, replies: map[int]chan json.RawMessage{}, events: make(chan event, 4096)}
	go c.pump(ctx)
	return c, nil
}

func (c *conn) close() { c.ws.Close(websocket.StatusNormalClosure, "") }

func (c *conn) pump(ctx context.Context) {
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.ID != 0 {
			if ch, ok := c.replies[msg.ID]; ok {
				if msg.Error != nil {
					ch <- json.RawMessage(`{"__error":` + string(msg.Error) + `}`)
				} else {
					ch <- msg.Result
				}
				delete(c.replies, msg.ID)
			}
			continue
		}
		select {
		case c.events <- event{msg.Method, msg.Params}:
		default:
		}
	}
}

// call sends one command and waits for its reply.
func (c *conn) call(ctx context.Context, method string, params map[string]any, sessionID string) (json.RawMessage, error) {
	c.id++
	id := c.id
	ch := make(chan json.RawMessage, 1)
	c.replies[id] = ch
	req := map[string]any{"id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	if sessionID != "" {
		req["sessionId"] = sessionID
	}
	data, _ := json.Marshal(req)
	if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		var e struct {
			Err json.RawMessage `json:"__error"`
		}
		if json.Unmarshal(r, &e) == nil && e.Err != nil {
			return nil, fmt.Errorf("%s: %s", method, e.Err)
		}
		return r, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

func decodeShot(r json.RawMessage) ([]byte, error) {
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(r, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}
