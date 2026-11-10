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
	"strconv"
	"sync"
	"sync/atomic"
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
	// ⛔ THE DISPLAY'S PIXEL RATIO, AND IT WAS 1 FOREVER. Headless Chrome runs
	// at 1 unless told otherwise, so for as long as this check ran once it
	// could only ever see the display nobody has a problem with. The sibling
	// playground came up unreadable on every Retina screen while its own
	// browser check stayed green for exactly this reason.
	dpr := 1.0
	if len(os.Args) > 4 {
		v, err := strconv.ParseFloat(os.Args[4], 64)
		if err != nil || !(v > 0) {
			return fmt.Errorf("%q is not a pixel ratio a display has", os.Args[4])
		}
		dpr = v
	}
	// ⛔⛔ THE COLOUR SCHEME IS SAID, NOT INHERITED, AND THIS IS THE THIRD TIME
	// IT HAS COST SOMETHING. Headless Chrome reports whatever the machine
	// reports, so a run meant the display of whoever started it:
	//
	//   - the page-arrival check asked for MORE ink than the empty band had,
	//     which is only true when the paper is lighter than what is behind it;
	//     it could not run on a dark display at all, and was green in CI
	//     because the runners are light (see the ⛔⛔ in check.go);
	//   - the theme gate asserted its FIRST transition, and so reported a
	//     defect twice about working code — once on a dark laptop, once on a
	//     light runner.
	//
	// Both are a check that knew one theme. A default of "light" makes a bare
	// local run say the same thing as CI, and `dark` is what the gate runs
	// beside it so that neither can go on knowing only one.
	scheme := "light"
	if len(os.Args) > 5 {
		scheme = os.Args[5]
	}
	switch scheme {
	case "light", "dark", "system":
	default:
		return fmt.Errorf("%q is not a colour scheme: light, dark, or system to take the "+
			"machine's own and be at its mercy", os.Args[5])
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	buildStamp.Store(firstBuild)
	srv := &http.Server{Handler: noStore(stamped(http.FileServer(http.Dir(root))))}
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
		"--force-device-scale-factor=" + strconv.FormatFloat(dpr, 'g', -1, 64),
	}
	fmt.Printf("driving the workbench at a pixel ratio of %g, with the browser reporting %s\n", dpr, scheme)
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

	// Long enough for the panel to be driven as well as the strip: the sweep
	// that finds a group and then a verb inside it is a few hundred presses,
	// each of which waits for the canvas to catch up.
	ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancel()
	c, err := dial(ctx, wsURL)
	if err != nil {
		return err
	}
	defer c.close()

	return check(ctx, c, page, sample, shot, dl, scheme)
}

// noStore keeps the browser from serving a stale build out of its own HTTP
// cache.
//
// ⛔ It does NOT keep the service worker from serving one. A worker's Cache
// Storage is its own: `caches.put` keeps whatever it is handed and
// `caches.match` returns it, and neither consults Cache-Control. This header
// is why the browser checks never saw the worker pin a returning visitor to
// the build they first loaded — the one defect in the shell that no amount of
// driving the canvas could find. shellReaches is the check that does.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// buildStamp is what /build.txt says. It stands in for the whole shell: the
// worker treats every GET alike, so one small file that the check can change
// under the browser's feet answers the question a 20MB wasm would answer more
// slowly — does a build published after somebody's first visit reach them?
var buildStamp atomic.Value

// stamped serves /build.txt from buildStamp and everything else from disk.
func stamped(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/build.txt" {
			h.ServeHTTP(w, r)
			return
		}
		s, _ := buildStamp.Load().(string)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, s)
	})
}

// browserStartup is how long a browser is given to open its debugging port.
// A cold build machine takes a good deal longer than a warm laptop — and the
// browser prints a page of complaints about the things a machine with no
// desktop on it has not got before it gets there.
const browserStartup = 90 * time.Second

// devtoolsURL reads the line Chrome prints when it opens its debugging port.
func devtoolsURL(stderr io.Reader) (string, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 512)
	deadline := time.Now().Add(browserStartup)
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
	ws *websocket.Conn

	// pump runs in its own goroutine and matches replies by number, so both
	// the counter and the map are touched from two goroutines at once. Without
	// this the process does not race quietly, it dies: "fatal error: concurrent
	// map writes", mid-check, with the browser still open.
	mu      sync.Mutex
	id      int
	replies map[int]chan json.RawMessage

	events chan event
}

type event struct {
	Method string
	Params json.RawMessage
}

// ⛔ EVERY TOUCH OF replies AND id GOES THROUGH THESE THREE, and that is the
// point of them. The comment on the mutex above has always said why it exists;
// pump then read and deleted from the map without taking it, which is the
// "concurrent map writes" the comment describes, sitting under a note saying
// it had been dealt with. A lock a reader has to remember to take is a lock
// somebody will forget — so there is now nowhere left to forget it.

// await reserves the next request number and the channel its reply will arrive
// on.
func (c *conn) await() (int, chan json.RawMessage) {
	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id++
	c.replies[c.id] = ch
	return c.id, ch
}

// deliver hands a reply to whoever is waiting for it, and reports whether
// anybody was. The send happens AFTER the mutex is dropped: the channel has
// room for one and could not block today, but a lock held across a send to a
// channel somebody else owns is how that stops being true.
func (c *conn) deliver(id int, msg json.RawMessage) bool {
	c.mu.Lock()
	ch, ok := c.replies[id]
	delete(c.replies, id)
	c.mu.Unlock()
	if !ok {
		return false
	}
	ch <- msg
	return true
}

// forget drops a request nobody will read the reply of. Deleting twice is a
// no-op, which is what lets call defer this unconditionally.
func (c *conn) forget(id int) {
	c.mu.Lock()
	delete(c.replies, id)
	c.mu.Unlock()
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
			reply := msg.Result
			if msg.Error != nil {
				reply = json.RawMessage(`{"__error":` + string(msg.Error) + `}`)
			}
			c.deliver(msg.ID, reply)
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
	id, ch := c.await()
	// Whatever happens next, this entry must not outlive the call: a write
	// that fails or a context that ends would otherwise leave pump holding a
	// channel nobody will ever read.
	defer c.forget(id)
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
