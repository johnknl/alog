// Copyright (c) 2026 John Kleijn
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type markerType string

const (
	markerExample markerType = "EXAMPLE"
)

var markerPattern = regexp.MustCompile(`^<!-- (EXAMPLE):([[:alnum:]_./=-]+):(start|end) -->$`)

type marker struct {
	mType       markerType
	name        string
	kind        string
	insertStart int
	start       int
}

type edit struct {
	start       int
	end         int
	replacement string
}

type exampleContent struct {
	name      string
	body      string
	docLead   string
	docDetail string
	h2        string
	h3        string
	isMethod  bool
	goRefURL  string
	goRefID   string
}

func main() {
	var root string
	var check bool

	flag.StringVar(&root, "root", "./docs", "docs root")
	flag.BoolVar(&check, "check", false, "fail if files would change")
	flag.Parse()

	replacements := make(map[string]exampleContent)

	exampleBlocks, loadErr := loadExampleBlocks(root)
	if loadErr != nil {
		die(loadErr)
	}
	for k, v := range exampleBlocks {
		replacements[string(markerExample)+":"+k] = v
	}

	mdFiles, err := collectMarkdownFiles(root)
	if err != nil {
		die(err)
	}

	changed := make([]string, 0)
	for _, md := range mdFiles {
		updated, fileChanged, replaceErr := replaceMarkers(md, replacements)
		if replaceErr != nil {
			die(replaceErr)
		}
		if !fileChanged {
			continue
		}

		changed = append(changed, md)
		if check {
			continue
		}

		if writeErr := os.WriteFile(md, []byte(updated), 0o644); writeErr != nil {
			die(writeErr)
		}
	}

	if check && len(changed) > 0 {
		fmt.Println("marker replacements are out of sync:")
		for _, file := range changed {
			fmt.Printf("- %s\n", file)
		}
		os.Exit(1)
	}

	for _, file := range changed {
		fmt.Printf("updated %s\n", file)
	}
}

func resolvePath(root string, p string) string {
	if filepath.IsAbs(p) {
		return p
	}

	return filepath.Join(root, p)
}

func collectMarkdownFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			if base == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	slices.Sort(files)
	return files, nil
}

func loadExampleBlocks(root string) (map[string]exampleContent, error) {
	modulePath, err := detectModulePath(root)
	if err != nil {
		return nil, err
	}

	exampleFiles, err := collectExampleFiles(root)
	if err != nil {
		return nil, err
	}

	if len(exampleFiles) == 0 {
		return nil, fmt.Errorf("no example source files found under %s", root)
	}

	blocks := make(map[string]exampleContent)
	for _, path := range exampleFiles {
		importPath := packageImportPath(root, modulePath, path)

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, src, parser.ParseComments)
		if parseErr != nil {
			return nil, parseErr
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*goast.FuncDecl)
			if !ok {
				continue
			}

			name := fn.Name.Name
			if !strings.HasPrefix(name, "Example") {
				continue
			}

			block, blockErr := buildExampleBlock(src, fset, fn, importPath)
			if blockErr != nil {
				return nil, blockErr
			}

			blocks[name] = block
		}
	}

	return blocks, nil
}

func buildExampleBlock(src []byte, fset *token.FileSet, fn *goast.FuncDecl, importPath string) (exampleContent, error) {
	if fn.Body == nil {
		return exampleContent{}, fmt.Errorf("example %s has no body", fn.Name.Name)
	}

	start := fset.Position(fn.Body.Lbrace).Offset + 1
	end := fset.Position(fn.Body.Rbrace).Offset
	if start < 0 || end < start || end > len(src) {
		return exampleContent{}, fmt.Errorf("invalid function body bounds for %s", fn.Name.Name)
	}

	body := trimOuterNewlines(string(src[start:end]))
	body = trimCommonIndent(body)

	docLead := ""
	docDetail := ""
	if fn.Doc != nil {
		docLead, docDetail = splitDocLeadAndDetail(fn.Doc.Text())
		docLead = rewriteExampleDocPrefix(fn.Name.Name, docLead)
	}

	h2, h3, isMethod := parseExampleHeading(fn.Name.Name)
	goRefURL, goRefID := buildGoRef(importPath, h2, h3, isMethod)

	return exampleContent{
		name:      fn.Name.Name,
		body:      body,
		docLead:   docLead,
		docDetail: docDetail,
		h2:        h2,
		h3:        h3,
		isMethod:  isMethod,
		goRefURL:  goRefURL,
		goRefID:   goRefID,
	}, nil
}

