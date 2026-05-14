package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func defaultDocsRoot() string {
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "viewer.nvim", "docs")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "AppData", "Local", "viewer.nvim", "docs")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "viewer.nvim", "docs")
		}
	default:
		if base := os.Getenv("XDG_DATA_HOME"); base != "" {
			return filepath.Join(base, "viewer.nvim", "docs")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "share", "viewer.nvim", "docs")
		}
	}
	return filepath.Join(".", "viewer.nvim", "docs")
}

func defaultDocsCacheDir(docsRoot string) string {
	if docsRoot == "" {
		docsRoot = defaultDocsRoot()
	}
	return filepath.Join(docsRoot, "cache")
}
