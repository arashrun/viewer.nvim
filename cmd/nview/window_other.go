//go:build !linux && !darwin && !windows

package main

func nativeWindowTitle(title string) string { return title }
