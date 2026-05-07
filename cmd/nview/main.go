package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type Message struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type clientState struct {
	Path      string
	FileType  string
	LineCount int
	Markdown  string
	HTML      template.HTML
	Cursor    map[string]any
	Viewport  map[string]any
	LastType  string
	UpdatedAt time.Time
}

type ViewState struct {
	Connected bool           `json:"connected"`
	FileType  string         `json:"filetype"`
	Path      string         `json:"path"`
	LineCount int            `json:"lineCount"`
	HeaderVisible bool       `json:"headerVisible"`
	Cursor    map[string]any `json:"cursor,omitempty"`
	Viewport  map[string]any `json:"viewport,omitempty"`
	Markdown  string         `json:"markdown"`
	HTML      template.HTML  `json:"html"`
	UpdatedAt time.Time      `json:"updatedAt"`
	LastType  string         `json:"lastType"`
}

type Hub struct {
	mu      sync.Mutex
	state   ViewState
	clientsState map[string]*clientState
	activeClient string
	clients map[chan struct{}]struct{}
}

func NewHub() *Hub {
	return &Hub{
		state: ViewState{
			Connected: false,
			UpdatedAt: time.Now(),
		},
		clientsState: make(map[string]*clientState),
		clients: make(map[chan struct{}]struct{}),
	}
}

func (h *Hub) Snapshot() ViewState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *Hub) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

func (h *Hub) Broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (h *Hub) Update(mutator func(*ViewState)) {
	h.mu.Lock()
	mutator(&h.state)
	h.state.UpdatedAt = time.Now()
	h.mu.Unlock()
	h.Broadcast()
}

func (h *Hub) ensureClientLocked(sessionID string) *clientState {
	client, ok := h.clientsState[sessionID]
	if !ok {
		client = &clientState{}
		h.clientsState[sessionID] = client
	}
	return client
}

func (h *Hub) setActiveClientLocked(sessionID string) {
	h.activeClient = sessionID
	client := h.clientsState[sessionID]
	if client == nil {
		return
	}
	h.state.FileType = client.FileType
	h.state.Path = client.Path
	h.state.LineCount = client.LineCount
	h.state.Markdown = client.Markdown
	h.state.HTML = client.HTML
	h.state.Cursor = client.Cursor
	h.state.Viewport = client.Viewport
	h.state.LastType = client.LastType
	h.state.UpdatedAt = time.Now()
}

func (h *Hub) upsertClient(sessionID string, update func(*clientState)) {
	h.mu.Lock()
	client := h.ensureClientLocked(sessionID)
	update(client)
	client.UpdatedAt = time.Now()
	h.setActiveClientLocked(sessionID)
	h.state.Connected = len(h.clientsState) > 0
	h.mu.Unlock()
	h.Broadcast()
}

func (h *Hub) removeClient(sessionID string) {
	h.mu.Lock()
	delete(h.clientsState, sessionID)
	if h.activeClient == sessionID {
		h.activeClient = ""
		for id := range h.clientsState {
			h.activeClient = id
			break
		}
	}
	if h.activeClient != "" {
		h.setActiveClientLocked(h.activeClient)
	} else {
		h.state.Path = ""
		h.state.FileType = ""
		h.state.LineCount = 0
		h.state.Markdown = ""
		h.state.HTML = ""
		h.state.Cursor = nil
		h.state.Viewport = nil
		h.state.LastType = "disconnect"
		h.state.Connected = false
		h.state.UpdatedAt = time.Now()
	}
	h.mu.Unlock()
	h.Broadcast()
}

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

type DesktopApp struct {
	hub    *Hub
	window *WindowController
}

func NewDesktopApp(hub *Hub, window *WindowController) *DesktopApp {
	return &DesktopApp{hub: hub, window: window}
}

func (a *DesktopApp) Run() error {
	return runNativeApp(a.hub, a.window)
}

func renderMarkdown(source string) template.HTML {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &buf); err != nil {
		return template.HTML("<pre class=\"error\">render failed</pre>")
	}
	return template.HTML(buf.String())
}

func renderAppHTML(state ViewState, headerVisible bool) string {
	data, _ := json.Marshal(state)
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, struct {
		ViewState
		StateJSON     template.JS
		HeaderVisible bool
	}{
		ViewState:     state,
		StateJSON:     template.JS(data),
		HeaderVisible: headerVisible,
	}); err != nil {
		return ""
	}
	return buf.String()
}

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:7357", "tcp listen address")
	statePath := flag.String("state-file", defaultStatePath(), "window state file")
	flag.Parse()

	hub := NewHub()
	window := NewWindowController("nview", "nview")
	window.SetStateSaver(func(state WindowState) error {
		return saveWindowState(*statePath, state)
	})
	if state, err := loadWindowState(*statePath); err == nil {
		window.state = mergeWindowState(defaultWindowState(), state)
	}

	var lastMessageAt int64
	atomic.StoreInt64(&lastMessageAt, time.Now().UnixNano())
	go monitorWindowInactivity(window, &lastMessageAt)

	go func() {
		if err := serveTCP(*listenAddr, hub, window, &lastMessageAt); err != nil {
			log.Fatalf("nview tcp server error: %v", err)
		}
	}()

	log.Printf("nview tcp listening on %s", *listenAddr)
	log.Printf("nview desktop UI starting")

	app := NewDesktopApp(hub, window)
	if err := app.Run(); err != nil {
		log.Printf("nview desktop error: %v", err)
		os.Exit(1)
	}
	_ = saveWindowState(*statePath, window.state)
}

