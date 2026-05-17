package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

type docsEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	DocsetName   string `json:"docsetName"`
	DocsetPath   string `json:"docsetPath"`
	DocsetRoot   string `json:"docsetRoot"`
	RelativePath string `json:"relativePath"`
	Fragment     string `json:"fragment,omitempty"`
	Score        int    `json:"score"`
}

type docsDocset struct {
	Root            string
	Name            string
	Version         string
	IndexFilePath   string
	DocumentsPath   string
	SearchIndexPath string
	PlatformFamily  string
	Metadata        map[string]string
	Entries         []docsEntry
	lastKnownSource string
}

type docsIndex struct {
	Docsets []*docsDocset
	Entries []docsEntry
}

type docsCacheState struct {
	Root  string                 `json:"root"`
	Cache string                 `json:"cache"`
	Index []docsDocsetCacheEntry `json:"index"`
}

type docsDocsetCacheEntry struct {
	Root            string            `json:"root"`
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	IndexFilePath   string            `json:"indexFilePath"`
	DocumentsPath   string            `json:"documentsPath"`
	SearchIndexPath string            `json:"searchIndexPath"`
	PlatformFamily  string            `json:"platformFamily"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Entries         []docsEntry       `json:"entries"`
}

type DocsService struct {
	root     string
	cacheDir string
	mu       sync.Mutex
	index    *docsIndex
}

func NewDocsService(root, cacheDir string) *DocsService {
	if root == "" {
		root = defaultDocsRoot()
	}
	if cacheDir == "" {
		cacheDir = defaultDocsCacheDir(root)
	}
	return &DocsService{root: root, cacheDir: cacheDir}
}

func (s *DocsService) ensureIndex() *docsIndex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		return s.index
	}

	if idx, ok := s.loadCache(); ok {
		s.index = idx
		return idx
	}

	idx, err := s.scanIndex()
	if err != nil {
		s.index = &docsIndex{}
		return s.index
	}
	s.index = idx
	_ = s.saveCache(idx)
	return idx
}

func (s *DocsService) Query(sessionID, query string) {
	query = strings.TrimSpace(query)
	index := s.ensureIndex()
	results := searchDocs(index, query, 100)
	html := renderDocsResultsHTML(s.root, query, results, len(index.Docsets), "")
	hub := globalHub
	if hub == nil {
		return
	}
	logicalSessionID := logicalSessionIDForClient(hub, sessionID)
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = logicalSessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.LineCount = 0
		client.Markdown = ""
		client.HTML = html
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
		client.DocsResults = results
		client.DocsPreviewTitle = ""
		client.DocsPreviewPath = ""
		client.DocsPreviewAnchor = ""
		client.DocsPreviewID = ""
		client.DocsCount = len(results)
		client.DocsRoot = s.root
		client.DocsCache = s.cacheDir
	})
}

func (s *DocsService) Open(sessionID, resultID string) {
	index := s.ensureIndex()
	entry, ok := findDocEntry(index, resultID)
	hub := globalHub
	if hub == nil {
		return
	}
	logicalSessionID := logicalSessionIDForClient(hub, sessionID)
	if !ok {
		hub.mu.Lock()
		query := ""
		if client := hub.clientsState[sessionID]; client != nil {
			query = client.DocsQuery
		}
		hub.mu.Unlock()
		hub.upsertClient(sessionID, func(client *clientState) {
			client.SessionID = logicalSessionID
			client.Mode = "docs"
			client.FileType = "docs"
			client.HTML = renderDocsResultsHTML(s.root, query, nil, len(index.Docsets), "")
			client.DocsQuery = query
			client.DocsCount = 0
			client.DocsPreviewTitle = ""
			client.DocsPreviewPath = ""
			client.DocsPreviewAnchor = ""
			client.DocsPreviewID = ""
		})
		return
	}

	renderedHTML, err := renderDocEntryHTML(entry)
	if err != nil {
		renderedHTML = template.HTML(`<pre class="error">failed to render document</pre>`)
	}
	query := ""
	hub.mu.Lock()
	if client := hub.clientsState[sessionID]; client != nil {
		query = client.DocsQuery
	}
	hub.mu.Unlock()

	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = logicalSessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = docEntryDisplayPath(entry)
		client.LineCount = 0
		client.Markdown = ""
		client.HTML = renderedHTML
		client.Cursor = nil
		client.Viewport = nil
		client.DocsQuery = query
		client.DocsPreviewTitle = entry.Name
		client.DocsPreviewPath = entry.RelativePath
		client.DocsPreviewAnchor = entry.Fragment
		client.DocsPreviewID = entry.ID
		client.DocsCount = len(client.DocsResults)
		client.DocsRoot = s.root
		client.DocsCache = s.cacheDir
		client.HTML = renderDocsPreviewHTML(query, entry, renderedHTML)
	})
}

func (s *DocsService) Back(sessionID string) {
	hub := globalHub
	if hub == nil {
		return
	}
	logicalSessionID := logicalSessionIDForClient(hub, sessionID)
	hub.mu.Lock()
	client := hub.clientsState[sessionID]
	if client == nil {
		hub.mu.Unlock()
		return
	}
	query := client.DocsQuery
	results := append([]docsEntry(nil), client.DocsResults...)
	hub.mu.Unlock()
	html := renderDocsResultsHTML(s.root, query, results, len(s.ensureIndex().Docsets), "")
	hub.upsertClient(sessionID, func(client *clientState) {
		client.SessionID = logicalSessionID
		client.Mode = "docs"
		client.FileType = "docs"
		client.Path = query
		client.HTML = html
		client.LineCount = 0
		client.Markdown = ""
		client.Cursor = nil
		client.Viewport = nil
		client.DocsCount = len(results)
		client.DocsRoot = s.root
		client.DocsCache = s.cacheDir
		client.DocsPreviewTitle = ""
		client.DocsPreviewPath = ""
		client.DocsPreviewAnchor = ""
		client.DocsPreviewID = ""
	})
}

func logicalSessionIDForClient(hub *Hub, sessionID string) string {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if client := hub.clientsState[sessionID]; client != nil && client.SessionID != "" {
		return client.SessionID
	}
	return sessionID
}

func (s *DocsService) loadCache() (*docsIndex, bool) {
	cachePath := filepath.Join(s.cacheDir, "doc_index.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}
	var state docsCacheState
	if err := jsonUnmarshal(data, &state); err != nil {
		return nil, false
	}
	if state.Root != s.root {
		return nil, false
	}
	idx := &docsIndex{}
	for _, cached := range state.Index {
		ds := &docsDocset{
			Root:            cached.Root,
			Name:            cached.Name,
			Version:         cached.Version,
			IndexFilePath:   cached.IndexFilePath,
			DocumentsPath:   cached.DocumentsPath,
			SearchIndexPath: cached.SearchIndexPath,
			PlatformFamily:  cached.PlatformFamily,
			Metadata:        cached.Metadata,
			Entries:         cached.Entries,
		}
		idx.Docsets = append(idx.Docsets, ds)
		idx.Entries = append(idx.Entries, cached.Entries...)
	}
	return idx, true
}

func (s *DocsService) saveCache(idx *docsIndex) error {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return err
	}
	state := docsCacheState{Root: s.root, Cache: s.cacheDir}
	for _, ds := range idx.Docsets {
		state.Index = append(state.Index, docsDocsetCacheEntry{
			Root:            ds.Root,
			Name:            ds.Name,
			Version:         ds.Version,
			IndexFilePath:   ds.IndexFilePath,
			DocumentsPath:   ds.DocumentsPath,
			SearchIndexPath: ds.SearchIndexPath,
			PlatformFamily:  ds.PlatformFamily,
			Metadata:        ds.Metadata,
			Entries:         ds.Entries,
		})
	}
	data, err := jsonMarshalIndent(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.cacheDir, "doc_index.json"), data, 0o644)
}

func (s *DocsService) scanIndex() (*docsIndex, error) {
	docsets, err := discoverDocsets(s.root)
	if err != nil {
		return nil, err
	}
	idx := &docsIndex{}
	for _, ds := range docsets {
		entries, err := loadDocsetEntries(ds)
		if err != nil {
			continue
		}
		ds.Entries = entries
		idx.Docsets = append(idx.Docsets, ds)
		idx.Entries = append(idx.Entries, entries...)
	}
	return idx, nil
}

func searchDocs(idx *docsIndex, query string, limit int) []docsEntry {
	if idx == nil || len(idx.Entries) == 0 {
		return nil
	}
	terms := splitQueryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	results := make([]docsEntry, 0, 64)
	for _, entry := range idx.Entries {
		score := scoreDocEntry(entry, terms)
		if score <= 0 {
			continue
		}
		entry.Score = score
		results = append(results, entry)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].DocsetName != results[j].DocsetName {
			return results[i].DocsetName < results[j].DocsetName
		}
		return results[i].Name < results[j].Name
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func findDocEntry(idx *docsIndex, id string) (docsEntry, bool) {
	if idx == nil {
		return docsEntry{}, false
	}
	for _, entry := range idx.Entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return docsEntry{}, false
}

func docEntryDisplayPath(entry docsEntry) string {
	if entry.RelativePath == "" {
		return entry.DocsetName
	}
	if entry.Fragment == "" {
		return filepath.ToSlash(entry.RelativePath)
	}
	return filepath.ToSlash(entry.RelativePath) + "#" + entry.Fragment
}

func splitQueryTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	return fields
}

func scoreDocEntry(entry docsEntry, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	target := strings.ToLower(strings.Join([]string{
		entry.Name,
		entry.Kind,
		entry.DocsetName,
		entry.RelativePath,
		entry.Fragment,
	}, " "))
	score := 0
	for _, term := range terms {
		if term == "" {
			continue
		}
		if strings.Contains(target, term) {
			score += 10
			if strings.HasPrefix(strings.ToLower(entry.Name), term) {
				score += 20
			}
			if strings.Contains(strings.ToLower(entry.DocsetName), term) {
				score += 5
			}
		}
	}
	if score > 0 {
		score += len(terms)
	}
	return score
}

func renderDocsEmptyHTML(query string, docsetCount int) template.HTML {
	if query == "" {
		return template.HTML(fmt.Sprintf(`
<div class="docs-shell docs-shell-empty">
  <div class="docs-toolbar">
    <div class="docs-title">Offline docs</div>
  </div>
  <div class="docs-empty">Type a query to search %d loaded docset(s).</div>
</div>`, docsetCount))
	}
	return template.HTML(fmt.Sprintf(`
<div class="docs-shell docs-shell-empty">
  <div class="docs-toolbar">
    <div class="docs-title">No results</div>
    <div class="docs-query">%s</div>
  </div>
  <div class="docs-empty">No offline docs matched your query.</div>
</div>`, template.HTMLEscapeString(query)))
}

func renderDocsResultsHTML(root, query string, results []docsEntry, docsetCount int, selectedID string) template.HTML {
	var buf bytes.Buffer
	buf.WriteString(`<div class="docs-shell docs-shell-results">`)
	buf.WriteString(`<aside class="docs-sidebar">`)
	buf.WriteString(`<div class="docs-sidebar-header">`)
	buf.WriteString(`<div class="docs-title">Offline docs</div>`)
	buf.WriteString(`<div class="docs-meta">`)
	buf.WriteString(template.HTMLEscapeString(fmt.Sprintf("%d result(s) from %d docset(s)", len(results), docsetCount)))
	buf.WriteString(`</div>`)
	buf.WriteString(`</div>`)
	buf.WriteString(`<form class="docs-search" data-doc-search-form>`)
	buf.WriteString(`<input class="docs-search-input" data-doc-search-input type="text" name="q" placeholder="Search docs" value="`)
	buf.WriteString(template.HTMLEscapeString(query))
	buf.WriteString(`" autocomplete="off">`)
	buf.WriteString(`<button type="submit">Search</button>`)
	buf.WriteString(`</form>`)
	if len(results) == 0 {
		buf.WriteString(`<div class="docs-empty">No results yet. Try a package, symbol, or API name.</div>`)
		buf.WriteString(`</aside><section class="docs-preview-pane"><div class="docs-empty docs-preview-empty">Select a result to preview it here.</div></section></div>`)
		return template.HTML(buf.String())
	}
	buf.WriteString(`<div class="docs-results">`)
	for _, entry := range results {
		className := "docs-result"
		if selectedID != "" && entry.ID == selectedID {
			className += " is-active"
		}
		buf.WriteString(`<button type="button" class="`)
		buf.WriteString(className)
		buf.WriteString(`" data-doc-open="`)
		buf.WriteString(template.HTMLEscapeString(entry.ID))
		buf.WriteString(`">`)
		buf.WriteString(`<div class="docs-result-name">`)
		buf.WriteString(template.HTMLEscapeString(entry.Name))
		buf.WriteString(`</div>`)
		buf.WriteString(`<div class="docs-result-meta">`)
		if entry.DocsetName != "" {
			buf.WriteString(template.HTMLEscapeString(entry.DocsetName))
		}
		if entry.RelativePath != "" {
			buf.WriteString(` · `)
			buf.WriteString(template.HTMLEscapeString(docEntryDisplayPath(entry)))
		}
		if entry.Kind != "" {
			buf.WriteString(` · `)
			buf.WriteString(template.HTMLEscapeString(entry.Kind))
		}
		buf.WriteString(`</div></button>`)
	}
	buf.WriteString(`</div></aside><section class="docs-preview-pane">`)
	buf.WriteString(`<div class="docs-empty docs-preview-empty">Select a result to preview it here.</div>`)
	buf.WriteString(`</section></div>`)
	return template.HTML(buf.String())
}

