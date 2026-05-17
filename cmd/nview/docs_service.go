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

func (s *DocsService) Query(sessionID, query string) {
	_ = sessionID
	query = strings.TrimSpace(query)
	s.mu.Lock()
	s.lastQuery = query
	s.mu.Unlock()

	launchErr := s.launchZeal(query)
	html := renderZealStatusHTML(query, s.zealCmd, launchErr)
	hub := globalHub
	if hub == nil {
		return
	}
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = sessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.LineCount = 0
		client.Markdown = ""
		client.HTML = html
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
	})
}

func (s *DocsService) Open(sessionID, resultID string) {
	_ = resultID
	hub := globalHub
	if hub == nil {
		return
	}
	query := ""
	hub.mu.Lock()
	if client := hub.clientsState[sessionID]; client != nil {
		query = client.DocsQuery
	}
	hub.mu.Unlock()
	launchErr := s.launchZeal(query)
	html := renderZealStatusHTML(query, s.zealCmd, launchErr)
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = sessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.LineCount = 0
		client.Markdown = ""
		client.HTML = html
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
	})
}

func (s *DocsService) Back(sessionID string) {
	_ = sessionID
}

func (s *DocsService) launchZeal(query string) error {
	if err := s.openZealURL(query); err == nil {
		return nil
	}
	return s.launchZealCommand(query)
}

func (s *DocsService) openZealURL(query string) error {
	scheme := "dash"
	escaped := url.QueryEscape(strings.TrimSpace(query))
	target := scheme + "://docset:" + escaped
	cmd := openURLCommand(target)
	return cmd.Start()
}

func (s *DocsService) launchZealCommand(query string) error {
	exe := strings.TrimSpace(s.zealCmd)
	if exe == "" {
		exe = defaultZealCommand()
	}
	if _, err := exec.LookPath(exe); err != nil && !strings.ContainsRune(exe, '/') && !strings.ContainsRune(exe, '\\') {
		return err
	}
	args := []string{}
	if query != "" {
		args = append(args, query)
	}
	cmd := exec.Command(exe, args...)
	return cmd.Start()
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

func renderZealStatusHTML(query, zealCmd string, launchErr error) template.HTML {
	var buf bytes.Buffer
	buf.WriteString(`<div class="docs-shell docs-shell-empty">`)
	buf.WriteString(`<aside class="docs-sidebar">`)
	buf.WriteString(`<div class="docs-sidebar-header">`)
	buf.WriteString(`<div class="docs-title">Offline docs</div>`)
	buf.WriteString(`<div class="docs-meta">Zeal</div>`)
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
		buf.WriteString(`Zeal has been started for this query.`)
	}
	buf.WriteString(`</div>`)
	buf.WriteString(`</section></div>`)
	return template.HTML(buf.String())
}
