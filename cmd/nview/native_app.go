package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	webview "github.com/webview/webview_go"
)

func runNativeApp(hub *Hub, window *WindowController) error {
	if !hasGraphicalSession() {
		return errors.New("no graphical session available: DISPLAY/Wayland not set")
	}

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("nview")
	w.SetSize(1200, 900, webview.HintNone)
	if err := window.Attach(w, hub); err != nil {
		return err
	}
	w.Run()
	return nil
}

func renderAppHTML(state ViewState) string {
	data, _ := json.Marshal(state)
	jsState := string(data)
	return strings.ReplaceAll(pageHTML, "{{STATE}}", jsState)
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}