func renderDocsPreviewHTML(query string, entry docsEntry, rendered template.HTML) template.HTML {
	var buf bytes.Buffer
	buf.WriteString(`<div class="docs-shell docs-shell-preview">`)
	buf.WriteString(`<aside class="docs-sidebar">`)
	buf.WriteString(`<div class="docs-sidebar-header">`)
	buf.WriteString(`<button type="button" class="docs-back" data-doc-back="true">Back</button>`)
	buf.WriteString(`<div class="docs-title">`)
	buf.WriteString(`Offline docs`)
	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="docs-meta">`)
	buf.WriteString(template.HTMLEscapeString(query))
	if entry.RelativePath != "" {
		buf.WriteString(` · `)
		buf.WriteString(template.HTMLEscapeString(docEntryDisplayPath(entry)))
	}
	buf.WriteString(`</div></div>`)
	buf.WriteString(`<div class="docs-result is-active">`)
	buf.WriteString(`<div class="docs-result-name">`)
	buf.WriteString(template.HTMLEscapeString(entry.Name))
	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="docs-result-meta">`)
	if entry.DocsetName != "" {
		buf.WriteString(template.HTMLEscapeString(entry.DocsetName))
	}
	if entry.RelativePath != "" {
		buf.WriteString(` · `)
		buf.WriteString(template.HTMLEscapeString(docEntryDisplayPath(entry)))
	}
	if entry.Kind != "" {
		buf.WriteString(` · `)
		buf.WriteString(template.HTMLEscapeString(entry.Kind))
	}
	buf.WriteString(`</div></div>`)
	buf.WriteString(`</aside>`)
	buf.WriteString(`<section class="docs-preview-pane">`)
	buf.WriteString(`<button type="button" class="docs-back" data-doc-back="true">Back</button>`)
	buf.WriteString(`<div class="docs-preview-header">`)
	buf.WriteString(`<div class="docs-title">`)
	buf.WriteString(template.HTMLEscapeString(entry.Name))
	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="docs-meta">`)
	buf.WriteString(template.HTMLEscapeString(entry.DocsetName))
	if entry.RelativePath != "" {
		buf.WriteString(` · `)
		buf.WriteString(template.HTMLEscapeString(docEntryDisplayPath(entry)))
	}
	buf.WriteString(`</div></div>`)
	buf.WriteString(`<iframe class="docs-preview-frame" data-doc-preview-frame="true" srcdoc="`)
	buf.WriteString(template.HTMLEscapeString(string(rendered)))
	buf.WriteString(`" sandbox="allow-same-origin allow-popups allow-forms allow-scripts"></iframe>`)
	buf.WriteString(`</section></div>`)
	return template.HTML(buf.String())
}

