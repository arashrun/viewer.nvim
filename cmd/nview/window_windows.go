//go:build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	bootstrapped bool
}

func NewWindowController(url, title string) *WindowController {
	return &WindowController{
		url:         url,
		title:       title,
		browserPath: detectWindowsBrowser(),
		profileDir:  filepath.Join(os.TempDir(), "viewer-nview-profile"),
	}
}

func (w *WindowController) Ensure() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ensureLocked()
}

func (w *WindowController) ensureLocked() error {
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
	go func() { _ = cmd.Wait() }()
	return w.waitForWindowLocked(5 * time.Second)
}

func (w *WindowController) SetVisible(visible bool) error {
	if err := w.Ensure(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if visible {
		return runPowerShell(w.showScriptLocked())
	}
	return runPowerShell(w.hideScriptLocked())
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
	return runPowerShell(fmt.Sprintf(`
%s
$h = Get-NViewWindowHandle
if ($h -ne [IntPtr]::Zero) {
  [Native]::SetWindowPos($h, [Native]::HWND_NOTOPMOST, 0, 0, %d, %d, 0x0001 -bor 0x0004) | Out-Null
}
`, windowsNativePreamble(), width, height))
}

func (w *WindowController) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.process != nil && w.process.Process != nil {
		_ = w.process.Process.Kill()
	}

	w.process = nil
	w.windowID = ""
	w.bootstrapped = false
	return nil
}

func (w *WindowController) waitForWindowLocked(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := w.findWindowLocked(); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("window %q not found", w.title)
}

func (w *WindowController) findWindowLocked() (string, error) {
	script := windowsNativePreamble() + `
$h = Get-NViewWindowHandle
if ($h -eq [IntPtr]::Zero) { exit 1 }
[Console]::WriteLine($h.ToInt64())
`
	out, err := runPowerShellOutput(script)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", errors.New("window not found")
	}
	w.windowID = id
	return id, nil
}

func (w *WindowController) showScriptLocked() string {
	return windowsNativePreamble() + `
$h = Get-NViewWindowHandle
if ($h -ne [IntPtr]::Zero) {
  [Native]::ShowWindow($h, 5) | Out-Null
  [Native]::SetWindowPos($h, [Native]::HWND_TOPMOST, 0, 0, 0, 0, 0x0001 -bor 0x0002) | Out-Null
  [Native]::SetWindowPos($h, [Native]::HWND_NOTOPMOST, 0, 0, 0, 0, 0x0001 -bor 0x0002) | Out-Null
}
`
}

func (w *WindowController) hideScriptLocked() string {
	return windowsNativePreamble() + `
$h = Get-NViewWindowHandle
if ($h -ne [IntPtr]::Zero) {
  [Native]::ShowWindow($h, 0) | Out-Null
}
`
}

func detectWindowsBrowser() string {
	for _, name := range []string{"msedge.exe", "chrome.exe", "msedge", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func runPowerShell(script string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runPowerShellOutput(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return stdout.String(), nil
}

func windowsNativePreamble() string {
	return `
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class Native {
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr hWnd, System.Text.StringBuilder text, int count);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);
  public static readonly IntPtr HWND_TOPMOST = new IntPtr(-1);
  public static readonly IntPtr HWND_NOTOPMOST = new IntPtr(-2);
}
function Get-NViewWindowHandle {
  $target = "nview"
  $script:result = [IntPtr]::Zero
  [Native]::EnumWindows({
    param($hWnd, $lParam)
    if (-not [Native]::IsWindowVisible($hWnd)) { return $true }
    $sb = New-Object System.Text.StringBuilder 512
    [void][Native]::GetWindowTextW($hWnd, $sb, $sb.Capacity)
    if ($sb.ToString().ToLower().Contains($target)) {
      $script:result = $hWnd
      return $false
    }
    return $true
  }, [IntPtr]::Zero) | Out-Null
  return $script:result
}
`
}
