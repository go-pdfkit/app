// The browser half of the colour scheme: what the system says, and what the
// reader chose last time.

//go:build js && wasm

package main

import "syscall/js"

// themeKey is where the reader's choice is kept. Namespaced, because
// localStorage is shared by every page on the origin and this origin carries
// the playground and the landing as well.
const themeKey = "pdfkit.app.theme"

// darkQuery is the media query that answers the question. ⛔ There is no
// command-line flag and no document property that says "this machine is in dark
// mode": prefers-color-scheme is the seam, and matchMedia is how a page reads
// it. Asking the page's computed background instead would answer a different
// question — what the stylesheet painted — which is the reader's own override
// once there is one.
const darkQuery = "(prefers-color-scheme: dark)"

// SystemDark reports whether the page is being shown in a dark colour scheme.
// A browser too old for matchMedia reports light, which is this app's own
// default and so changes nothing for it.
func (browser) SystemDark() bool {
	q := darkMedia()
	if !q.Truthy() {
		return false
	}
	return q.Get("matches").Bool()
}

// OnSystemDarkChange calls back whenever the system's colour scheme changes
// while the tab is open — a laptop at sunset, or a reader flipping the setting
// to see what happens.
//
// ⛔ Both listener spellings. addEventListener on a MediaQueryList is the
// current one; addListener is the deprecated form that Safari carried alone for
// years. Registering only the first is a workbench that follows the system on
// some machines and silently does not on others, which is the hardest kind of
// defect to be told about.
func (browser) OnSystemDarkChange(f func(dark bool)) {
	q := darkMedia()
	if !q.Truthy() {
		return
	}
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		f(q.Get("matches").Bool())
		return nil
	})
	switch {
	case q.Get("addEventListener").Truthy():
		q.Call("addEventListener", "change", cb)
	case q.Get("addListener").Truthy():
		q.Call("addListener", cb)
	}
}

// darkMedia is the MediaQueryList, or a zero Value where matchMedia is absent.
func darkMedia() js.Value {
	if !js.Global().Get("matchMedia").Truthy() {
		return js.Value{}
	}
	return js.Global().Call("matchMedia", darkQuery)
}

// ThemeMode is the mode remembered from a previous visit, or "" when there is
// none to remember. Anything unreadable reads as "" and so as "system".
func (browser) ThemeMode() string {
	s := storage()
	if !s.Truthy() {
		return ""
	}
	v := s.Call("getItem", themeKey)
	if v.Type() != js.TypeString {
		return "" // null when unset, and whatever a wrong type would be
	}
	return v.String()
}

// StoreThemeMode remembers the reader's choice.
//
// ⛔ A failure here is swallowed on purpose. Writing to localStorage THROWS in
// a private window, with site data blocked, and when the origin's quota is
// full — and none of those is a reason to stop the workbench. The choice then
// lasts as long as the tab, which is the behaviour a reader in a private window
// is asking for anyway.
func (browser) StoreThemeMode(mode string) {
	s := storage()
	if !s.Truthy() {
		return
	}
	defer func() { _ = recover() }()
	s.Call("setItem", themeKey, mode)
}

// storage is window.localStorage, or a zero Value when the browser will not
// give it up — reading the property itself throws where site data is blocked.
func storage() (v js.Value) {
	defer func() {
		if recover() != nil {
			v = js.Value{}
		}
	}()
	return js.Global().Get("localStorage")
}
