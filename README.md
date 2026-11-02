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

Every verb carries a mark from
[go-icons/iconoir](https://github.com/go-icons/iconoir), rasterised and
recoloured by the toolkit rather than outlined here. A test asks the pack
whether it has each of them, because `DrawIconoir` answers a name it does not
know by painting **nothing at all** — on a ribbon, whose large form is an icon
above its text, a missing mark is a hole where a third of the control should
be. It caught two the first time it ran. Each group leads with exactly one
large control, its primary command, which is what makes a ribbon readable at a
glance rather than a wall of equal buttons.

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