func renderDocEntryHTML(entry docsEntry) (template.HTML, error) {
	path := entry.DocsetPath
	if path == "" {
		return "", fmt.Errorf("missing document path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(path))
	baseDir := filepath.Dir(path)
	switch ext {
	case ".md", ".markdown", ".mdown":
		return template.HTML(wrapDocHTML(string(renderMarkdownHTML(string(data), baseDir, nil)), fileURL(baseDir))), nil
	default:
		body := string(data)
		return template.HTML(injectBaseHref(body, fileURL(filepath.Dir(path)))), nil
	}
}

func wrapDocHTML(body, baseHref string) string {
	var buf bytes.Buffer
	buf.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	if baseHref != "" {
		buf.WriteString(`<base href="`)
		buf.WriteString(template.HTMLEscapeString(baseHref))
		buf.WriteString(`">`)
	}
	buf.WriteString("</head><body>")
	buf.WriteString(body)
	buf.WriteString("</body></html>")
	return buf.String()
}

func injectBaseHref(htmlSource, baseHref string) string {
	if baseHref == "" {
		return htmlSource
	}
	lower := strings.ToLower(htmlSource)
	baseTag := `<base href="` + template.HTMLEscapeString(baseHref) + `">`
	if strings.Contains(lower, "<base ") {
		return htmlSource
	}
	if idx := strings.Index(lower, "<head"); idx >= 0 {
		if end := strings.Index(lower[idx:], ">"); end >= 0 {
			pos := idx + end + 1
			return htmlSource[:pos] + baseTag + htmlSource[pos:]
		}
	}
	return wrapDocHTML(htmlSource, baseHref)
}

func fileURL(dir string) string {
	if dir == "" {
		return ""
	}
	abs := dir
	if !filepath.IsAbs(abs) {
		abs, _ = filepath.Abs(abs)
	}
	abs = filepath.ToSlash(abs)
	if len(abs) >= 2 && abs[1] == ':' {
		return "file:///" + abs + "/"
	}
	return "file://" + abs + "/"
}

func discoverDocsets(root string) ([]*docsDocset, error) {
	var docsets []*docsDocset
	seen := make(map[string]struct{})
	visit := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		ds, err := loadDocset(path)
		if err == nil {
			docsets = append(docsets, ds)
		}
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".docset") {
		visit(root)
		return docsets, nil
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".docset") {
			visit(path)
		}
		return nil
	})
	return docsets, nil
}

