# browsercheck

Drives the workbench in a real browser over the DevTools protocol, and looks
at the pixels that come back. Nothing here is eyeballed.

It serves `web/`, starts Chrome with no window, waits for the wasm to start,
then presses along the toolbar until the file picker is asked for — that press
is Open, and nothing else on the strip asks for a file — hands the picker a
document it built itself, and reads the canvas back to prove the page arrived.
Then it presses on until something is downloaded, which is Save, and checks
what landed on disk begins with `%PDF`.

```
go run ./browsercheck ../web            # a document it makes up
go run ./browsercheck ../web some.pdf   # one of yours
```

`CHROME` names a browser outright; otherwise the usual places are tried.

It is a module of its own so that the workbench keeps to the fleet's own
libraries and the standard library, and this one dependency — a WebSocket
client to speak to the browser — stays out of it.

Finding a defect this way is the point of it: a file arriving from the browser
long after the press that asked for it has no event to be drawn on, and the
canvas simply never changed. Nothing in the native tests could have said so.
