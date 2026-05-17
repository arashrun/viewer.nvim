package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type DocsService struct {
	zealCmd   string
	mu        sync.Mutex
	lastQuery string
}

func defaultZealCommand() string {
	if runtime.GOOS == "windows" {
		return "zeal.exe"
	}
	return "zeal"
}

func NewDocsService(zealCmd string) *DocsService {
	if strings.TrimSpace(zealCmd) == "" {
		zealCmd = defaultZealCommand()
	}
	return &DocsService{zealCmd: zealCmd}
}

func (s *DocsService) Query(sessionID, filetype, query string) error {
	query = strings.TrimSpace(query)
	s.mu.Lock()
	s.lastQuery = query
	s.mu.Unlock()

	hub := globalHub
	if hub == nil {
		return nil
	}

	currentFileType := strings.TrimSpace(filetype)
	hub.mu.Lock()
	if client := hub.clientsState[sessionID]; client != nil {
		if currentFileType == "" {
			currentFileType = client.DocsFileType
		}
	}
	hub.mu.Unlock()

	launchErr := s.launchDocs(currentFileType, query)
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = sessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.LineCount = 0
		client.Markdown = ""
		if launchErr != nil {
			client.HTML = renderZealStatusHTML(currentFileType, query, s.zealCmd, launchErr)
		} else {
			client.HTML = template.HTML("")
		}
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
		client.DocsFileType = currentFileType
	})
	return launchErr
}

func (s *DocsService) Open(sessionID, resultID string) error {
	_ = resultID
	hub := globalHub
	if hub == nil {
		return nil
	}
	query := ""
	currentFileType := ""
	hub.mu.Lock()
	if client := hub.clientsState[sessionID]; client != nil {
		query = client.DocsQuery
		currentFileType = client.DocsFileType
	}
	hub.mu.Unlock()
	launchErr := s.launchDocs(currentFileType, query)
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = sessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.LineCount = 0
		client.Markdown = ""
		if launchErr != nil {
			client.HTML = renderZealStatusHTML(currentFileType, query, s.zealCmd, launchErr)
		} else {
			client.HTML = template.HTML("")
		}
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
		client.DocsFileType = currentFileType
	})
	return launchErr
}

func (s *DocsService) Back(sessionID string) {
	_ = sessionID
}

func (s *DocsService) launchDocs(filetype, query string) error {
	if err := s.launchDashPluginURL(filetype, query); err == nil {
		return nil
	}
	if err := s.launchDashURL(filetype, query); err == nil {
		return nil
	}
	return s.launchZealCommand(filetype, query)
}

func (s *DocsService) launchDashPluginURL(filetype, query string) error {
	target := buildDashPluginURL(filetype, query)
	cmd := openURLCommand(target)
	return cmd.Start()
}

func (s *DocsService) launchDashURL(filetype, query string) error {
	target := buildDashURL(filetype, query)
	cmd := openURLCommand(target)
	return cmd.Start()
}

func (s *DocsService) launchZealCommand(filetype, query string) error {
	exe := strings.TrimSpace(s.zealCmd)
	if exe == "" {
		exe = defaultZealCommand()
	}
	if _, err := exec.LookPath(exe); err != nil && !strings.ContainsRune(exe, '/') && !strings.ContainsRune(exe, '\\') {
		return err
	}
	args := []string{}
	normalized := normalizeDashQuery(filetype, query)
	if normalized != "" {
		args = append(args, normalized)
	}
	cmd := exec.Command(exe, args...)
	return cmd.Start()
}

func buildDashPluginURL(filetype, query string) string {
	return "dash-plugin://keys=" + url.QueryEscape(resolveDashKeys(filetype)) + "&query=" + url.QueryEscape(query)
}

func buildDashURL(filetype, query string) string {
	return "dash://" + url.QueryEscape(resolveDashKeys(filetype)) + ":" + url.QueryEscape(query)
}

func resolveDashKeys(filetype string) string {
	ft := strings.ToLower(strings.TrimSpace(filetype))
	switch ft {
	case "go":
		return "go"
	case "rust":
		return "rust"
	case "python":
		return "python"
	case "lua":
		return "lua"
	case "javascript", "javascriptreact", "typescript", "typescriptreact":
		return "javascript"
	case "c":
		return "c"
	case "cpp", "c++", "cc", "cxx", "objc", "objcpp":
		return "cpp"
	case "markdown", "md":
		return "markdown"
	default:
		return ft
	}
}

func normalizeDashQuery(filetype, query string) string {
	keys := resolveDashKeys(filetype)
	if keys == "" || query == "" {
		return query
	}
	return keys + ":" + query
}

func docsDisplayName(filetype string) string {
	keys := resolveDashKeys(filetype)
	if keys == "" {
		return "docs"
	}
	return keys
}

func openURLCommand(target string) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		return exec.Command("open", target)
	default:
		return exec.Command("xdg-open", target)
	}
}

func renderZealStatusHTML(filetype, query, zealCmd string, launchErr error) template.HTML {
	var buf bytes.Buffer
	buf.WriteString(`<div class="docs-shell docs-shell-empty">`)
	buf.WriteString(`<aside class="docs-sidebar">`)
	buf.WriteString(`<div class="docs-sidebar-header">`)
	buf.WriteString(`<div class="docs-title">Offline docs</div>`)
	buf.WriteString(`<div class="docs-meta">`)
	buf.WriteString(template.HTMLEscapeString(docsDisplayName(filetype)))
	buf.WriteString(`</div>`)
	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="docs-empty">`)
	if query == "" {
		buf.WriteString(`Use the docs keymap or :ViewerDocs to search the current word.`)
	} else {
		buf.WriteString(`Query: <code>`)
		buf.WriteString(template.HTMLEscapeString(query))
		buf.WriteString(`</code>`)
	}
	buf.WriteString(`</div>`)
	buf.WriteString(`</aside>`)
	buf.WriteString(`<section class="docs-preview-pane">`)
	buf.WriteString(`<div class="docs-empty docs-preview-empty">`)
	if launchErr != nil {
		buf.WriteString(`Failed to launch <code>`)
		buf.WriteString(template.HTMLEscapeString(zealCmd))
		buf.WriteString(`</code>: `)
		buf.WriteString(template.HTMLEscapeString(fmt.Sprintf("%v", launchErr)))
	} else {
		buf.WriteString(`Launched via Dash/Zeal URL handling with command fallback.`)
	}
	buf.WriteString(`</div>`)
	buf.WriteString(`</section></div>`)
	return template.HTML(buf.String())
}
