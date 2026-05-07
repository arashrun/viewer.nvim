package main

import (
	"encoding/json"
	"sync"

	webview "github.com/webview/webview_go"
)

type WindowController struct {
	mu     sync.Mutex
	title  string
	view   webview.WebView
	hub    *Hub
	done   chan struct{}
	closed bool
}

func NewWindowController(title, _ string) *WindowController {
	return &WindowController{
		title: title,
		done:  make(chan struct{}),
	}
}

func (w *WindowController) Attach(view webview.WebView, hub *Hub) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.view = view
	w.hub = hub
	w.view.SetTitle(w.title)
	w.view.SetSize(1200, 900, webview.HintNone)
	w.view.SetHtml(renderAppHTML(hub.Snapshot()))

	sub := hub.Subscribe()
	go func() {
		defer hub.Unsubscribe(sub)
		for {
			select {
			case <-sub:
				state := hub.Snapshot()
				payload, _ := json.Marshal(state)
				js := "window.__applyState(" + string(payload) + ");"
				if w.view != nil {
					w.view.Dispatch(func() {
						w.view.Eval(js)
					})
				}
			case <-w.done:
				return
			}
		}
	}()

	return nil
}

func (w *WindowController) Done() <-chan struct{} {
	return w.done
}

func (w *WindowController) Show() error {
	return nil
}

func (w *WindowController) Hide() error {
	return nil
}

func (w *WindowController) SetVisible(visible bool) error {
	_ = visible
	return nil
}

func (w *WindowController) Resize(payload map[string]any) error {
	_ = payload
	return nil
}

func (w *WindowController) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		close(w.done)
		w.closed = true
	}
	if w.view != nil {
		w.view.Dispatch(func() {
			w.view.Terminate()
		})
	}
	return nil
}