func detectModulePath(root string) (string, error) {
	goModPath := filepath.Join(root, "go.mod")
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}

		return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
	}

	return "", nil
}

func packageImportPath(root string, modulePath string, sourcePath string) string {
	if modulePath == "" {
		return ""
	}

	rel, err := filepath.Rel(root, filepath.Dir(sourcePath))
	if err != nil {
		return ""
	}

	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return modulePath
	}

	return modulePath + "/" + rel
}

func buildGoRef(importPath string, h2 string, h3 string, isMethod bool) (string, string) {
	if importPath == "" {
		return "", ""
	}

	anchor := h2
	if isMethod && h2 != "" && h3 != "" {
		anchor = h2 + "." + h3
	}
	if anchor == "" {
		return "", ""
	}

	return "https://pkg.go.dev/" + importPath + "#" + anchor, anchor
}

func splitDocLeadAndDetail(raw string) (string, string) {
	doc := strings.TrimSpace(raw)
	if doc == "" {
		return "", ""
	}

	lines := strings.Split(doc, "\n")
	lead := ""
	idx := 0
	for idx < len(lines) {
		line := strings.TrimSpace(lines[idx])
		if line != "" {
			lead = line
			idx++
			break
		}
		idx++
	}

	if lead == "" {
		return "", ""
	}

	detail := strings.TrimSpace(strings.Join(lines[idx:], "\n"))
	return lead, detail
}

func rewriteExampleDocPrefix(name string, doc string) string {
	if name == "" || doc == "" {
		return doc
	}

	if strings.HasPrefix(doc, name) {
		return "The following example" + doc[len(name):]
	}

	return doc
}

func parseExampleHeading(name string) (string, string, bool) {
	if !strings.HasPrefix(name, "Example") {
		return "", "", false
	}

	rest := strings.TrimPrefix(name, "Example")
	if rest == "" {
		return "", "", false
	}

	parts := strings.Split(rest, "_")
	primary := parts[0]
	if primary == "" {
		return "", "", false
	}

	if len(parts) > 1 && isExportedIdent(primary) && isExportedIdent(parts[1]) {
		return primary, parts[1], true
	}

	return primary, "", false
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}

	r := []rune(name)[0]
	return r >= 'A' && r <= 'Z'
}

func trimOuterNewlines(s string) string {
	s = strings.TrimPrefix(s, "\r\n")
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(s, "\r\n")
	s = strings.TrimSuffix(s, "\n")

	return s
}

func trimCommonIndent(s string) string {
	lines := strings.Split(s, "\n")
	common := ""
	hasContent := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		hasContent = true
		indentLen := 0
		for indentLen < len(line) {
			if line[indentLen] != ' ' && line[indentLen] != '\t' {
				break
			}
			indentLen++
		}

		indent := line[:indentLen]
		if common == "" {
			common = indent
			continue
		}

		common = sharedIndentPrefix(common, indent)
		if common == "" {
			break
		}
	}

	if !hasContent || common == "" {
		return s
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, common) {
			lines[i] = line[len(common):]
		}
	}

	return strings.Join(lines, "\n")
}

func sharedIndentPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}

	return a[:i]
}

func collectExampleFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			if base == "bin" || base == ".tmp" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, "examples_test.go") {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.Sort(files)
	return files, nil
}

