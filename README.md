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

The service worker caches the shell and the binary on first visit, so the
second visit works with the network off — and so does the first one, once
loaded, since nothing is fetched after start-up. A file is read through
the browser's own picker and handed back as a download; the bytes never
touch a server, and there is no server to touch.

## What is on the strip

**Open** · **Save** · **&lt;** · **&gt;** · **Rotate** · **Delete** ·
**Two up** · **Watermark** · **Sanitize**. The arrow keys turn pages too.

Every change is applied to the document, written out, and read back before
it is drawn — so what is on the screen is what would come out of Save,
rather than a picture of what was meant to happen.

## How it is checked

The workbench is a plain Go type with no build tag, so a native test drives
the whole of it against a byte buffer: open a document built in the test,
press a control at a pixel on the strip, and look at what got drawn. Every
control is reached by pressing along the strip rather than by calling its
handler, which is what says the thing on the screen is wired to the thing
it says it is.

**100% of statements**, including every branch that reports a document
that cannot be written, read back or drawn — none of which should ever
happen, which is exactly why they are worth being able to see.

Then in a real browser, over the DevTools protocol with nothing eyeballed:
Chrome loads the shell, the wasm starts, the file picker is intercepted and
handed a PDF, and the canvas pixels are read back to prove the page arrived
— which is how it was found that a file arriving from the browser needs a
frame of its own, having no event to be drawn on.

## Building

```
GOOS=js GOARCH=wasm go build -o web/main.wasm .
```

then serve `web/` over HTTP. `web/wasm_exec.js` comes from the Go
distribution and must match the toolchain that built the binary.

## Licence

BSD-3-Clause.
