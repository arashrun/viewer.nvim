//go:build linux || darwin

package main

import webview "github.com/webview/webview_go"

type nativeWindow struct {
	view webview.WebView
}

func (n nativeWindow) SetTitle(title string) {
	n.view.SetTitle(title)
}

func (n nativeWindow) SetHtml(html string) {
	n.view.SetHtml(html)
}

func (n nativeWindow) Eval(js string) {
	n.view.Eval(js)
}

func (n nativeWindow) Dispatch(fn func()) {
	n.view.Dispatch(fn)
}

func (n nativeWindow) Resize(width, height int) {
	n.view.SetSize(width, height, webview.HintNone)
}

func (n nativeWindow) SetBounds(bounds WindowBounds) {
	n.view.SetSize(bounds.Width, bounds.Height, webview.HintNone)
}

func (n nativeWindow) SetTopMost(topMost bool) {
	_ = topMost
}

func (n nativeWindow) Show() {
}

func (n nativeWindow) Hide() {
	// webview_go does not expose hide/show on non-Windows backends here.
}

func (n nativeWindow) Focus() {
}

func (n nativeWindow) CurrentBounds() (WindowBounds, bool) {
	return WindowBounds{}, false
}

func (n nativeWindow) Terminate() {
	n.view.Terminate()
}

func attachNativeWindow(window *WindowController, w webview.WebView, hub *Hub) error {
	return window.Attach(nativeWindow{view: w}, hub)
}