func loadDocset(root string) (*docsDocset, error) {
	infoPlist := filepath.Join(root, "Contents", "Info.plist")
	searchIndex := filepath.Join(root, "Contents", "Resources", "docSet.dsidx")
	documentsPath := filepath.Join(root, "Contents", "Resources", "Documents")
	meta, _ := parseInfoPlist(infoPlist)
	if _, err := os.Stat(searchIndex); err != nil {
		return nil, err
	}
	name := meta["CFBundleName"]
	if name == "" {
		name = filepath.Base(root)
	}
	return &docsDocset{
		Root:            root,
		Name:            name,
		Version:         meta["CFBundleVersion"],
		IndexFilePath:   meta["dashIndexFilePath"],
		DocumentsPath:   documentsPath,
		SearchIndexPath: searchIndex,
		PlatformFamily:  meta["DocSetPlatformFamily"],
		Metadata:        meta,
	}, nil
}

func loadDocsetEntries(ds *docsDocset) ([]docsEntry, error) {
	db, err := sql.Open("sqlite", sqliteDSN(ds.SearchIndexPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name, type, path FROM searchIndex`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []docsEntry
	for rows.Next() {
		var name, typ, path string
		if err := rows.Scan(&name, &typ, &path); err != nil {
			continue
		}
		if name == "" && path == "" {
			continue
		}
		relativePath, fragment := splitDocPath(path)
		absPath := filepath.Join(ds.DocumentsPath, filepath.FromSlash(relativePath))
		if _, err := os.Stat(absPath); err != nil {
			absPath = filepath.Join(ds.DocumentsPath, relativePath)
		}
		id := encodeDocEntryID(ds.Name, relativePath, fragment)
		entries = append(entries, docsEntry{
			ID:           id,
			Name:         name,
			Kind:         typ,
			DocsetName:   ds.Name,
			DocsetPath:   absPath,
			DocsetRoot:   ds.Root,
			RelativePath: relativePath,
			Fragment:     fragment,
		})
	}
	return entries, rows.Err()
}

func sqliteDSN(path string) string {
	abs := path
	if !filepath.IsAbs(abs) {
		abs, _ = filepath.Abs(abs)
	}
	abs = filepath.ToSlash(abs)
	return "file:" + abs + "?mode=ro"
}

func splitDocPath(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if strings.Contains(raw, "#") {
		parts := strings.SplitN(raw, "#", 2)
		return parts[0], parts[1]
	}
	return raw, ""
}

func encodeDocEntryID(docsetName, relativePath, fragment string) string {
	parts := []string{url.QueryEscape(docsetName), url.QueryEscape(relativePath), url.QueryEscape(fragment)}
	return strings.Join(parts, "::")
}

func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func parseInfoPlist(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	result := make(map[string]string)
	var currentKey string
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			value, err := readElementText(decoder, "key")
			if err != nil {
				return result, err
			}
			currentKey = value
		case "string":
			if currentKey == "" {
				_, _ = readElementText(decoder, "string")
				continue
			}
			value, err := readElementText(decoder, "string")
			if err != nil {
				return result, err
			}
			result[currentKey] = value
			currentKey = ""
		case "true":
			if currentKey != "" {
				result[currentKey] = "true"
				currentKey = ""
			}
		case "false":
			if currentKey != "" {
				result[currentKey] = "false"
				currentKey = ""
			}
		}
	}
	return result, nil
}

func readElementText(decoder *xml.Decoder, element string) (string, error) {
	var buf strings.Builder
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch node := tok.(type) {
		case xml.CharData:
			buf.WriteString(string(node))
		case xml.EndElement:
			if node.Name.Local == element {
				return strings.TrimSpace(buf.String()), nil
			}
		}
	}
}
