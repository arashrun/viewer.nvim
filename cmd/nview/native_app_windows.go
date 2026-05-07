//go:build windows

package main

import webview2 "github.com/jchv/go-webview2"

func runNativeApp(hub *Hub, window *WindowController) error {
	w := webview2.New(false)
	defer w.Destroy()

	w.SetTitle("nview")
	w.SetSize(1200, 900, webview2.HintNone)
	hideWebViewWindow(w)
	if err := attachNativeWindow(window, w, hub); err != nil {
		return err
	}
	hideWebViewWindow(w)
	w.Run()
	return nil
}

func hideWebViewWindow(w webview2.WebView) {
	if h, ok := w.(interface{ Hide() error }); ok {
		_ = h.Hide()
	}
}
