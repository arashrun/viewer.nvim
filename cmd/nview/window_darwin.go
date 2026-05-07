//go:build darwin

package main

import "strings"

func applyNativeWindowVisible(_ bool) error              { return nil }
func applyNativeWindowSize(payload map[string]any) error { _ = payload; return nil }
func nativeWindowTitle(title string) string              { return strings.TrimSpace(title) }
func nativeWindowHandle() uintptr                        { return 0 }
func nativeWindowReady() bool                            { return true }
