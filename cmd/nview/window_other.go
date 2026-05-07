//go:build !linux && !darwin && !windows

package main

func applyNativeWindowVisible(_ bool) error              { return nil }
func applyNativeWindowSize(payload map[string]any) error { _ = payload; return nil }
func nativeWindowTitle(title string) string              { return title }
func nativeWindowHandle() uintptr                        { return 0 }
func nativeWindowReady() bool                            { return false }
