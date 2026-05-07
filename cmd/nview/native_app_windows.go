//go:build windows

package main

import webview2 "github.com/jchv/go-webview2"

func runNativeApp(hub *Hub, window *WindowController) error {
	w := webview2.New(false)
	defer w.Destroy()

	w.SetTitle("nview")
	w.SetSize(1200, 900, webview2.HintNone)
	if err := attachNativeWindow(window, w, hub); err != nil {
		return err
	}
	w.Run()
	return nil
}
