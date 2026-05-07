//go:build windows

package main

import (
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetFocus        = user32.NewProc("SetFocus")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
	procGetWindowLong   = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLong   = user32.NewProc("SetWindowLongPtrW")
)

const (
	sWHide          = 0
	sWShow          = 5
	hwndTopMost     = ^uintptr(0)
	hwndNoTopMost   = uintptr(0)
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
	gwlExStyle      = -20
	wsExToolWindow  = 0x00000080
	wsExAppWindow   = 0x00040000
)

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

func (n nativeWindow) SetBounds(bounds WindowBounds) {
	hwnd := uintptr(n.view.Window())
	if hwnd == 0 {
		return
	}
	_, _, _ = procSetWindowPos.Call(
		hwnd,
		hwndNoTopMost,
		uintptr(bounds.X),
		uintptr(bounds.Y),
		uintptr(bounds.Width),
		uintptr(bounds.Height),
		swpNoActivate|swpFrameChanged,
	)
}

func (n nativeWindow) setToolWindowStyle() {
	hwnd := uintptr(n.view.Window())
	if hwnd == 0 {
		return
	}
	style := getWindowLong(hwnd, gwlExStyle)
	style &^= wsExAppWindow
	style |= wsExToolWindow
	setWindowLong(hwnd, gwlExStyle, style)
	_, _, _ = procSetWindowPos.Call(
		hwnd,
		hwndNoTopMost,
		0,
		0,
		0,
		0,
		swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged,
	)
}

func (n nativeWindow) SetTopMost(topMost bool) {
	hwnd := uintptr(n.view.Window())
	if hwnd == 0 {
		return
	}
	insertAfter := hwndNoTopMost
	if topMost {
		insertAfter = hwndTopMost
	}
	_, _, _ = procSetWindowPos.Call(
		hwnd,
		insertAfter,
		0,
		0,
		0,
		0,
		swpNoActivate|swpNoMove|swpNoZOrder|swpFrameChanged|swpShowWindow,
	)
}

func (n nativeWindow) Show() {
	_ = n.view.Show()
}

func (n nativeWindow) Hide() {
	_ = n.view.Hide()
}

func (n nativeWindow) Focus() {
	_, _, _ = procSetFocus.Call(uintptr(n.view.Window()))
}

func (n nativeWindow) CurrentBounds() (WindowBounds, bool) {
	hwnd := uintptr(n.view.Window())
	if hwnd == 0 {
		return WindowBounds{}, false
	}
	var rect struct {
		Left   int32
		Top    int32
		Right  int32
		Bottom int32
	}
	r, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if r == 0 {
		return WindowBounds{}, false
	}
	return WindowBounds{
		X:      int(rect.Left),
		Y:      int(rect.Top),
		Width:  int(rect.Right - rect.Left),
		Height: int(rect.Bottom - rect.Top),
	}, true
}

func (n nativeWindow) Terminate() {
	n.view.Terminate()
}

func attachNativeWindow(window *WindowController, w webview2.WebView, hub *Hub) error {
	n := nativeWindow{view: w}
	n.setToolWindowStyle()
	return window.Attach(n, hub)
}

func getWindowLong(hwnd uintptr, index int) uintptr {
	ret, _, _ := procGetWindowLong.Call(hwnd, uintptr(index))
	return ret
}

func setWindowLong(hwnd uintptr, index int, value uintptr) uintptr {
	ret, _, _ := procSetWindowLong.Call(hwnd, uintptr(index), value)
	return ret
}
