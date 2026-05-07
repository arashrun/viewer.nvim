package main

import "encoding/json"

type NativeWindow interface {
	SetTitle(title string)
	SetHtml(html string)
	Eval(js string)
	Dispatch(fn func())
	Resize(width, height int)
	Terminate()
}

type WindowController struct {
	title  string
	view   NativeWindow
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

func (w *WindowController) Attach(view NativeWindow, hub *Hub) error {
	w.view = view
	w.hub = hub
	w.view.SetTitle(w.title)
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
	if w.view == nil {
		return nil
	}
	w.view.Dispatch(func() {
		w.view.SetTitle(w.title)
	})
	return nil
}

func (w *WindowController) Hide() error {
	return w.Stop()
}

func (w *WindowController) SetVisible(visible bool) error {
	if visible {
		return w.Show()
	}
	return w.Hide()
}

func (w *WindowController) Resize(payload map[string]any) error {
	if w.view == nil {
		return nil
	}
	width, height := estimatePixelSize(payload)
	w.view.Dispatch(func() {
		w.view.Resize(width, height)
	})
	return nil
}

func (w *WindowController) Stop() error {
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
