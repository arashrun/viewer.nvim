//go:build !linux

package main

type WindowController struct{}

func NewWindowController(url, title string) *WindowController { return &WindowController{} }
func (w *WindowController) Ensure() error                     { return nil }
func (w *WindowController) SetVisible(visible bool) error     { return nil }
func (w *WindowController) Show() error                       { return nil }
func (w *WindowController) Hide() error                       { return nil }
func (w *WindowController) Resize(payload map[string]any) error {
	return nil
}
func (w *WindowController) Stop() error { return nil }
