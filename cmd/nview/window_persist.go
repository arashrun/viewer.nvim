package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func defaultStatePath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return ".nview-window.json"
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "viewer.nvim", "nview-window.json")
}

func loadWindowState(path string) (WindowState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WindowState{}, err
	}
	var raw struct {
		Bounds        WindowBounds `json:"bounds"`
		TopMost       bool         `json:"topMost"`
		Visible       bool         `json:"visible"`
		Focused       bool         `json:"focused"`
		HeaderVisible *bool        `json:"headerVisible"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return WindowState{}, err
	}
	state := WindowState{
		Bounds:  raw.Bounds,
		TopMost: raw.TopMost,
		Visible: raw.Visible,
		Focused: raw.Focused,
	}
	if raw.HeaderVisible != nil {
		state.HeaderVisible = *raw.HeaderVisible
	}
	return state, nil
}

func saveWindowState(path string, state WindowState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
