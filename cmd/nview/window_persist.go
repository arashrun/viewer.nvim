package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type persistedWindowState struct {
	Default  WindowState            `json:"default"`
	Sessions map[string]WindowState `json:"sessions,omitempty"`
}

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

func loadWindowState(path string) (persistedWindowState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return persistedWindowState{}, err
	}
	var raw persistedWindowState
	if err := json.Unmarshal(data, &raw); err == nil {
		if raw.Sessions == nil {
			raw.Sessions = make(map[string]WindowState)
		}
		if raw.Default.Valid() {
			raw.Default = mergeWindowState(defaultWindowState(), raw.Default)
		} else {
			raw.Default = defaultWindowState()
		}
		return raw, nil
	}

	var legacy WindowState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return persistedWindowState{}, err
	}
	return persistedWindowState{
		Default:  mergeWindowState(defaultWindowState(), legacy),
		Sessions: map[string]WindowState{},
	}, nil
}

func saveWindowState(path string, state persistedWindowState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
