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
  box, put a blank page in, straighten a crooked scan, set the paper to one of
  six sheets, paint the text one colour, split into files of *n* pages.
- **Sheet** — *n* pages to a sheet, fold into a booklet, cut each page up into
  a grid of smaller pages, put every page onto one, print a page larger than
  the paper across several sheets, add another file after this one, lay
  another file over it.
- **Marks** — a watermark, page numbers in a shape you choose, a Bates number
  padded and started where you say, and a stamp in one of nine places.
- **File** — sanitize, flatten the annotations in, drop the annotations, the
  bookmarks or what the file says about itself, pack it smaller, give it a
  title and an author, and **show** what it already says: every entry of its
  information dictionary, how big each page is, and whether its structure had
  to be rebuilt to read it.
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

## A verb with no control is not delivered

Four of the verbs above — cut each page up, put them on one page, print it
larger than the paper, straighten the scan — were **already built** when they
were added here. `ops` carries `Divide`, `OnePage`, `Poster` and `Deskew`, the
workbench has always linked `ops`, and not one of them was on the ribbon. The
fleet could do them; nobody using the workbench could.

That is not an accident, it is a *measured* condition. Judged against
BentoPDF's own tool tables, **40 of its 111 tools are in exactly that state**
here — more than the 18 this fleet genuinely cannot do. So the cheapest
equivalence available is not writing anything: it is putting a control on what
is already finished.

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
