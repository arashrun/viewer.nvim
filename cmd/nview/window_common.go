package main

import "encoding/json"

type NativeWindow interface {
	SetTitle(title string)
	SetHtml(html string)
	Eval(js string)
	Dispatch(fn func())
	Resize(width, height int)
	SetBounds(bounds WindowBounds)
	SetTopMost(topMost bool)
	Show()
	Hide()
	Focus()
	CurrentBounds() (WindowBounds, bool)
	Terminate()
}

type WindowController struct {
	title  string
	view   NativeWindow
	hub    *Hub
	state  WindowState
	done   chan struct{}
	closed bool
}

func NewWindowController(title, _ string) *WindowController {
	return &WindowController{
		title: title,
		state: defaultWindowState(),
		done:  make(chan struct{}),
	}
}

func (w *WindowController) Attach(view NativeWindow, hub *Hub) error {
	w.view = view
	w.hub = hub
	w.view.SetTitle(w.title)
	w.view.SetHtml(renderAppHTML(hub.Snapshot()))
	w.applyState()

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
	w.state.Visible = true
	w.state.Focused = true
	w.view.Dispatch(func() {
		w.view.Show()
		w.view.SetTopMost(true)
		w.view.Focus()
		w.view.SetTitle(w.title)
	})
	return nil
}

func (w *WindowController) Hide() error {
	w.state.Visible = false
	w.state.Focused = false
	if w.view == nil {
		return nil
	}
	w.view.Dispatch(func() {
		w.view.Hide()
	})
	return nil
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

func (w *WindowController) RememberBounds() {
	if w.view == nil {
		return
	}
	if bounds, ok := w.view.CurrentBounds(); ok {
		w.state.Bounds = bounds
	}
}

func (w *WindowController) applyState() {
	if w.view == nil {
		return
	}
	bounds := w.state.Bounds
	if bounds.Width > 0 && bounds.Height > 0 {
		w.view.SetBounds(bounds)
	}
	w.view.SetTopMost(w.state.TopMost)
	if w.state.Visible {
		w.view.Show()
		if w.state.Focused {
			w.view.Focus()
		}
		return
	}
	w.view.Hide()
	if w.state.Focused {
		w.view.Focus()
	}
}

func (w *WindowController) Stop() error {
	w.RememberBounds()
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
