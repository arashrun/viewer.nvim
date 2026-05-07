//go:build windows

package main

import webview2 "github.com/jchv/go-webview2"

type nativeWindow struct {
	view webview2.WebView
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
	n.view.SetSize(width, height, webview2.HintNone)
}

func (n nativeWindow) Terminate() {
	n.view.Terminate()
}

func attachNativeWindow(window *WindowController, w webview2.WebView, hub *Hub) error {
	return window.Attach(nativeWindow{view: w}, hub)
}
