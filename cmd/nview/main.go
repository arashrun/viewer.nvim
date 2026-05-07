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
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type Message struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type ViewState struct {
	Connected bool           `json:"connected"`
	Focused   bool           `json:"focused"`
	FileType  string         `json:"filetype"`
	Path      string         `json:"path"`
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
	clients map[chan struct{}]struct{}
}

func NewHub() *Hub {
	return &Hub{
		state: ViewState{
			Focused:   true,
			Connected: false,
			UpdatedAt: time.Now(),
		},
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

func renderAppHTML(state ViewState) string {
	data, _ := json.Marshal(state)
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, struct {
		ViewState
		StateJSON template.JS
	}{
		ViewState: state,
		StateJSON: template.JS(data),
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
	if state, err := loadWindowState(*statePath); err == nil {
		window.state = mergeWindowState(defaultWindowState(), state)
	}

	go func() {
		if err := serveTCP(*listenAddr, hub, window); err != nil {
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

func serveTCP(addr string, hub *Hub, window *WindowController) error {
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
		go handleConn(conn, hub, window)
	}
}

func handleConn(conn net.Conn, hub *Hub, window *WindowController) {
	defer conn.Close()

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
		switch msg.Type {
		case "hello":
			hub.Update(func(state *ViewState) {
				state.Connected = true
				state.LastType = msg.Type
			})
			_ = encoder.Encode(Message{
				Type: "ack",
				Payload: map[string]any{
					"ok": true,
				},
			})
			_ = window.Show()
		case "preview":
			updatePreview(hub, msg)
			_ = window.Show()
		case "viewport":
			updateViewport(hub, msg)
		case "focus":
			updateFocus(hub, msg)
			if focused, ok := msg.Payload["focused"].(bool); ok {
				if focused {
					_ = window.Show()
				} else {
					_ = window.Hide()
				}
			}
		case "close":
			hub.Update(func(state *ViewState) {
				state.Connected = false
				state.LastType = msg.Type
			})
			_ = window.Hide()
		}
	}
}

func updatePreview(hub *Hub, msg Message) {
	hub.Update(func(state *ViewState) {
		state.LastType = msg.Type
		if v, ok := msg.Payload["path"].(string); ok {
			state.Path = v
		}
		if v, ok := msg.Payload["filetype"].(string); ok {
			state.FileType = v
		}
		if v, ok := msg.Payload["lines"].([]any); ok {
			lines := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					lines = append(lines, s)
				}
			}
			md := joinLines(lines)
			state.Markdown = md
			state.HTML = renderMarkdown(md)
		}
	})
}

func updateViewport(hub *Hub, msg Message) {
	hub.Update(func(state *ViewState) {
		state.LastType = msg.Type
		state.Viewport = msg.Payload
		if cursor, ok := msg.Payload["cursor"].(map[string]any); ok {
			state.Cursor = cursor
		}
	})
}

func updateFocus(hub *Hub, msg Message) {
	hub.Update(func(state *ViewState) {
		state.LastType = msg.Type
		if focused, ok := msg.Payload["focused"].(bool); ok {
			state.Focused = focused
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
    body {
      margin: 0;
      background: radial-gradient(circle at top, #fff 0%, var(--bg) 55%, #ece3d4 100%);
      color: var(--text);
      font: 15px/1.6 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    header {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      padding: 20px 24px;
      border-bottom: 1px solid var(--border);
      background: rgba(255,255,255,0.75);
      backdrop-filter: blur(10px);
      position: sticky;
      top: 0;
    }
    .title {
      font-size: 14px;
      text-transform: uppercase;
      letter-spacing: 0.18em;
      color: var(--muted);
    }
    .meta {
      font-size: 13px;
      color: var(--muted);
      max-width: 60vw;
      text-align: right;
      word-break: break-word;
    }
    main {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 20px;
      padding: 24px;
      max-width: 1100px;
      margin: 0 auto;
    }
    .card {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 18px;
      box-shadow: 0 10px 30px rgba(35, 31, 22, 0.06);
      overflow: hidden;
    }
    .card h2 {
      margin: 0;
      padding: 16px 20px;
      font-size: 13px;
      letter-spacing: 0.12em;
      text-transform: uppercase;
      border-bottom: 1px solid var(--border);
      background: #fff;
    }
    .content {
      padding: 22px 24px 30px;
      overflow: auto;
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
      max-width: 74ch;
      margin: 0 auto;
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
  </style>
</head>
<body>
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

    window.__applyState = function(state) {
      statusEl.textContent = state.connected ? 'connected' : 'waiting for nvim';
      statusEl.classList.toggle('off', !state.connected);
      pathEl.textContent = state.path || 'No document yet';
      const cursor = state.cursor ? 'cursor ' + (state.cursor.row || 0) + ':' + (state.cursor.col || 0) : 'cursor idle';
      infoEl.textContent = (state.filetype || 'unknown filetype') + ' · ' + cursor;
      previewEl.innerHTML = state.html || '<div class="placeholder">Open a markdown buffer in nvim and run :ViewerPreview</div>';
    };
    window.__applyState({{.StateJSON}});
  </script>
</body>
</html>`

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))
