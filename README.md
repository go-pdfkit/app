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

Twelve tabs: **Home** · **Pages** · **Sheet** · **Marks** · **Stamp** ·
**File** · **Contents** · **Redraw** · **Protect** · **Read** · **Mark up** ·
**Hand over**. Five of them exist because a measurement forced a split rather
than because a list was planned. Two different gates do the forcing: a crowded
*strip* shrinks its controls to wordless 28-pixel icons, and a crowded *panel*
runs its last rows past the room it is given, where a control can only be
scrolled to — which is a control a reader does not know is there. Both name the
fix in their own message. Home carries the page itself — **Rotate**,
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
  three quarters, reverse the order, **arrange** them as a grid and put one in
  front of another, move the one on the screen by its number, crop to a box,
  put a blank page in, straighten a crooked scan, set the paper to one of six
  sheets, paint the text one colour.
- **Sheet** — *n* pages to a sheet, fold into a booklet, cut each page up into
  a grid of smaller pages, put every page onto one, print a page larger than
  the paper across several sheets, add another file after this one, lay another
  file over it, mix another file's pages in one at a time.
- **Marks** — what is written across every page by a rule: a watermark, page
  numbers in a shape you choose, a Bates number padded and started where you
  say.
- **Stamp** — what is put where you *choose*: a line of text in one of nine
  places, a **header and a footer together** in one press, and a picture at a
  place and a width — a signature, a logo, a seal. `{page}`, `{pages}` and
  `{n}` are filled in for any of them, which is what makes a stamp at *top
  centre* a header with a page number in it.
- **File** — sanitize, flatten the annotations in, drop the annotations or what
  the file says about itself, pack it smaller, give it a title, an author, a
  subject, keywords, a creator and a producer, and **show** what it already says: every entry of its
  information dictionary, how big each page is, and whether its structure had
  to be rebuilt to read it. Files travelling inside the document are listed
  here too, with a way of putting one in, taking one out or dropping it.
- **Contents** — the table of contents a reader navigates by: bookmark the page
  on the screen, write a whole tree from a text file, or drop the bookmarks —
  and the **printed** kind, a contents page written from those same entries.
  See *Writing the table of contents* below.
- **Protect** — open a file that has a password, put a user and an owner
  password on this one and say what a reader may do with it, or take the
  protection off.
- **Read** — what the page says, and what it carries: every picture on it,
  listed with its size and its shape, each one handed over under a name that
  says what it holds. And the page itself the other way round — drawn at twice
  the size it is shown at and handed over as a **PNG, JPEG, GIF, TIFF or BMP**,
  one page or every page zipped, with the words on their own as a text file.
  The grids its words line up into are shown too, and handed over as CSV or as
  JSON with every piece of text and where it sits.
  The formats offered are the ones `gfx` can write, asked of it rather than
  listed here; a page that ran out of time is still handed over, and the status
  line says how far it got.
- **Hand over** — a copy of the document in some other form, and nothing on it
  changes what is open: this page or every page as a picture, the words as a
  text file, the grids as CSV or JSON, every page zipped or as a `.cbz`, and
  **the pieces** of a document split into files of *n* pages — in one zip,
  because a browser asks about each download separately and ten prompts can be
  refused one at a time, leaving somebody with four pieces and no way of knowing
  which four. One piece is handed over as the PDF itself; a zip holding a single
  file is a wrapper to open for nothing.

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

⛔⛔ **A document with no outline gets its LINKS listed, and that was a list of
pages first.** The fallback shipped as one row per page, and the person who
asked for the view said, on seeing it, that what they meant by a table of
contents was a display of the document's **links**. They were right, and the
difference is not presentational: a link is structure the **author** wrote —
*this phrase leads there* — while a list of pages is arithmetic that reads the
same for every file of the same length.

A row says **the words the link sits on**, because that is the only name a link
has: a PDF link is a rectangle and a destination, and nothing in it is a title.
The words under the rectangle are read off the page (`extract.Runs`, once per
page however many links sit on it), down the page and then across — runs arrive
in the order the page *draws* them, and a page is free to draw its last word
first. A link with no words under it keeps its row and names where it goes
instead: an image that is a link, a link over a blank area of a scan, a link
whose text cannot be read.

⛔ **A link that leaves the document is shown and not followed.** Pressing it
says where it would go, in the status line. The whole promise of this workbench
is that nothing leaves this tab, and navigating the tab to somebody else's URL
is the one gesture that would break it — silently, and in the one direction a
reader cannot undo.

⛔ **The list is bounded and says so.** A document chooses how many links it
carries, and reading the words under each one means reading the text of the page
it is on; both are the file's to make enormous. Past a thousand rows the last
one reads *"… and N more links, not shown"*, because a list that silently stops
lies about what the document holds.

A page list is still there, as the **last** resort, for a document that carries
neither an outline nor a link.

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

⛔ **And they come back by themselves, which for a release they did not.**
Ending the drag throws the pictures away so they can be made again at the new
width — and *again* never happened: only `refresh` rebuilds the rail, and
widening it is not a verb, so nothing called one. The reader was left with the
pictures made for the old width, stretched, and the workbench answered that it
was holding no rail at all. `fillRail` now puts it back on the next frame. The
in-process test did not see it because the test calls `s.refresh()` itself, on
the line after the one that drops them; the browser check found it on the first
run that could perform a drag.

