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

func main() {
	var root string
	var check bool

	flag.StringVar(&root, "root", "./docs", "docs root")
	flag.BoolVar(&check, "check", false, "fail if files would change")
	flag.Parse()

	replacements := make(map[string]string)

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

func loadExampleBlocks(root string) (map[string]string, error) {
	exampleFiles, err := collectExampleFiles(root)
	if err != nil {
		return nil, err
	}

	if len(exampleFiles) == 0 {
		return nil, fmt.Errorf("no example source files found under %s", root)
	}

	blocks := make(map[string]string)
	for _, path := range exampleFiles {
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

			start := fset.Position(fn.Pos()).Offset
			end := fset.Position(fn.End()).Offset
			if start < 0 || end <= start || end > len(src) {
				return nil, fmt.Errorf("invalid function bounds for %s", name)
			}

			snippet := strings.TrimSpace(string(src[start:end]))
			blocks[name] = "```go\n" + snippet + "\n```"
		}
	}

	return blocks, nil
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

func replaceMarkers(path string, replacements map[string]string) (string, bool, error) {
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

		edits = append(edits, edit{
			start:       start.insertStart,
			end:         m.start,
			replacement: replacement + "\n",
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
