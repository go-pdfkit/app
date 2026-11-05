# app

[![CI](https://github.com/go-pdfkit/app/actions/workflows/ci.yml/badge.svg)](https://github.com/go-pdfkit/app/actions/workflows/ci.yml)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen.svg)](#how-it-is-checked)

A PDF workbench that runs in a browser tab and nowhere else. Open a file,
turn its pages, rotate one, drop one, lay it out two to a sheet, write
across it, strip what runs rather than shows, save it back out — and none
of it leaves the machine, because there is nowhere for it to go. The whole
tool is one wasm binary the browser downloads once and then keeps.

It is the front of the [`go-pdfkit`](https://github.com/go-pdfkit) stack:
[`reader`](https://github.com/go-pdfkit/reader) parses and writes,
[`ops`](https://github.com/go-pdfkit/ops) is the verbs,
[`render`](https://github.com/go-pdfkit/render) turns a page into pixels,
and [`go-widgets`](https://github.com/go-widgets/toolkit) draws the
controls onto a canvas through
[`webcanvas`](https://github.com/go-widgets/webcanvas). Pure Go, no C, no
network.

## Offline for real

The service worker keeps a copy of the shell and the binary, so a visit with
the network off works — and so does the first one, once loaded, since nothing
is fetched after start-up. A file is read through the browser's own picker
and handed back as a download; the bytes never touch a server, and there is
no server to touch.

It asks the network **first** and falls back on that copy, rather than the
other way round. Cache-first is the usual shape and it is a trap: a worker
only reinstalls when its own script changes byte for byte, so the copy taken
on somebody's first visit is the one they keep, and five releases in a row
reached nobody who had been here before. `browsercheck` changes a file under
the browser, reloads, and fails if the tab is still served the old one.

⛔ **And the status line says which build it is**, at the bottom right, on the
empty workbench as well as an open document: `v0.19.0` for a released build,
`v0.19.0-2-g0aecb79` for one two commits past it. That is the one question a
reader cannot answer by looking — an old binary and a missing feature are the
same screen — and it is what the cache trap above cost before anybody could
ask it. The deploy passes the string with `-ldflags -X` and then **greps the
built wasm for it**, because a `-X` naming a variable that no longer exists is
accepted in silence.

⛔ **A release is tagged after the deploy that carries it**, so the published
binary would name the *previous* one for ever: a merge to main deploys, and
only once main is green does the tag go on, so the deploy builds a commit no
tag points at yet. Measured the first time it ran — the live binary said
`v0.18.0-2-gc1c82b4` while the release was v0.19.0, which is the same confusion
one step removed. Publishing a release now dispatches the deploy again, on
`main`, where `git describe` returns the bare tag. It dispatches rather than
deploying itself because the Pages environment admits one ref and a release
event's is the tag.

## What is on the ribbon, and what is beside the page

Seven tabs: **Home** · **Pages** · **Sheet** · **Marks** · **File** ·
**Protect** · **Read**. Home carries the page itself — **Rotate**,
**Previous**, **Next**, **Delete**, and **Fill in** for a document that carries
a form. The arrow keys turn pages too.

**Open** and **Save** are on the ribbon's quick access bar, at the right of the
tab row, and on no tab at all. A ribbon puts each command on one tab, so Open
and Save on *Home* meant leaving whatever tab you were working in to save — the
browser check had to walk back to Home for it, which is the clearest possible
statement that a reader would have had to as well. The bar is there on every
tab and on a collapsed ribbon, and answers to **Alt** then **1** or **2**.

**The ribbon carries the verbs; the panel carries what they are told.** That
one sentence decides where every control goes. *Turn them* is a verb and sits
on the ribbon; *a quarter / a half / three quarters* is what it is told and
sits in the panel beside the page. A tab **is** a panel: choosing **Marks**
opens the Marks settings beside the page, because a group of verbs and the
settings those verbs read are the same subject seen twice.

This replaced a strip of thirteen buttons, six of which were not verbs at all
but switches that opened a panel of verbs — so the twenty-five things the
library does were two presses away and named nowhere a reader could see them.
The panels were a caption, a box and a button under it, which is why each was a
scrolling column that ran off its own bottom edge.

What each tab commands, and what its panel holds:

- **Pages** — which pages to keep or drop, turn them a quarter, a half or
  three quarters, reverse the order, move the one on the screen, crop to a
  box, put a blank page in, split into files of *n* pages.
- **Sheet** — *n* pages to a sheet, fold into a booklet, add another file
  after this one, lay another file over it.
- **Marks** — a watermark, page numbers in a shape you choose, a Bates number
  padded and started where you say, and a stamp in one of nine places.
- **File** — sanitize, flatten the annotations in, drop the annotations, the
  bookmarks or what the file says about itself, pack it smaller, give it a
  title and an author.
- **Protect** — open a file that has a password, put a user and an owner
  password on this one and say what a reader may do with it, or take the
  protection off.
- **Read** — what the page says, and what it carries: every picture on it,
  listed with its size and its shape, each one handed over under a name that
  says what it holds. And the page itself the other way round — drawn at twice
  the size it is shown at and handed over as a **PNG, JPEG, GIF, TIFF or BMP**,
  one page or every page zipped, with the words on their own as a text file.
  The formats offered are the ones `gfx` can write, asked of it rather than
  listed here; a page that ran out of time is still handed over, and the status
  line says how far it got.

A protected document is still drawn, because the workbench reads its own
output back with the password it was just given. It is not the same bytes
twice — encryption needs randomness, by design — but it is the same document,
protected the same way.

Beside, because nearly every one of those changes the document and the document
is drawn from what would be saved: typing a crop box and watching the page come
back cropped is the whole point of the control, and a panel over the page would
hide the one thing worth looking at. **Read** is the exception that keeps the
rule — it changes nothing, and what it reads it reads out of the document
written and read back, so what it lists is what would come out of Save.

Every change is applied to the document, written out, and read back before
it is drawn — so what is on the screen is what would come out of Save,
rather than a picture of what was meant to happen.

## Where you are in it

A document of more than one sheet gets a rail of minipages down the left:
one numbered tile per page, the one you are looking at marked, and a press
goes there. Each tile is the shape of its own page, so a landscape table in
the middle of a portrait report is findable before any word in it is
legible.

The rail is bounded on purpose. Pages are drawn small — at the rail's width,
not at full size and shrunk afterwards — and only within a window either
side of the page you are on; further out a tile is its frame and its number,
which is most of what the rail is for. Pages are drawn nearest-first within a
budget per frame, so a long document fills its rail in rather than holding up
the page it belongs to.

⛔ **Going to a page does not rewrite the document.** Everything the workbench
shows is built from the document *written out and read back*, so that what is
on the screen is always what would come out of Save — and every verb ends in
exactly that. Going to a page was ending in it too, for a gesture whose entire
effect is that a different number is lit: measured on a forty-page sample, one
press on a minipage wrote 8 422 bytes, parsed them back and re-rendered twenty
pages, and both of those grow with the file, so the press got slower the bigger
the document. It was reported as *the app lags*. A test now counts the writes
and fails on any; its control is a verb, which must still pay the whole bill.

Every verb carries a mark from
[go-icons/iconoir](https://github.com/go-icons/iconoir), rasterised and
recoloured by the toolkit rather than outlined here. A test asks the pack
whether it has each of them, because `DrawIconoir` answers a name it does not
know by painting **nothing at all** — on a ribbon, whose large form is an icon
above its text, a missing mark is a hole where a third of the control should
be. It caught two the first time it ran. Each group leads with exactly one
large control, its primary command, which is what makes a ribbon readable at a
glance rather than a wall of equal buttons.

## It follows the display

The workbench implements webcanvas's `PixelRatioAware`, so on a screen that
puts two device pixels in a CSS pixel it is drawn at the screen's own
resolution **and** every widget, band, margin and minipage is laid out twice as
large. Both halves, and a browser zoom or a window dragged to another display
says so again.

⛔ It used to do only the first half — size its framebuffer by the ratio and
scale nothing else — so the whole interface came out sharp and half the size it
should be. Its sibling, the [playground](https://go-pdfkit.github.io/playground/),
had the complementary half, scaling the widgets and not the surface, and showed
nothing at all. Neither failed anything: headless Chrome runs at ratio 1 unless
told otherwise, so the one condition both defects needed was the one condition
CI never produced. The browser check takes a ratio now and runs at **1 and 2**.

### The rail shows the contents AND the minipages

The document's table of contents — its `/Outlines` tree — sits **above** the
minipages, each row indented by its depth and pressable to go to its page, with
a divider between the two that can be dragged, stepped with the arrow keys, and
double-clicked to collapse. A switch on the quick bar puts the contents away
and brings them back.

⛔ **Above, not instead of.** The first version of this swapped the rail
between the two, arguing that they answer one question and two columns
answering it would take the page's width twice. That argument is about width
and it does not survive stacking: the contents say what a part is *called* and
the minipages say what it *looks like*, and a reader after "the table near the
start" is served by both at once and by neither alone.

⛔ **A document with no outline gets its pages listed, not an empty pane and
not nothing.** The pane used to be withheld from a file with no `/Outlines` —
which is *most* files — so the view shipped and was reported missing by
somebody running the very build that added it. A contents view only the
minority of documents can open is a contents view nobody finds. The fallback is
not a consolation prize either: a column of titles is how you reach page 340 of
a scanned book, because it scrolls in one flick where four hundred pictures do
not. A document that labels its pages something other than their position —
`iv`, `A-2`, a plate number — says both, because the label is what its own
index refers to and the position is what every other control here counts in.

⛔ **Every link in an outline comes out of the file.** It is a tree of indirect
references, and nothing in the format stops a file pointing `/Next` at an item
already visited, nesting a thousand deep, or carrying a hundred thousand
entries. The walk is bounded on all three axes, and the three bounds do
different work: the count stops a cycle *spinning*, the visited set stops it
coming back as a thousand copies of the same two rows, and the depth bound
bounds the recursion itself — the only one of the three that would otherwise
end the process rather than merely waste it.

A bookmark whose destination this document cannot resolve **keeps its row** and
loses its press: the row is still what the document says about itself, and
dropping it would renumber everything under it.

### A secondary press names the page it will act on

Right-clicking a minipage or the page itself opens a menu of the verbs that act
on **one page** — turn it, drop it, put a blank before it, hand it over as a
picture, read its words.

⛔ **It names the page, and it goes there first.** Every verb in it already
exists on the ribbon, where it acts on *the page being read*. A menu opened
over minipage 7 that then turned whatever page the viewer happened to be
showing would be worse than no menu at all — so opening it moves the workbench
to the page under the pointer, and the first row says which page that is.
Nothing in the menu is a verb the ribbon lacks; what it adds is **the page**.

The rows are read from the ribbon's own verb table by id, never restated, so a
rename cannot leave the menu calling something by its old name — and there is
no "skip an unknown id", because that would let a renamed verb drop out of the
menu in silence.

### The rail is resizable, and the minipages follow

The band is a `toolkit.Splitter` rather than a fixed row: the divider between
the rail and the page can be dragged, stepped with the arrow keys, and
double-clicked to put the rail away and bring it back at the width it had. The
floor is a tile still worth looking at; the ceiling is half the surface,
because a rail wider than the page it is a map of has stopped being a rail.

⛔ **The minipages follow the width, which is the whole of what "resizable"
has to mean.** A rail that got wider while its tiles stayed the same size would
be a rail with a margin. Every tile is measured, laid out *and rendered* from
the live width, and `pageW` is cut from it too — a rail the reader has widened
with a page still drawn to the old width is a page drawn under the rail.

⛔ **The pictures are re-made when the gesture ends, not while it runs.** Each
minipage is a page put through the renderer at the rail's width, so re-rendering
them on every frame of a drag would render the whole window's worth of pages
dozens of times on the way to a width the reader has not chosen yet. During the
drag the tiles are the old pictures scaled into the new boxes, which is what a
`Thumbnail` does anyway.

### The page band is a viewer

One page scaled to fit was all there was. The band now holds
`toolkit.PagedView`: **zoom in and out**, **fit to width**, **fit to page**,
and a choice between one page at a time and a **continuous stack** of every
page in the document — with the wheel, PageUp/PageDown, Home/End and a
`n / N` read-out, none of it written here.

⛔ **The viewer outlives the document it is showing.** Everything else in the
workbench is rebuilt by `refresh`, because everything else is a function of the
document as it now stands. A viewer is not: its zoom, its layout mode and where
it is scrolled to are the *reader's*, and rebuilding it on every edit would
hand back a page at 100% and at the top after each crop — the moment somebody
most wants to still be looking at what they were looking at.

⛔ **Every page is laid out; few are drawn.** The sizes of all of them are
declared, so the scrollbar is honest and a page number means what it says,
while bitmaps are held only for a window around the reader (`SetPageBitmap`,
added to the toolkit for this). A tile in the rail is small and a page is not:
a five-hundred-page document held whole would be more memory than the tab has.

⛔ **A page is drawn at the size it is shown at**, not drawn once and scaled.
`PagedView` scales a bitmap to the card it declared, so a page rendered at one
zoom and then magnified is that first rendering enlarged — soft, and softest
exactly when somebody has zoomed in to look closely.

The frame budget is the rail's: what is not finished in 400 ms is left for the
next repaint, which the scene asks for. A page the renderer *refuses* is
finished, not pending — asking again would be a loop rather than a retry.

### Light, dark, or whatever the machine says

The switch on the quick bar cycles **system → light → dark → system**, and
starts on *system*.

⛔ **Three states and not two.** A switch that only says light-or-dark has no
way to say "whatever this machine is set to", so the first press takes the
workbench out of the system's hands for good — including at sunset, when the
system flips and the workbench does not. The third state is the default, which
is what makes a reader who never touches the button still get the right one.

The system's own answer comes from `matchMedia("(prefers-color-scheme: dark)")`,
and the workbench subscribes to it: there is no command-line flag and no
document property that says "this machine is in dark mode", and the system can
change while the tab is open. Both listener spellings are registered —
`addEventListener` and the deprecated `addListener` Safari carried alone for
years — because registering only the first is a workbench that follows the
system on some machines and silently does not on others.

The choice is remembered in `localStorage` and **not** written back when it is
restored: a write on every visit would overwrite a choice made in another tab
between this tab's read and its write. A value that cannot be read reads as
*system*; a write that throws (a private window, blocked site data, a full
quota) is swallowed, and the choice then lasts as long as the tab.

The page picture itself is not recoloured. A PDF page is paper, and paper does
not turn black because the window did.

Text is set in [IBM Plex Sans](https://github.com/go-opentype/fonts) through
the toolkit's TrueType path. It replaced Inter because a reader said the
lowercase `g` read as a `9`, and that turned out to be measurable: rendering
both glyphs from every family the fleet embeds and comparing their ink, Inter
shares 56.7% of it between `g` and `9` and Plex 24.2%. The band and row
heights follow the installed face rather than being constants, which is what
lets the workbench change typeface at all — the numbers it used to be laid
out with were the padding a 5×7 bitmap had been given.

## The panel and its datum are the same value

Every control in a panel is **two-way bound** to the thing it shows, through
[`mvvm`](https://github.com/go-widgets/mvvm) and its `tkbind` adapters — the
box, the spinner, the list and the tick box each hold the same
`Observable[T]` the rest of the program reads.

They used to be plain fields filled in by a `Subscribe` on each control: a
value could travel from the control to the model and never the other way,
because the only way back in was to rebuild the widget, and a panel is built
once and kept. Nothing could correct a number from outside, so nothing tried
— dropping pages 3 to 9 left *"Put a blank page before 7"* still offering a
page that was no longer there.

The cycle buttons are bound through a table saying what each **position**
means, since a quarter turn is `90`, `180` or `270` and the button sits at
`0`, `1` or `2`. The form panel's fields are bound the same way, and what the
*file* kept is written back: `SetText` truncates to `MaxLen` and strips
newlines without saying so, and the box used to go on showing nine characters
while the document held five.

Nothing is wired to a widget by hand any more. The three `Subscribe` calls
left are the other boundary — a field's datum being written into the PDF.

## A control says beforehand whether it can be pressed

Every verb is one [`toolkit.Action`](https://pkg.go.dev/github.com/go-widgets/toolkit#Action)
— the toolkit's single source of truth for a command — with a rule, and the
rule drives whether the control may be used at all: the arrows grey at the ends
of a document, **Delete** greys on a document of one page, **Fill in** greys
unless there is a form, and everything greys when nothing is open. One Action
per verb means a verb that cannot run greys on every surface showing it at
once.

Each of those rules already existed inside its handler, where it answered
*afterwards* with a sentence in the status line. The handlers keep their
guards — they are the contract of the verb, not of the button — so this does
not make anything safer. It makes the workbench say it first.

## How it is checked

The workbench is a plain Go type with no build tag, so a native test drives
the whole of it against a byte buffer: open a document built in the test,
press a control where the ribbon drew it, and look at what got drawn. Every
control is reached by pressing it rather than by calling its handler, which is
what says the thing on the screen is wired to the thing it says it is —
`Ribbon.ControlRect` is the ribbon's own answer to where a control ended up, so
the press follows the layout instead of a second copy of it.

**100% of statements**, including every branch that reports a document
that cannot be written, read back or drawn — none of which should ever
happen, which is exactly why they are worth being able to see.

Then in a real browser, over the DevTools protocol with nothing eyeballed:
Chrome loads the shell, the wasm starts, the file picker is intercepted and
handed a PDF, and the pixels where the page is drawn are read back to prove it
arrived — which is how it was found that a file arriving from the browser needs
a frame of its own, having no event to be drawn on. The check then chooses the
**Marks** tab, watches its panel of settings appear, presses the watermark verb
on the ribbon, goes back to **Home** where the file commands are, saves, and
parses what came back to find the mark in it: a tab that is painted and gets no
events looks exactly like one that works.

**A canvas has no DOM**, so the workbench says where its controls are:
`window.pdfkitControls()` publishes the ribbon's own answer — every tab, every
verb the tab now shows, whether each may be used, and where the page and the
panel are. The check used to sweep the strip from `x=0` to `x=1000` and call
whichever press opened a file picker *Open*, which could never say which verb
it had run and is out of reach entirely for controls on several rows of several
tabs. It presses by name now, and refuses to press a greyed one: pressing one
and waiting for the document to change waits out the whole timeout and reports
a working verb as broken.

## Building

```
GOOS=js GOARCH=wasm go build -o web/main.wasm .
```

then serve `web/` over HTTP. `web/wasm_exec.js` comes from the Go
distribution and must match the toolchain that built the binary.

## The rest of the family

[**reader**](https://github.com/go-pdfkit/reader) reads and writes the format ·
[**ops**](https://github.com/go-pdfkit/ops) is the verbs and the `pdfops` command ·
[**render**](https://github.com/go-pdfkit/render) turns a page into pixels ·
[**pdffont**](https://github.com/go-pdfkit/pdffont) is what a document says about a font ·
[**extract**](https://github.com/go-pdfkit/extract) reads a page back ·
[**coedit**](https://github.com/go-pdfkit/coedit) is a PDF several people edit at once ·
[**app**](https://github.com/go-pdfkit/app) is the browser workbench.

Documentation for all of it: <https://go-pdfkit.github.io/docs/>

## Licence

BSD-3-Clause.