⛔ **And a frame in the middle of a gesture does not cancel it.** Putting the
band back together builds a **new** `Splitter`, and the one holding the drag
goes with the old one: the pointer keeps moving and nothing is listening.
Measured in the browser — a 60-pixel drag moved the divider 7 pixels, one
frame's worth, and stopped. Nothing rebuilds the band while
`Splitter.Dragging()` is set. It was invisible in process because a test sends
press, move and release back to back and the running workbench gets a frame
between every pair of pointer events; there is now a test that puts one there.

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

## Selecting the words on a page

Drag across a page to select its text; **Copy the words** on the Read tab puts
it on the clipboard, and Escape lets go. A press anywhere else — the scrollbar,
the rail, the strip, a form being filled in — keeps the press it always had.

⛔ **The page is a picture, which is why this needs a layer of its own.**
`PagedView` says so: *"Content is a `[]*image.RGBA` of page bitmaps at natural
size … PagedView never knows how they were rasterised."* Nothing in the widget
tree is text, so without this there is nothing under the pointer to select. The
glyphs come from [go-pdfkit/extract](https://github.com/go-pdfkit/extract),
placed where they are **drawn** — the placement comes from
`PagedView.PageRect`, the same layout the pages are rendered from, so the text
cannot drift from the picture it sits on through a scroll, a zoom, a page flip
or a resize.

⛔ **And rendering the pages as SVG later would not remove the need for it.**
Worth writing down, because the go-tex playground deleted its own text layer
when it moved from bitmaps to SVG. It could: it typesets the document itself,
so its SVG carries real `<text>`. A PDF renderer cannot —
[go-pdfkit/render](https://github.com/go-pdfkit/render) turns every glyph into
an **outline** before it paints, so an SVG written from it holds `<path>`
elements and no text at all. Crisp at any zoom, still not selectable.

**One run per word, not per glyph.** A PDF sets glyphs one at a time — a real
page measured here has 152 runs for four lines — and the toolkit joins the runs
a selection covers *with a space*. Handed the glyphs, a reader who selected a
line and copied it got `t h e f i r s t c h a p t e r`. Grouped into words, the
same gesture gives `the first chapter`.

A PDF never draws a space: it moves the pen. So where a word ends is a
judgement, and `Run.Space` — which is supposed to be the width of a space —
comes back as the font **size** on real documents
([extract#28](https://github.com/go-pdfkit/extract/issues/28)). The threshold
was half of that: 5 points at 10pt text, where a real word gap is 3.2 and a gap
between two letters 0.15, so every gap failed and no space was ever found. A
space that cannot be one is now distrusted, and a face that reports a
believable number is still believed.

**A run is measured against its own box.** Everything the selection does inside
a run — where the caret lands, how wide a covered run is lit, where a mark gets
written — is measured with the run's `Font`, while the run's *box* comes from
where the glyphs sit on the page. Those were two different metrics: each run was
handed the interface font, which measures `chapter` wider than the page's glyphs
do, so the caret at the right-hand edge of a box landed at the sixth character
and the seventh **could not be reached by any gesture**. A reader who selected a
line and pressed Copy got `the first chapte`. A run is one word at one size, so
it now measures *n* characters as *n*/length of its own box, which makes the
whole text measure the whole box by construction.

## Marking a passage

With words selected, the Read tab offers **Highlight**, **Underline** and
**Strike out**. Each says how much it marked — *"highlighted 17 characters on 1
page"* — and lets the selection go, because a mark lands in the file rather than
on the screen and a verb that worked in silence is one you press twice.

⛔ **The mark goes where the highlight is, and that is the whole design.** The
rectangles come from `TextSelection.SelectedRects`, which is *what `Draw`
paints* — `Draw` is a loop over it
([toolkit#540](https://github.com/go-widgets/toolkit/pull/540)). Re-deriving the
covered span from the runs and the caret positions would be a second
implementation of the same rule, and the first time the two disagreed the saved
file would be wrong while the screen looked right. `placeRun` puts a run onto
the screen and `quadOfRect` puts a lit rectangle back into page space; they are
inverses, checked at three scales, exact to the pixel `placeRun` rounds to.

⛔ **And a mark is an annotation, not ink.** ISO 32000-1 §12.5.6.10: a
text-markup annotation carries `QuadPoints`, *"an array of 8 × n numbers
specifying the coordinates of n quadrilaterals in default user space"*. The
page's content stream is untouched, so the words under a highlight stay
selectable, searchable and extractable, and the mark can be deleted again.
There is one quadrilateral per run, so the mark has the same shape as the lit
rectangles rather than one box drawn around them all — and one annotation per
page, because `QuadPoints` are in a page's own space and no annotation can span
two of them.

The count is **characters, not quadrilaterals**: a quadrilateral is not
something a reader can see, and there is one per run, whose size is the
producer's choice. The page measured here sets one glyph at a time, so `the
first chapter` is three runs — the first draft said *"highlighted 3 lines"* for
one line of text, and a page drawing that line with a single `Tj` would have
said *"1 line"* for the same gesture.

## Redacting a passage

**Redact**, on the same tab, takes the selected words **out of the file**. It is
the only verb on the Read tab that destroys anything, and the only one a saved
document cannot be talked out of.

⛔ **It is not a black rectangle.** That is the standard data leak: the text
stays in the page's content stream, so anybody can copy it out from under the
box, and `pdfops text` prints it. So this removes the operator that drew the
text, and the status line says which it did —
*"removed 17 characters from 1 page — the text is gone from the file, not
covered"*.

It sits next to the selection rather than on Protect because a selection is made
two controls away: a reader drags across a passage and the verb has to be where
their eye already is. A redaction they had to go and find on another tab is one
they would do with a rectangle instead, which is the thing this exists to stop.

⛔ **And it refuses, out loud, where it cannot remove the words completely.**
[`ops.Doc.Redact`](https://github.com/go-pdfkit/ops) will not take out text that
is inside a form, across an image, under an annotation, or drawn by an operator
that also draws words outside the selection — and its message goes to the status
line unchanged, because that message is the useful part. A reader told *"this
line is drawn in one piece, and taking it would take more than you asked for"*
can select the whole line instead. A workbench that swallowed the refusal and
drew a box would be the leak.

The commonest refusal is the one a reader walks into: **highlight a passage,
then decide to redact it.** The highlight is an annotation across the region and
its own note can hold the very words, so the library says no. Drop the
highlight first.

Where the words go is decided **once**, by the same `selectionQuads` the marking
verbs use. Marking and redacting are opposite acts — one is written over the
page and comes off again, the other takes the text out of the file — but they are
the same question about *where*, and a reader who highlights a line and then
redacts it has to get the same words both times.

## The panel could write and could not read

Giving a document a title and an author has been on the File tab since the
beginning. **Showing the title it already has was never there.** So a reader
who wanted to know what a file claims about itself had to open it in something
else — and anybody typing into those two boxes was overwriting a value the
workbench had never shown them.

**Show what it says** puts that where the page was: every entry of the
information dictionary, how big each page is, and whether the file's structure
had to be rebuilt to read it at all.

Three details it has to get right:

- it lists the entries it does **not** know. A producer may put anything in
  there, and a readout that printed only the nine entries of ISO 32000-1
  Table 317 would quietly hide the rest, which is the opposite of what a
  readout is for. The nine come first, in the order the standard lists them;
  anything else follows, sorted, so two openings of one file read the same.
- the page size is the one the page is **shown** at — the crop box over the
  media box, because a page cropped to half its paper is half its paper to
  everybody looking at it, and a readout quoting the paper would disagree with
  the picture beside it. It is read from the document written and read back, so
  it follows a crop you have not saved yet.
- a sound file **says** it is sound. *"This file was repaired"* is the useful
  message, but its absence must not be silence: a reader cannot tell a file
  that was fine from a readout that forgot to look. And a damaged one says what
  to do about it — Saving writes the rebuilt document — because repair is
  something a PDF reader does on the way *in*, so a button offering to do it
  would be offering something that already happened.

### And then it could read and could only half write

The File panel showed the file's **Subject**, **Keywords**, **Creator** and
**Producer** as labels while *Say so in the file* wrote the title and the author
— four entries of the document's own information dictionary sitting there
read-only for no reason but that nothing had been wired to them. They are boxes
now, and the verb writes all six.

- **the boxes are filled from the UNCUT value.** The panel clips what it shows
  to 120 runes and an ellipsis, which is right for a label and destructive for a
  box that gets written back: a long title would have come back as the ellipsis
  the panel drew. Showing and editing are two readings of the same entry, and
  only one of them may be abridged.
- **an entry longer than that is shown and not offered**, under a caption saying
  so, and its value is still loaded — so writing the other boxes puts it back
  exactly as it came. A file may carry a subject a megabyte long; no box can
  usefully hold one, and silently cutting it down by way of displaying it is the
  same bug one step along.
- **the two dates are shown as the file wrote them.** A PDF date is
  `D:YYYYMMDDHHmmSSOHH'mm` (ISO 32000-1, 7.9.4) and nothing in this fleet parses
  one, so a box to type a date into is a box in which a reader writes something
  no PDF reader will show. *Drop what it says about itself* is how they go.

## Putting the pages in another order by looking at them

`Move it there` has moved a page since the Pages tab existed — by its **number**,
typed into a box, which is the one thing nobody has in mind when they want a page
somewhere else. What they have in mind is the picture of it.

**Arrange the pages** puts every page where the page was, as a grid of tiles.
Press one to pick it up, press the page it should go in front of, and it goes
there; press it again to put it back. The tile that is lit is the one being
**held**, not the one being read — in the rail the mark means *you are here*, and
here it means *this is the one you picked up*, because that is the question this
view answers and a mark meaning both would answer neither.

- **two presses, not a drag.** A drag over a grid inside a scroll view has to
  hit-test tiles against a moving viewport and decide what a press that moved
  four pixels meant. Two presses say the same thing — this page, then where it
  goes — and a reader can see what is held between them, because the tile says so
  and the status line says so. It is also the gesture a touch screen gives for
  nothing.
- **in front of the page pressed, which is not the same as "to its number".**
  `ops.Move` takes the page out *first*, so moving page 2 to position 5 of six
  leaves it **after** the page that was fifth — and a reader who pressed the fifth
  tile meant *before this one*. The number asked for is one less when the page
  moves forwards, and the status line says where it landed rather than where it
  was asked to go.
- **the pictures are the rail's own**, at the rail's width and already drawn.
  Rendering a second set at another size would double what a forty-page document
  costs to look at, and the tiles further out are a frame and a number either way
  — which, for putting pages in order, is most of what a tile is for.
- **leaving the grid lets go of what was held.** A page picked up in a view that
  is no longer on the screen is a move nobody can finish.

## Writing the table of contents

The rail has shown a document's bookmarks since the rail existed, and for twelve
versions the only thing the ribbon could do to them was **Drop the bookmarks**.
The **Contents** tab answers the other half.

- **Bookmark this page** adds one entry to the tree the document already
  carries, pointing at the page on the screen. Select a passage first and the
  entry is titled with those words and points at the page *they* are on —
  not at the page being looked at, because a reader can select a heading on page
  three and scroll to page two before pressing the button, and a bookmark that
  says one thing and goes somewhere else is worse than no bookmark.
- **Bookmarks from a file** replaces the tree with one written as text: a title,
  a **tab** and a page number on each line, two spaces of indent for each level
  under another.

      Introduction<TAB>1
        Why<TAB>2
      Chapter one<TAB>7

  It is the same file `pdfops outline -from toc.txt` takes, and the same parser:
  `ops.ParseOutline` was lifted out of `cmd/pdfops` into the library so that the
  command and the workbench accept one grammar by construction rather than by
  care. A line it cannot read is a refusal **naming the line**, handed on
  unchanged — a table of contents quietly missing an entry is worse than one
  that refuses to be written.

- **Write a contents page** is the other kind of table of contents: the
  **printed** one. A page at the front with a line for each bookmark, its page
  number at the right and dots leading to it, built from the entries the rail is
  already showing. The numbers account for the page it adds — an entry that
  pointed at page 7 is on page 8 once a contents page is in front of it, and
  printing the 7 would send a reader one page short of every heading in the
  book. It does **not** rewrite the bookmarks: `ops` carries a document's own
  outline over and remaps each entry to the page object it still points at, so
  the tree follows its pages across the insertion, where writing it back from
  this program's own walk would lose the entries that point nowhere and anything
  past the thousand it reads.

⛔ **Adding one bookmark means handing over the whole tree.** `SetOutline`
writes what it is given *instead of* the sources' own, so the existing
bookmarks are read back out of the document and nested again with the new one
among them. A verb that passed only the new entry would have read as *bookmark
this page* and acted as *replace the table of contents with this page*.

⛔⛔ **And a bookmark that points nowhere takes its children with it.** The rail
keeps such a row, so that the shape of a document's contents is not a lie about
what the file holds; a tree being *written* cannot keep it, because there is no
page to write. Dropping the row and keeping what hung off it was measured: a
sub-section under a heading that pointed nowhere came out under the *previous*
heading instead — depth 1 after a kept row at depth 0 is a rung
`ops.Nest` has no reason to question — so the entry changed chapter silently.
Deeper down the same fault is refused outright and the whole outline is written
as nothing. Either way the status line would have said a bookmark was added.

## A document can carry other files

And the workbench never said so, so a PDF arriving with a spreadsheet inside it
looked like a PDF with nothing inside it. `ops` has had `Attach`, `Attachments`
and `Detach` throughout.

**Put a file inside it** and **Show the files inside** are on the File tab; the
list gives each one a **Take it out** and a **Drop it**.

- the list is the **document's own**, not the saved file's — `s.doc` is what
  would come out of Save, so a file attached a moment ago is in it and one just
  dropped is gone from it. Reading it out of the written bytes would show the
  list one refresh late, which for somebody attaching two files in a row means
  showing them the wrong one.
- a **name already used is refused**, not replaced: *"two files under one name
  is a document that has lost one of them"*, which is `ops`'s own wording and
  reaches the status line unchanged.
- **dropping a name the document never carried says so.** `Detach` reports
  whether it found anything, and a verb that said "dropped" either way would be
  telling somebody their file is gone when it was never there.

**Mix another file in**, on the Sheet tab, takes one page from each in turn —
which is how two single-sided scans of the same document become one. It is
three lines over the seam `merge` and `overlay` already asked the file picker
through.

## The grids a page lines its words up into

**What lines up into a grid** shows them where the page was; **Hand over the
grids** writes them as CSV, and **Hand over the page as data** writes every
piece of text with where it sits, plus the grids, as JSON.

⛔ **A PDF page holds no tables.** It holds text put at places, and a table is
what a reader sees when those places line up — so these are an inference, and
the labels say so rather than promising something the file cannot declare.

Two things found while adding them:

- ⛔ **Anchoring on the repository a verdict cites nearly cost 33.5 MB.** These
  three rows were recorded against `go-pdfkit/document`, and that package is
  about 33.5 MB in a wasm binary *in either direction*, because it imports
  `html2pdf` and so the browser engine behind it — 3.50 MB for `ooxml`'s writer
  alone, 37.05 MB once `document.Parse` is so much as named.
  [`extract.Tables`](https://github.com/go-pdfkit/extract) answers the same
  question and has been linked since the Read tab could read text, so the three
  rows cost nothing at all.
- ⛔ **A blank line between two grids is invisible.** It is the obvious
  separator and `encoding/csv` *skips blank lines when reading*, so a file
  written that way comes back as one grid with the next one's rows stuck to it.
  `pdfops tables -as csv` does exactly that, with a comment saying "so a reader
  can see where each ended"
  ([ops#71](https://github.com/go-pdfkit/ops/issues/71)). The test here found
  it by reading its own output back with a CSV reader instead of comparing
  bytes. RFC 4180 has no table separator, so this writes a one-cell marker row
  naming the grid — visible to every reader, including a spreadsheet — and
  writes none at all when there is only one grid.

## What Open accepts that is not a PDF

Somebody who hands a file to a PDF workbench wants a PDF of it. A **picture**
has opened as a page since the workbench could open anything; a **drawing**
(SVG) and a **comic archive** (`.cbz`, or any zip of pictures) now do too, over
[`convert`](https://github.com/go-pdfkit/convert).

Each arm says *which* it was, because the three keep different amounts of the
original:

- a drawing goes on the page **as paths**, so it stays sharp at any size — and
  when it uses a gradient, a clip path or an embedded picture, which cannot
  become path operators, the page is **drawn** instead and the status line
  warns. ⛔ The code asks `SVGToPDFVector` first even though `SVGToPDF` falls
  back by itself: the fallback is right, but `convert` cannot tell the person
  standing at the workbench which of the two they just got, and that difference
  is everything they can do with the result.
- an archive becomes its pages **in the order their names put them** —
  `page10` after `page2`, which a byte-wise sort gets wrong and which is the
  one defect every naive reader of the format has.
- the archive arm is **last**, because a `.docx` and an `.odt` are zips too. What
  reaches the status line for one of those is `convert`'s own sentence — *"no
  pictures in the archive: 1 entry held nothing readable"* — rather than a guess
  at what was meant.

The ceilings are `convert`'s (`MaxArchiveEntries`, `MaxArchiveEntryBytes`,
`MaxArchiveBytes`), and they are *variables* so a test can reach them. A test
here lowers each one and checks the refusal arrives naming the bound. ⛔ Two of
the three print the same sentence, because the room left is the smaller of the
per-entry ceiling and what remains of the whole — so the cumulative case is told
apart by *which entry* it refuses, never the first. Asserting only the number
would have passed on the wrong bound; a control marked the per-entry case the
same way and failed, which is how the other is known to measure itself.

### And the second file is whatever the first one may be

**Add a file after this one**, **Lay another file over these** and **Mix another
file in** go through the same dialog, so the moment Open accepted a picture, a
drawing and an archive, those three started offering them — and refused them.
That is the *other* direction of the same defect: a file the dialog shows and
the code will not take is a file somebody chose for nothing.

They take all of it now, which is also the useful behaviour. Putting a scan
after a document, laying a signature over a page and interleaving a
photographed back page are the three things anybody would try first.

### ⛔⛔ The file dialog is half of every one of these arms

`web/index.html` carried `accept="application/pdf,.pdf"`.

So the picture arm — there from the beginning, tested, counted as delivered —
was behind a dialog that **would not show a PNG**. The code was right. Every
test passed, because a test hands bytes straight to `open`. Six rows of the
BentoPDF reading were ticked off on a path a person could not take.

A filter list is the picker's half of a verb and has to change in the same
breath as the arm it admits. So the list lives in Go now, as `openSuffixes`, and
a test reads the HTML and compares the two **in both directions**: a suffix the
code accepts and the dialog hides is unreachable, and a suffix the dialog offers
and the code refuses is a file somebody chose for nothing. Run against this
morning's HTML the gate names all **21** shapes that were hidden.

It is also wider than the rows that asked for it. `codec` reads ICO, ICNS, PNM,
QOI, JP2 and JBIG2 as well, `ops.Picture` makes a page of any of them, and a
dialog that hides a file the program would have opened is the same defect at a
smaller size.

## Drawing on a passage

**Box them**, **Ring them** and **Point at them** are on the Mark up tab, over
`ops.Doc.AddShapes` (new in ops v0.31.0): `/Square`, `/Circle` and `/Line` with
Table 176's `ClosedArrow`. They are annotations like the marks, so the page
underneath is untouched and each comes off again by dropping it.

⛔ **Each goes round the passage, never over it.** A box drawn on the glyphs
hides exactly what it draws attention to — the same failure the sticky note had
— so the rectangle is grown beyond the words and clamped to the paper. An arrow
comes in from whichever margin has room and **stops short of the glyphs**: a line
drawn *through* a passage would be a strike-out, and there is a verb for that two
controls along.

⛔ A margin too narrow for an arrow is **refused with the number it needs**, and
points at the box instead, which needs no margin.

### ⛔⛔ And the gate that found two tabs already broken

Adding the three shapes made a seventh group on the Read tab, and seven of that
tab's controls came out **28 pixels wide with no words on them** — the readings,
the search and Copy, reduced to bare icons. Nothing was *dropped*, so
`TestEveryVerbOnEveryTabIsReachable` passed: it measured reachability, which is
not legibility.

The gate now measures **width** too, and the first run of it found the **File
tab already in that state** — six controls shrunk by the redraw group added in
v0.32.0 and shipped without anyone looking. Two splits followed:

- **Mark up** is its own tab: the marks, the shapes, the comments and the two
  verbs that alter the text, with the boxes they read. Read keeps what answers
  *"what does this page say"* — read it, search it, copy it — and its panel can
  promise again that nothing on it changes the document, which was **false for
  several releases** while Redact sat there.
- **Redraw** is its own tab, with the ground chooser and the two spinners that
  belong to it.

A tab that trips the gate is a tab to split; the ribbon is doing the right thing
with the room it was given. What was missing was anybody measuring it.

## Replacing the words a page says

**Replace the words**, beside Redact on the Read tab, puts the typed text in
place of the selected words — over
[`ops.Doc.ReplaceText`](https://github.com/go-pdfkit/ops), new in ops v0.30.0.

⛔⛔ **The hard part is not the string, it is everything after it.** A page sets
glyphs by advancing a pen, so a replacement of a different width silently moves
the rest of the line. The spec supplies the remedy: a number in a `TJ` array is
*"expressed in thousandths of a unit of text space"* and is **subtracted** from
the horizontal coordinate (9.4.3), and 9.4.4's displacement formula multiplies
*both* the glyph width and that number by the font size — so the kern needed is
exactly the difference of the two raw width sums, **with no font size in it at
all**. The library asserts that at 6, 12, 24 and 48pt, with a control showing
the next word moving 18 points when the kern is left out; the workbench asserts
that the word after the edit starts at the same x it started at before.

⛔ **It replaces one piece of text as the page drew it**, which is not the same
as "the words you selected". A PDF draws text in operations and a page is free
to make each one a line, a word or a single glyph — TeX sets type glyph by
glyph. So a selection covering several operations is **refused with the count**
and a way forward, because on such a page a bare "cannot" would read as a broken
verb rather than as a page shaped the wrong way for this:

> this page draws those words in 15 separate operations, and they can only be
> replaced one at a time: select one piece of the text, or redact the passage
> and write the replacement in the margin

⛔ `ops` refuses the rest **by name** and the refusal reaches the status line: a
run inside a form (shared with every page that draws it), a composite font
(whose byte codes a CMap defines and nothing can invert), a character the page's
own font cannot draw, and a replacement that changes the number of spaces (word
spacing adds a step per space that cannot be read back from the stream).

Two things found writing the tests:

- ⛔ **The selection layer merges adjacent operations.** `(Hello) Tj (World) Tj`
  leaves no gap — the pen runs straight on — so the screen offers one word,
  `HelloWorld`, covering both operations. A test that selected it was refused
  for naming two operations, correctly, and never reached the arithmetic. What a
  reader does there is drag across *part* of the line, so the test maps page
  coordinates to the screen the way `placeRun` does.
- ⛔ **Text with no span is refused rather than skipped**, as `ops`'s redaction
  does. Taking the runs away entirely does not reach that: the selection comes
  from the same runs, so a page with none has nothing selected and says so. What
  reaches it is a run whose `Op` is empty, which is the state `extract`
  documents for an operation it could not locate.

## Saying something about a passage

**Put a note on it** attaches a sticky note; **Write in the margin** writes a
comment beside the passage, where a reader would pencil one. Both are on the
Read tab, both act on the selection, and both are real annotations — `/Text` and
`/FreeText`, new in [`ops` v0.29.0](https://github.com/go-pdfkit/ops) — so the
page underneath is untouched and the comment comes off again by dropping it.

⛔ **Both act on the selection**, like the four marks and Redact, and that is a
decision about the *gesture* rather than about the format. A note is attached to
a point and a comment to a box, so either could have been placed by pointing —
but the workbench's way of pointing at words is already a drag, and a second
placing gesture would be a second thing to learn and a second set of arithmetic
to keep in step with the first.

⛔ **A note goes beside the passage, never on it.** The icon is twenty points
square and opaque: dropped on the words it comments on, it hides exactly them.
So the left margin is tried, then the right, then above the line — and when none
of the three is clear, the verb **refuses and says so**, pointing at the margin
comment, which needs no icon.

The first version of that clamped instead of choosing, and the test caught it
doing precisely what the verb exists to prevent: a passage starting 19.7 points
from the edge got a 20-point icon drawn from x=8 to x=28, straight across the
words. **A clamp keeps a number in range; it does not keep a promise.** Each
candidate is now checked against the passage and against the paper, and one that
fails either is not used.

⛔ **A margin too narrow to read is refused**, with its width, rather than filled
with one word per line. `ops` clips whatever does not fit its box, which is
right for a box somebody chose and wrong for one this verb chose for them — and
the reader cannot widen it from here.

Two things the tests had to be talked out of:

- the note test first demanded the **left** margin and failed on a correct
  placement. The promise is not "the icon is on the left", it is "the icon is
  not on the words" — an assertion about the implementation instead of the
  behaviour.
- the write-failure test first made `docBytes` fail and expected *"cannot be
  written"*. Both verbs reported success, and they were **right**:
  `changeSaying` applies the operation to the document in memory, and a failed
  write-back does not unmake the comment — it spoils the preview, and Save is
  what reports it. What the test pins down now is that the comment is in the
  document even when the preview could not be rebuilt.

## Finding words, and why a match is a selection

**Find** and **Find the next one** are on the Read tab, with the box beside
them. A match turns to its page and **lights up exactly as though you had
dragged across it**.

⛔ That is the whole design, not a flourish. The workbench already knows how to
light a stretch of a page's text, copy it, mark it four ways and redact it. A
search that drew its own highlight would be a *second* answer to where words
are — a second set of rounding decisions against a second layout, which is the
one way a mark lands somewhere other than where the reader saw it. So this finds
the words, turns the page and drives the **same selection** a drag would have
built. Copy, Highlight, Underline, Strike out, Squiggle and Redact then work on
what was found without knowing a search happened.

⛔ **And it searches the way the selection reads.** A PDF never draws a space; it
moves the pen, and the word grouping is where gaps become boundaries. The
toolkit's `SelectedText` joins the words of a line with one space — so a phrase
is looked for in exactly that text, by the same `needsSpace` rule the selection
uses. Anything else would find phrases a reader cannot then select, or miss ones
they can see.

⛔ **A phrase never spans a line break.** Different lines are joined with a
newline, not a space, and one selection covers one stretch of one line — so a
match reported across a break could not be lit, copied or marked as what it
claims to be. The test asserts the refusal *and* the control: the same two words
on one line are found.

`Find the next one` re-searches when the box has changed since the last Find,
because the box is a live binding and stepping through the matches of a word
nobody is looking for any more is what makes a search feel broken.

## ⛔⛔ A group named like its own tab disappears

Adding the search pushed the Read tab to eighteen controls, and a test failed
naming **`What this page says`** — a verb nobody had touched.

The cause was not width. The Read tab's first group was itself called *Read*, and
`Ribbon.ControlRect` is keyed by label, so that group could not be told from its
own tab's button. While the strip had room this was harmless; once it was
crowded enough to compact, all four of that group's controls reported **no
rectangle at all**. Renaming the group to *Readings* put them back at full size
with nothing else changed — measured twice, in both directions.

Two things came out of it, and the second is the uncomfortable one:

- the hand-over verbs are their own tab now. Six verbs that all answer *"give me
  a copy"* are a tab, and the picture-format chooser that sat on the Read panel
  for want of anywhere else is exactly what that tab is told. The ribbon's answer
  to a crowded strip is to shrink a group to **28-pixel icon buttons with no
  words on them** — reachable, unreadable — and splitting puts everything on both
  tabs back at full size.
- **the reachability gate passed through this.** It excused a group whenever
  `ControlRect(group.label)` answered, on the reasoning that a folded group is
  drawn as one button bearing its name. The tab's own button answered. Narrowing
  it to "a label the tab does not share" was not enough either: the next
  arrangement had a *group* called "Hand over" on the Read tab and a *tab* called
  "Hand over" beside it, and that tab's button answered just as happily. The
  excuse is gone — every verb on every tab must now be drawn — and the
  ambiguity is filed as
  [toolkit#543](https://github.com/go-widgets/toolkit/issues/543), because an
  API keyed by label cannot tell a fold from a coincidence.

## ⛔⛔ And a panel has its own limit, which is not the strip's

A header and a footer need two boxes to type them into, and adding those to the
Marks panel took it **109 pixels past the room it is given**. The strip was
fine; the gate that caught it was a different one —
`TestNoPanelOverflowsTheRoomItIsGiven`, which refuses a panel whose last rows
can only be *scrolled to*, on the reasoning that a control a reader has to
discover by scrolling is one they do not know is there.

The Marks panel was **21 pixels** from that limit before the two boxes went in:
the next addition of any kind would have broken it. So the split is not about
the header and footer — they are what measured it.

The line was in the groups, as it was every other time. **Marks** writes the
same thing across every page by a rule: a watermark, a page number, a running
serial. **Stamp** puts something where you *choose*, which is the question the
*Where it goes* chooser asks — and a picture asks it too, so the picture moved
with it. The range box is shown on **both** panels, bound to one datum: a reader
who typed `1-3` before stamping must not find it gone after choosing the other
tab, and that is the opposite of the Pages panel's rule for the opposite reason
— *which pages to keep* and *which pages to mark* are two questions, so they are
two values.

## Counting the library's own list against the ribbon's

Two more verbs that were finished and unreachable, found by a different method
from the tool-table reading: comparing an enum and a signature in `ops` against
what the ribbon offers.

- **Squiggle under** — `ops` has had **four** markup kinds since `Mark` existed
  (`MarkHighlight`, `MarkUnderline`, `MarkStrikeOut`, `MarkSquiggly`) and the
  ribbon offered three. The wavy line that means *look at this again* was
  written, tested and released, and nobody using the workbench could draw one.
  Each of the four now asserts **its own `/Subtype`** in the saved file: a test
  that only checked for `/QuadPoints` passed for all four however the kind was
  wired, so a squiggle written as a highlight would have looked like a working
  verb.
- **Draw a picture on them** — `ops.Place` was never called by this program at
  all, though its own documentation says what it is for: *"a signature, a logo,
  a stamp that is an image rather than a line of text"*. It is the *add images*
  half of the one row the BentoPDF reading still calls `partly`.

⛔ The picture is sized **by width**, not by its own pixels. `Place` falls back
to one point per pixel, which is right where the picture *is* the page and wrong
for a logo — a 1000-pixel badge would go down a metre wide. A width alone keeps
the proportions, which is `Place`'s own promise, and the default is 144 points:
two inches, a signature on a letter. The test measures the **rendered pixels**:
144 wide and 72 tall for a 100×50 picture, with a second case at another width
so that a hard-coded number anywhere in the chain fails.

## Redrawing the pages is a trade, and the workbench says so

**Turn them inside out**, **Drop the colour**, **Draw them on a colour**,
**Brighten or deepen them** and **Make them look scanned** are the *Redraw the
pixels* group on the File tab, over
[`convert.Redraw`](https://github.com/go-pdfkit/convert). **Hand it over as a
comic** is beside them in spirit and on the Read tab in fact, over
`convert.ToArchive`.

⛔ **Every one of them replaces each page with a picture of it**, and every tip
and every status line says so: *"⚠ the pages are pictures now: their text cannot
be selected, searched or read out any more"*. Turning light into dark is a pixel
operation and there is no other way to do it — but a reader who inverts a
contract to read it at night and then presses Save has quietly thrown away
everything that made it a document. That warning is on the control, not in the
release notes.

- ⛔ **433 KiB, not 18.18 MB — and the delivered binary grew 1.07 MiB.**
  `convert` measures about 18.18 MB on its own, which is roughly what the
  browser engine costs and would have ruled this group out. Built as an
  *increment* into the binary that was already there, by a control that names
  `Redraw` and `ToArchive` and nothing else, it is **443,265 bytes**:
  22,489,931 → 22,933,196. A package's standalone size is not its price in a
  binary that already paid for its dependencies, and measuring the wrong one of
  the two is how six finished verbs stay off a ribbon. (`go-pdfkit/document` is
  the other answer: **26.9 MB** as an increment, which is why the rows behind it
  wait on [document#11](https://github.com/go-pdfkit/document/issues/11).)

  ⛔ But the whole batch is **+1,126,671 bytes** (22,489,931 → 23,616,602), so
  **683,406 of them are not `convert`** and are not a dependency bump either —
  the control's `go.mod` and this branch's are byte-identical. 134 lines of Go
  do not weigh 667 KiB, so something else here reaches code the control does
  not — filed as [app#85](https://github.com/go-pdfkit/app/issues/85) rather
  than rounded away, because this batch was *gated* on an increment measurement
  and a control that can under-report by 2.5× puts every decision taken on one
  of those numbers in doubt, including the ones that said no. The figure to
  count is the **1.07 MiB**, not the 433 KiB the probe promised.
- ⛔ **The ground colour goes to the renderer, not under the raster.**
  `convert`'s own note records the version that painted it afterwards and *"does
  nothing at all, because the renderer has already filled the page with opaque
  white"* — documented, shipped, and with no effect.
- ⛔ **The scanner uses a fixed seed**, which is `convert`'s default and its
  reason: *"two runs over the same file must produce the same bytes"*. The test
  that holds it compares **rendered pixels**, because `doc.Bytes()` carries the
  write's own date and so differs between two runs of the same verb — a
  reproducibility test that compared files would have failed for the one reason
  that is not about reproducibility.
- ⛔ **The two spinners are percentages.** Brightness −100…+100 maps to
  `convert`'s −1…+1, and contrast **100 means leave it alone**, because
  `convert`'s neutral is 1 and a 0 there *"would turn every page into a flat
  grey rectangle"* — so a zero never reaches it.
- ⛔ **A `.cbz` and not a `.zip`.** The zip beside it holds the same one picture
  per page; a comic reader opens the one named for the format, which is the
  whole difference between a file somebody can use and one they have to rename.

## A verb with no control is not delivered

Four of the verbs above — cut each page up, put them on one page, print it
larger than the paper, straighten the scan — were **already built** when they
were added here. `ops` carries `Divide`, `OnePage`, `Poster` and `Deskew`, the
workbench has always linked `ops`, and not one of them was on the ribbon. The
fleet could do them; nobody using the workbench could.

That is not an accident, it is a *measured* condition. Judged against
BentoPDF's own tool tables, **19 of its 111 tools are still in exactly that
state** here — against 19 this fleet genuinely cannot do, and 61 it has. The
figure was 40 when this section was written; most of the fall since is this
reading repeated, not new code. So the cheapest equivalence available is not
writing anything: it is putting a control on what is already finished.

Four more followed the same way — **set the paper**, **paint the text**, and
before them the two readings above. Each of these also says the thing the
library knows and a button would swallow:

- the paper is the **sheet**, not the crop box: cropping cuts a page down to
  part of itself, this says which sheet it is on, and a page cropped to a
  postcard on A4 is still on A4. The sizes are the standard's — A4 is
  **595.276 × 841.89** points, because rounding it to 595 × 842 is a tenth of
  a millimetre that shows as a hairline against a trimmed edge.
- painting the text says **what it could not reach**. Text drawn through a Form
  XObject is shared with every page that draws it, so recolouring one would
  change pages nobody named; the library leaves those alone and counts them,
  and the status line passes the count on. A document that came back looking
  unchanged with nothing said would be worse than a refusal.

Two things the first batch had to get right, and both are the kind of thing a
ribbon gets wrong quietly:

- ⛔ **Cutting and tiling keep their own numbers.** `Divide` cuts one page into
  smaller pages of its own content; `Poster` spreads one page over several
  sheets to be printed larger and taped together. Both take "across and down",
  and *2 × 2* means halves in the first and quarters-enlarged in the second —
  so one pair of spinners shared between them would show the wrong thing in
  whichever was not last used.
- ⛔ **Straightening says what it measured**, not that it ran: *"measured 12
  pages, straightened 3"*. A page already level is left alone on purpose,
  because turning it by a hundredth of a degree costs it a transformation it
  did not need — so a verb that only said "done" would leave a reader unable to
  tell a level scan from a verb that did nothing. A page that cannot be
  measured at all is a third answer and is reported as one.

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

```sh
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
