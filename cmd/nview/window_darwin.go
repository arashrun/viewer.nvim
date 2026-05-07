//go:build darwin

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type WindowController struct {
	mu           sync.Mutex
	url          string
	title        string
	browserName  string
	profileDir   string
	process      *exec.Cmd
	bootstrapped bool
}

func NewWindowController(url, title string) *WindowController {
	return &WindowController{
		url:         url,
		title:       title,
		browserName: detectDarwinBrowser(),
		profileDir:  filepath.Join(os.TempDir(), "viewer-nview-profile"),
	}
}

func (w *WindowController) Ensure() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ensureLocked()
}

func (w *WindowController) ensureLocked() error {
	if w.browserName == "" {
		return errors.New("no browser found")
	}
	if w.process != nil && w.process.Process != nil {
		if err := w.process.Process.Signal(syscall.Signal(0)); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(w.profileDir, 0o755); err != nil {
		return err
	}

	args := []string{
		"-na", w.browserName,
		"--args",
		"--app=" + w.url,
		"--new-window",
		"--user-data-dir=" + w.profileDir,
		"--window-size=1200,900",
	}
	cmd := exec.Command("open", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	w.process = cmd
	w.bootstrapped = true
	go func() { _ = cmd.Wait() }()
	return nil
}

func (w *WindowController) SetVisible(visible bool) error {
	if err := w.Ensure(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	var script string
	if visible {
		script = `
tell application "` + w.browserName + `" to activate
tell application "System Events" to set visible of process "` + w.browserName + `" to true
`
	} else {
		script = `
tell application "System Events" to set visible of process "` + w.browserName + `" to false
`
	}
	return runOSAScript(script)
}

func (w *WindowController) Show() error {
	return w.SetVisible(true)
}

func (w *WindowController) Hide() error {
	return w.SetVisible(false)
}

func (w *WindowController) Resize(payload map[string]any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.bootstrapped {
		return nil
	}

	width, height := estimatePixelSize(payload)
	script := fmt.Sprintf(`
tell application "System Events" to tell process "%s" to set bounds of front window to {0, 0, %d, %d}
`, w.browserName, width, height)
	return runOSAScript(script)
}

func (w *WindowController) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.process != nil && w.process.Process != nil {
		_ = w.process.Process.Kill()
	}
	w.process = nil
	w.bootstrapped = false
	return nil
}

func detectDarwinBrowser() string {
	for _, name := range []string{"Google Chrome", "Chromium", "Microsoft Edge"} {
		if _, err := exec.LookPath("open"); err == nil {
			return name
		}
	}
	return ""
}

func runOSAScript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
