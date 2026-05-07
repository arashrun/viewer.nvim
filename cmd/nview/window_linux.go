//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type WindowController struct {
	mu           sync.Mutex
	url          string
	title        string
	browserPath  string
	profileDir   string
	process      *exec.Cmd
	windowID     string
	visible      bool
	bootstrapped bool
}

func NewWindowController(url, title string) *WindowController {
	return &WindowController{
		url:         url,
		title:       title,
		browserPath: detectBrowser(),
		profileDir:  filepath.Join(os.TempDir(), "viewer-nview-profile"),
	}
}

func (w *WindowController) Ensure() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.ensureLocked()
}

func (w *WindowController) ensureLocked() error {

	if os.Getenv("DISPLAY") == "" {
		return errors.New("DISPLAY is not set")
	}
	if w.browserPath == "" {
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
		"--app=" + w.url,
		"--new-window",
		"--user-data-dir=" + w.profileDir,
		"--window-size=1200,900",
	}
	cmd := exec.Command(w.browserPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	w.process = cmd
	w.bootstrapped = true
	go func() {
		_ = cmd.Wait()
	}()

	return w.waitForWindowLocked(5 * time.Second)
}

func (w *WindowController) SetVisible(visible bool) error {
	if err := w.Ensure(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if visible {
		if err := w.showLocked(); err != nil {
			return err
		}
		w.visible = true
		return nil
	}

	if err := w.hideLocked(); err != nil {
		return err
	}
	w.visible = false
	return nil
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

	id, err := w.ensureWindowIDLocked()
	if err != nil {
		return err
	}

	width, height := estimatePixelSize(payload)
	return runWMCtrl("-i", "-r", id, "-e", "0,-1,-1,"+strconv.Itoa(width)+","+strconv.Itoa(height))
}

func (w *WindowController) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.process != nil && w.process.Process != nil {
		_ = w.process.Process.Kill()
	}

	w.process = nil
	w.windowID = ""
	w.visible = false
	w.bootstrapped = false
	return nil
}

func (w *WindowController) waitForWindowLocked(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		id, err := w.findWindowIDLocked()
		if err == nil {
			w.windowID = id
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("window %q not found", w.title)
}

func (w *WindowController) ensureWindowIDLocked() (string, error) {
	if w.windowID != "" {
		return w.windowID, nil
	}
	id, err := w.findWindowIDLocked()
	if err != nil {
		return "", err
	}
	w.windowID = id
	return id, nil
}

func (w *WindowController) findWindowIDLocked() (string, error) {
	out, err := exec.Command("wmctrl", "-lx").Output()
	if err != nil {
		return "", err
	}

	titleLower := strings.ToLower(w.title)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if !strings.Contains(strings.ToLower(line), titleLower) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0], nil
		}
	}

	return "", errors.New("window not found")
}

func (w *WindowController) showLocked() error {
	id, err := w.ensureWindowIDLocked()
	if err != nil {
		return err
	}

	if err := runWMCtrl("-i", "-r", id, "-b", "remove,hidden"); err != nil {
		return err
	}
	if err := runWMCtrl("-i", "-r", id, "-b", "add,above"); err != nil {
		return err
	}
	return runWMCtrl("-i", "-a", id)
}

func (w *WindowController) hideLocked() error {
	id, err := w.ensureWindowIDLocked()
	if err != nil {
		return err
	}

	return runWMCtrl("-i", "-r", id, "-b", "add,hidden")
}

func detectBrowser() string {
	if path, err := exec.LookPath("google-chrome"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium-browser"); err == nil {
		return path
	}
	return ""
}

func runWMCtrl(args ...string) error {
	cmd := exec.Command("wmctrl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
