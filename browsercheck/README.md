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
go run ./browsercheck ../web                              # a document it makes up
go run ./browsercheck ../web some.pdf                     # one of yours
go run ./browsercheck ../web "" /tmp/shot.png 2 dark      # Retina, dark
```

The arguments are the directory to serve, a document, where to put the
screenshot, the display's pixel ratio, and the colour scheme the browser is to
report — `light` (the default), `dark`, or `system` to take the machine's own.

⛔ **The scheme is said, not inherited, and CI runs both.** Headless Chrome
reports whatever the machine reports, so a run used to mean the display of
whoever started it — and these runners are light. Two defects came of that: the
page-arrival check asked for *more* ink than the empty band had, which is only
true when the paper is lighter than what is behind it; and the theme gate
asserted its **first** transition, and so reported a defect twice about working
code — once on a dark laptop, once on a light runner. Both were a check that
knew one theme.

The gate is proved rather than assumed: putting the old light-only assumption
back makes the **light** run pass and the **dark** run fail with *"the document
never appeared where the page is drawn"*.

`CHROME` names a browser outright; otherwise the usual places are tried.

It is a module of its own so that the workbench keeps to the fleet's own
libraries and the standard library, and this one dependency — a WebSocket
client to speak to the browser — stays out of it.

Finding a defect this way is the point of it: a file arriving from the browser
long after the press that asked for it has no event to be drawn on, and the
canvas simply never changed. Nothing in the native tests could have said so.

## It performs a gesture, not only presses

⛔ For a long time everything here was a **press**, and a press is one point.
The divider between the minipages and the page only answers to a gesture —
press, move while held, let go — so this check was green before and after a
divider that moved nothing in a browser. The person using the workbench is who
found that out, not CI. A check that cannot perform the gesture a feature is
made of cannot see that feature fail.

`drag` now presses, sends eight `mouseMoved` events with `buttons: 1` (Chrome
ignores a move that does not say a button is still held), and releases. More
than one move, because the defect class is *in the sequence*: a single jump from
the handle to the destination already lands where the pointer is going.

It found two defects on its first run: the minipages never came back after a
resize, and a repaint during a drag cancelled the drag. Both are in the
workbench's README.

## The pixel test works in both themes

⛔ It used to ask for **more** ink than the empty band had, which is only true
when the paper is lighter than what is behind it. On a display whose browser
reports `prefers-color-scheme: dark` the empty band is nearly all ink and a page
arriving is a large pale rectangle, so the count **falls** — measured
2026-11-05: 811 671 with nothing open, 515 818 with the sample open. The check
then waited out its timeout and reported *the document never appeared where the
page is drawn* about a document that was plainly drawn, in a reading where the
workbench's own answer said a page was loaded.

It was green in CI the whole time, because GitHub's runners are light. It now
asks whether a lot of the band is **different**, whichever way round the two
colours are.
