package main

import (
	"bytes"
	"html/template"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(&sourceLineTransformer{}, 100)),
	),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

type sourceLineTransformer struct{}

func (t *sourceLineTransformer) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.Kind() {
		case ast.KindParagraph, ast.KindHeading, ast.KindBlockquote, ast.KindList, ast.KindListItem, ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindHTMLBlock, ast.KindTextBlock, ast.KindThematicBreak:
			start, end := nodeLineRange(n, source)
			if start > 0 {
				n.SetAttributeString("data-line-start", strconv.Itoa(start))
				n.SetAttributeString("data-line-end", strconv.Itoa(end))
			}
		}

		return ast.WalkContinue, nil
	})
}

func nodeLineRange(n ast.Node, source []byte) (int, int) {
	switch block := n.(type) {
	case interface{ Lines() *text.Segments }:
		lines := block.Lines()
		if lines == nil || lines.Len() == 0 {
			return 0, 0
		}
		start := 0
		end := 0
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			if seg.Start < 0 {
				continue
			}
			line := 1 + bytes.Count(source[:seg.Start], []byte{'\n'})
			if start == 0 || line < start {
				start = line
			}
			if line > end {
				end = line
			}
		}
		if start > 0 {
			return start, end
		}
	}
	if pos := n.Pos(); pos >= 0 && pos <= len(source) {
		line := 1 + bytes.Count(source[:pos], []byte{'\n'})
		return line, line
	}
	return 0, 0
}

type imageResolver struct {
	baseDir string
}

func (r imageResolver) Resolve(dest string) string {
	raw := strings.TrimSpace(dest)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	if filepath.IsAbs(raw) {
		return filepath.ToSlash(raw)
	}
	if r.baseDir == "" {
		return filepath.ToSlash(raw)
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(r.baseDir, raw)))
}

var imageSrcPattern = regexp.MustCompile(`(<img\b[^>]*\bsrc=")([^"]+)(")`)

func rewriteImageSources(htmlSource string, baseDir string, resources map[string]string) string {
	if baseDir == "" && len(resources) == 0 {
		return htmlSource
	}

	resolver := imageResolver{baseDir: baseDir}
	return imageSrcPattern.ReplaceAllStringFunc(htmlSource, func(match string) string {
		parts := imageSrcPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		if resources != nil {
			if dataURI, ok := resources[parts[2]]; ok && dataURI != "" {
				return parts[1] + template.HTMLEscapeString(dataURI) + parts[3]
			}
		}
		resolved := resolver.Resolve(parts[2])
		if resources != nil {
			if dataURI, ok := resources[resolved]; ok && dataURI != "" {
				return parts[1] + template.HTMLEscapeString(dataURI) + parts[3]
			}
		}
		if resolved == "" || resolved == parts[2] {
			return match
		}
		return parts[1] + template.HTMLEscapeString(resolved) + parts[3]
	})
}

func renderMarkdownHTML(markdown, baseDir string, resources map[string]string) template.HTML {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(markdown), &buf); err != nil {
		return template.HTML("<pre class=\"error\">render failed</pre>")
	}
	return template.HTML(rewriteImageSources(buf.String(), baseDir, resources))
}
