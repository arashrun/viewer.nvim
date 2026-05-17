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

func (n nativeWindow) IsForeground() bool {
	return false
}

func (n nativeWindow) CurrentBounds() (WindowBounds, bool) {
	return WindowBounds{}, false
}

func (n nativeWindow) Terminate() {
	n.view.Terminate()
}

func attachNativeWindow(window *WindowController, w webview.WebView, hub *Hub, docs *DocsService) error {
	_ = w.Bind("toggleHeaderVisible", func() bool {
		return window.ToggleHeaderVisible()
	})
	_ = w.Bind("docsQuery", func(query string) {
		if docs != nil {
			sessionID := hub.clientKeyForSessionID(window.activeSession)
			filetype := ""
			hub.mu.Lock()
			if client := hub.clientsState[sessionID]; client != nil {
				filetype = client.DocsFileType
			}
			hub.mu.Unlock()
			if err := docs.Query(sessionID, filetype, query); err != nil {
				_ = window.Show()
			}
		}
	})
	_ = w.Bind("docsOpen", func(id string) {
		if docs != nil {
			if err := docs.Open(hub.clientKeyForSessionID(window.activeSession), id); err != nil {
				_ = window.Show()
			}
		}
	})
	_ = w.Bind("docsBack", func() {
		if docs != nil {
			docs.Back(hub.clientKeyForSessionID(window.activeSession))
		}
	})
	return window.Attach(nativeWindow{view: w}, hub)
}
