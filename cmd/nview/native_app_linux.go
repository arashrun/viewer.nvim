//go:build linux || darwin

package main

import (
	"errors"
	"os"
	"strings"

	webview "github.com/webview/webview_go"
)

func runNativeApp(hub *Hub, window *WindowController, docs *DocsService) error {
	if !hasGraphicalSession() {
		return errors.New("no graphical session available: DISPLAY/Wayland not set")
	}

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("nview")
	w.SetSize(1200, 900, webview.HintNone)
	if err := attachNativeWindow(window, w, hub, docs); err != nil {
		return err
	}
	w.Run()
	return nil
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}