func monitorWindowInactivity(window *WindowController, lastMessageAt *int64) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if window.view != nil && window.view.IsForeground() {
			atomic.StoreInt64(lastMessageAt, time.Now().UnixNano())
			continue
		}
		if time.Since(time.Unix(0, atomic.LoadInt64(lastMessageAt))) >= 3*time.Second {
			if window.state.Visible {
				_ = window.Hide()
			}
		}
	}
}

func serveTCP(addr string, hub *Hub, window *WindowController, lastMessageAt *int64) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConn(conn, hub, window, lastMessageAt)
	}
}

func handleConn(conn net.Conn, hub *Hub, window *WindowController, lastMessageAt *int64) {
	defer conn.Close()
	sessionID := conn.RemoteAddr().String()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		log.Printf("recv %s", msg.Type)
		atomic.StoreInt64(lastMessageAt, time.Now().UnixNano())
		_ = window.Show()
		switch msg.Type {
		case "hello":
			hub.upsertClient(sessionID, func(client *clientState) {
				client.LastType = msg.Type
			})
			_ = encoder.Encode(Message{
				Type: "ack",
				Payload: map[string]any{
					"ok": true,
				},
			})
		case "preview":
			updatePreview(hub, sessionID, msg)
		case "viewport":
			updateViewport(hub, sessionID, msg)
		case "close":
			hub.upsertClient(sessionID, func(client *clientState) {
				client.LastType = msg.Type
			})
			hub.removeClient(sessionID)
		}
	}
	hub.removeClient(sessionID)
}

func updatePreview(hub *Hub, sessionID string, msg Message) {
	hub.upsertClient(sessionID, func(client *clientState) {
		client.LastType = msg.Type
		if v, ok := msg.Payload["path"].(string); ok {
			client.Path = v
		}
		if v, ok := msg.Payload["filetype"].(string); ok {
			client.FileType = v
		}
		if v, ok := msg.Payload["lines"].([]any); ok {
			lines := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					lines = append(lines, s)
				}
			}
			md := joinLines(lines)
			client.Markdown = md
			client.LineCount = len(lines)
			client.HTML = renderMarkdown(md)
		}
	})
}

