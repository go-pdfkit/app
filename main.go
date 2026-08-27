// Command app is the workbench: a PDF opened, rearranged, stamped and saved
// entirely inside a browser tab. Nothing is uploaded, because there is nowhere
// to upload it to — the whole of it is this one wasm binary.
//
//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	"github.com/go-widgets/webcanvas"
)

func main() { webcanvas.Run("screen", newWorkbench(browser{})) }

// browser is the page this runs in: it opens the file picker and hands a
// finished document back through a download.
type browser struct{}

// Open shows the file picker and calls back with what was chosen. The picker
// is an <input type=file> the page already carries, so the click that reaches
// it is the one the person made — which is what browsers require.
func (browser) Open(done func(name string, data []byte)) {
	input := js.Global().Get("document").Call("getElementById", "file")
	if input.IsNull() || input.IsUndefined() {
		return
	}
	var onChange js.Func
	onChange = js.FuncOf(func(this js.Value, _ []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			return nil
		}
		file := files.Index(0)
		name := file.Get("name").String()
		read(file, func(data []byte) {
			// The same file may be chosen twice running; clearing the value
			// makes the second choice raise an event of its own.
			input.Set("value", "")
			done(name, data)
		})
		onChange.Release()
		return nil
	})
	input.Call("addEventListener", "change", onChange, map[string]any{"once": true})
	input.Call("click")
}

// read pulls the bytes out of a File, which the browser only hands over
// through a promise.
func read(file js.Value, done func([]byte)) {
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		buf := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, buf.Length())
		js.CopyBytesToGo(data, buf)
		done(data)
		return nil
	})
	file.Call("arrayBuffer").Call("then", then)
}

// Save hands a file to the person as a download. The blob is made from a copy
// of the bytes, since the Go side may write over its own buffer.
func (browser) Save(name string, data []byte) {
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	parts := js.Global().Get("Array").New(1)
	parts.SetIndex(0, buf)
	blob := js.Global().Get("Blob").New(parts, map[string]any{"type": mimeOf(name)})
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	link := js.Global().Get("document").Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", name)
	link.Call("click")
	js.Global().Get("URL").Call("revokeObjectURL", url)
}

// mimeOf is what a file handed back is, taken from what it is called.
//
// Not everything the workbench hands over is a document: a picture pulled off
// a page is a picture, and a browser told it is a PDF opens it in a PDF viewer
// that cannot read it.
func mimeOf(name string) string {
	switch {
	case strings.HasSuffix(name, ".jpg"):
		return "image/jpeg"
	case strings.HasSuffix(name, ".jp2"):
		return "image/jp2"
	case strings.HasSuffix(name, ".jbig2"):
		return "image/x-jbig2"
	case strings.HasSuffix(name, ".samples"):
		return "application/octet-stream"
	}
	return "application/pdf"
}