func replaceMarkers(path string, replacements map[string]exampleContent) (string, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	markers, err := collectMarkers(raw)
	if err != nil {
		return "", false, fmt.Errorf("parse markers in %s: %w", path, err)
	}

	edits := make([]edit, 0)
	stack := make(map[string]marker)
	seenH2 := make(map[string]bool)
	pageH1 := collectPageH1(string(raw))
	for _, m := range markers {
		if m.mType != markerExample {
			continue
		}

		key := string(m.mType) + ":" + m.name
		if m.kind == "start" {
			stack[key] = m
			continue
		}

		start, ok := stack[key]
		if !ok {
			return "", false, fmt.Errorf("missing start marker for %s in %s", key, path)
		}
		delete(stack, key)

		replacement, ok := replacements[key]
		if !ok {
			return "", false, fmt.Errorf("missing replacement content for %s used in %s", key, path)
		}

		rendered := renderExampleReplacement(replacement, seenH2, pageH1)

		edits = append(edits, edit{
			start:       start.insertStart,
			end:         m.start,
			replacement: rendered + "\n",
		})
	}

	if len(stack) > 0 {
		for key := range stack {
			return "", false, fmt.Errorf("missing end marker for %s in %s", key, path)
		}
	}

	if len(edits) == 0 {
		return string(raw), false, nil
	}

	updated, changed := applyEdits(raw, edits)
	return string(updated), changed, nil
}

func collectPageH1(md string) string {
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}

		heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		if heading != "" {
			return heading
		}
	}

	return ""
}

func renderExampleReplacement(example exampleContent, seenH2 map[string]bool, pageH1 string) string {
	parts := make([]string, 0, 5)
	typeInPageH1 := matchesHeading(example.h2, pageH1)

	if example.h2 != "" {
		if !example.isMethod {
			if !typeInPageH1 && !seenH2[example.h2] {
				parts = append(parts, "## "+example.h2)
				seenH2[example.h2] = true
			}
		} else {
			if !typeInPageH1 && !seenH2[example.h2] {
				parts = append(parts, "## "+example.h2)
				seenH2[example.h2] = true
			}
			if example.h3 != "" {
				if typeInPageH1 {
					parts = append(parts, "## "+example.h3)
				} else {
					parts = append(parts, "### "+example.h3)
				}
			}
		}
	}

	if example.docDetail != "" {
		parts = append(parts, example.docDetail)
	}

	if example.goRefURL != "" && example.goRefID != "" {
		parts = append(parts, "Go reference: ["+example.goRefID+"]("+example.goRefURL+").")
	}

	if example.docLead != "" {
		parts = append(parts, example.docLead)
	}

	parts = append(parts, "```go\n"+example.body+"\n```")

	return strings.Join(parts, "\n\n")
}

func matchesHeading(a string, b string) bool {
	if a == "" || b == "" {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func collectMarkers(raw []byte) ([]marker, error) {
	parser := goldmark.DefaultParser()
	doc := parser.Parse(text.NewReader(raw))

	markers := make([]marker, 0)
	err := goldast.Walk(doc, func(node goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}

		html, ok := node.(*goldast.HTMLBlock)
		if !ok {
			return goldast.WalkContinue, nil
		}

		lines := html.Lines()
		if lines.Len() != 1 {
			return goldast.WalkContinue, nil
		}

		segment := lines.At(0)
		line := strings.TrimSpace(string(segment.Value(raw)))
		match := markerPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			return goldast.WalkContinue, nil
		}

		insertStart := segment.Stop
		for insertStart < len(raw) && (raw[insertStart] == '\n' || raw[insertStart] == '\r') {
			insertStart++
		}

		markers = append(markers, marker{
			mType:       markerType(match[1]),
			name:        match[2],
			kind:        match[3],
			insertStart: insertStart,
			start:       segment.Start,
		})

		return goldast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(markers, func(a, b marker) int {
		switch {
		case a.start < b.start:
			return -1
		case a.start > b.start:
			return 1
		default:
			return 0
		}
	})

	return markers, nil
}

func applyEdits(raw []byte, edits []edit) ([]byte, bool) {
	slices.SortFunc(edits, func(a, b edit) int {
		switch {
		case a.start < b.start:
			return 1
		case a.start > b.start:
			return -1
		default:
			return 0
		}
	})

	updated := raw
	changed := false
	for _, e := range edits {
		if e.start < 0 || e.end < e.start || e.end > len(updated) {
			continue
		}

		existing := updated[e.start:e.end]
		replacement := []byte(e.replacement)
		if bytes.Equal(existing, replacement) {
			continue
		}

		changed = true
		updated = append(updated[:e.start], append(replacement, updated[e.end:]...)...)
	}

	return updated, changed
}

func die(err error) {
	if err == nil {
		return
	}

	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
		fmt.Fprintf(os.Stderr, "docsync: %s\n", pathErr)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "docsync: %v\n", err)
	os.Exit(1)
}