func updateViewport(hub *Hub, sessionID string, msg Message) {
	hub.upsertClient(sessionID, func(client *clientState) {
		client.LastType = msg.Type
		client.Viewport = msg.Payload
		if cursor, ok := msg.Payload["cursor"].(map[string]any); ok {
			client.Cursor = cursor
		}
	})
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for i, line := range lines {
		buf.WriteString(line)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>nview</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f2ea;
      --panel: #fffdf8;
      --text: #1d1d1d;
      --muted: #67635b;
      --accent: #0f766e;
      --border: #ded7ca;
    }
    html, body {
      width: 100%;
      height: 100%;
      margin: 0;
      overflow: hidden;
    }
    body {
      display: flex;
      flex-direction: column;
      background: radial-gradient(circle at top, #fff 0%, var(--bg) 55%, #ece3d4 100%);
      color: var(--text);
      font: 15px/1.6 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 20px;
      padding: 10px 16px;
      border-bottom: 1px solid var(--border);
      background: rgba(255,255,255,0.75);
      backdrop-filter: blur(10px);
      position: sticky;
      top: 0;
      flex: 0 0 auto;
    }
    .title {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.16em;
      color: var(--muted);
    }
    .meta {
      font-size: 12px;
      color: var(--muted);
      max-width: 52vw;
      text-align: right;
      word-break: break-word;
      line-height: 1.35;
    }
    main {
      display: flex;
      flex-direction: column;
      gap: 12px;
      padding: 12px 16px 16px;
      width: min(1100px, 100%);
      flex: 1;
      min-height: 0;
      min-width: 0;
      margin: 0 auto;
      box-sizing: border-box;
      overflow: hidden;
    }
    .card {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 18px;
      box-shadow: 0 10px 30px rgba(35, 31, 22, 0.06);
      overflow: hidden;
      display: flex;
      flex-direction: column;
      min-height: 0;
      min-width: 0;
      max-width: 100%;
    }
    .card h2 {
      margin: 0;
      padding: 10px 14px;
      font-size: 11px;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      border-bottom: 1px solid var(--border);
      background: #fff;
      cursor: pointer;
      user-select: none;
    }
    .card h2:hover {
      background: #fbfaf7;
    }
    .content {
      flex: 1;
      padding: 16px 18px 22px;
      overflow: auto;
      scroll-behavior: auto;
      min-height: 0;
      min-width: 0;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-radius: 999px;
      background: #eef7f5;
      color: #0f4d47;
      font-size: 13px;
    }
    .status.off {
      background: #f3efe8;
      color: #6d6454;
    }
    article {
      max-width: 82ch;
      margin: 0 auto;
      min-width: 0;
    }
    article > *:first-child {
      margin-top: 0;
    }
    article img {
      max-width: 100%;
    }
    pre {
      padding: 16px;
      background: #171717;
      color: #f4f4f4;
      border-radius: 12px;
      overflow: auto;
    }
    code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    .placeholder {
      color: var(--muted);
      border: 1px dashed var(--border);
      border-radius: 16px;
      padding: 48px 24px;
      text-align: center;
      background: rgba(255,255,255,0.5);
    }
    .error {
      color: #b42318;
    }
    body.header-hidden header {
      max-height: 0;
      padding-top: 0;
      padding-bottom: 0;
      opacity: 0;
      border-bottom-width: 0;
      pointer-events: none;
      overflow: hidden;
    }
  </style>
</head>
<body{{if not .HeaderVisible}} class="header-hidden"{{end}}>
  <header>
    <div>
      <div class="title">nview</div>
      <div id="status" class="status{{if not .Connected}} off{{end}}">{{if .Connected}}connected{{else}}waiting for nvim{{end}}</div>
    </div>
    <div class="meta">
      <div id="path">{{if .Path}}{{.Path}}{{else}}No document yet{{end}}</div>
      <div id="info">{{if .FileType}}{{.FileType}}{{else}}unknown filetype{{end}} · {{if .Cursor}}cursor {{index .Cursor "row"}}:{{index .Cursor "col"}}{{else}}cursor idle{{end}}</div>
    </div>
  </header>
  <main>
    <section class="card">
      <h2>Preview</h2>
      <div class="content">
        <article id="preview">{{if .HTML}}{{.HTML}}{{else}}<div class="placeholder">Open a markdown buffer in nvim and run :ViewerPreview</div>{{end}}</article>
      </div>
    </section>
  </main>
  <script>
    const statusEl = document.getElementById('status');
    const pathEl = document.getElementById('path');
    const infoEl = document.getElementById('info');
    const previewEl = document.getElementById('preview');
    const contentEl = document.querySelector('.content');
    const previewHeadingEl = document.querySelector('.card h2');
    let headerVisible = {{if .HeaderVisible}}true{{else}}false{{end}};

    function scrollPreview(state) {
      if (!contentEl || !previewEl) {
        return;
      }

      const cursorRow = state.cursor && typeof state.cursor.row === 'number' ? state.cursor.row : 1;
      const lineCount = typeof state.lineCount === 'number' && state.lineCount > 1 ? state.lineCount : 1;
      const progress = Math.max(0, Math.min(1, (cursorRow - 1) / Math.max(1, lineCount - 1)));
      const maxScroll = Math.max(0, contentEl.scrollHeight - contentEl.clientHeight);
      contentEl.scrollTop = maxScroll * progress;
    }

    function applyHeaderVisible(visible) {
      headerVisible = !!visible;
      document.body.classList.toggle('header-hidden', !headerVisible);
    }

    window.__setHeaderVisible = function(visible) {
      applyHeaderVisible(visible);
    };

    if (previewHeadingEl) {
      previewHeadingEl.addEventListener('click', function() {
        if (typeof window.toggleHeaderVisible === 'function') {
          window.toggleHeaderVisible().then(function(nextVisible) {
            applyHeaderVisible(nextVisible);
          });
          return;
        }
        applyHeaderVisible(!headerVisible);
      });
    }

    window.__applyState = function(state) {
      statusEl.textContent = state.connected ? 'connected' : 'waiting for nvim';
      statusEl.classList.toggle('off', !state.connected);
      pathEl.textContent = state.path || 'No document yet';
      const cursor = state.cursor ? 'cursor ' + (state.cursor.row || 0) + ':' + (state.cursor.col || 0) : 'cursor idle';
      infoEl.textContent = (state.filetype || 'unknown filetype') + ' · ' + cursor;
      previewEl.innerHTML = state.html || '<div class="placeholder">Open a markdown buffer in nvim and run :ViewerPreview</div>';
      if (typeof state.headerVisible === 'boolean') {
        applyHeaderVisible(state.headerVisible);
      }
      scrollPreview(state);
    };
    applyHeaderVisible(headerVisible);
    window.__applyState({{.StateJSON}});
  </script>
</body>
</html>`

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))
