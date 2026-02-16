// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package repomap builds a ranked repository map for LLM context.
// Implements: prd006-repo-map R1, R2, R3, R4, R5;
//
//	docs/ARCHITECTURE § Repository Map.
package repomap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// langSpec holds the tree-sitter language and query patterns for a file type.
type langSpec struct {
	lang    *sitter.Language
	defQ    string // Tree-sitter query for definitions (capture @name)
	refQ    string // Tree-sitter query for references (capture @ref)
	sigQ    string // Optional query for signatures (capture @sig)
}

// supportedLangs maps file extensions to their langSpec.
var supportedLangs = map[string]*langSpec{
	".go": {
		lang: golang.GetLanguage(),
		defQ: `
			(function_declaration name: (identifier) @name)
			(method_declaration name: (field_identifier) @name)
			(type_declaration (type_spec name: (type_identifier) @name))
		`,
		refQ: `
			(identifier) @ref
			(field_identifier) @ref
			(type_identifier) @ref
		`,
	},
	".py": {
		lang: python.GetLanguage(),
		defQ: `
			(function_definition name: (identifier) @name)
			(class_definition name: (identifier) @name)
		`,
		refQ: `
			(identifier) @ref
		`,
	},
	".js": {
		lang: javascript.GetLanguage(),
		defQ: `
			(function_declaration name: (identifier) @name)
			(class_declaration name: (identifier) @name)
			(variable_declarator name: (identifier) @name)
		`,
		refQ: `
			(identifier) @ref
		`,
	},
	".ts": {
		lang: typescript.GetLanguage(),
		defQ: `
			(function_declaration name: (identifier) @name)
			(class_declaration name: (identifier) @name)
			(variable_declarator name: (identifier) @name)
			(interface_declaration name: (type_identifier) @name)
		`,
		refQ: `
			(identifier) @ref
			(type_identifier) @ref
		`,
	},
	".yaml": {
		lang: yaml.GetLanguage(),
		defQ: `
			(block_mapping_pair key: (flow_node) @name)
		`,
		refQ: "",
	},
	".yml": {
		lang: yaml.GetLanguage(),
		defQ: `
			(block_mapping_pair key: (flow_node) @name)
		`,
		refQ: "",
	},
}

// cacheEntry stores extraction results keyed by file path and mod time.
type cacheEntry struct {
	modTime time.Time
	symbols []types.SymbolRef
}

// Extractor extracts symbols from source files using tree-sitter.
//
// Implements: prd006-repo-map R1, R5.
type Extractor struct {
	mu    sync.Mutex
	cache map[string]cacheEntry
	stats ExtractStats
}

// ExtractStats tracks extraction statistics.
type ExtractStats struct {
	FilesProcessed int
	FilesSkipped   int
	CacheHits      int
	ParseCount     int
}

// NewExtractor creates a new symbol extractor with an empty cache.
func NewExtractor() *Extractor {
	return &Extractor{
		cache: make(map[string]cacheEntry),
	}
}

// ExtractAll walks the directory and extracts symbols from all supported files.
// Returns the combined symbol list and extraction statistics.
//
// Implements: prd006-repo-map R1.1-R1.6.
func (e *Extractor) ExtractAll(ctx context.Context, workDir string) ([]types.SymbolRef, ExtractStats, error) {
	e.mu.Lock()
	e.stats = ExtractStats{}
	e.mu.Unlock()

	var allSymbols []types.SymbolRef

	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we cannot stat.
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		spec, ok := supportedLangs[ext]
		if !ok {
			e.mu.Lock()
			e.stats.FilesSkipped++
			e.mu.Unlock()
			return nil
		}

		relPath, _ := filepath.Rel(workDir, path)
		symbols, err := e.extractFile(ctx, path, relPath, info.ModTime(), spec)
		if err != nil {
			// R1.6: Skip files that fail to parse.
			e.mu.Lock()
			e.stats.FilesSkipped++
			e.mu.Unlock()
			return nil
		}

		e.mu.Lock()
		e.stats.FilesProcessed++
		e.mu.Unlock()

		allSymbols = append(allSymbols, symbols...)
		return nil
	})

	e.mu.Lock()
	stats := e.stats
	e.mu.Unlock()

	return allSymbols, stats, err
}

// extractFile extracts symbols from a single file, using the cache if possible.
//
// Implements: prd006-repo-map R5.1-R5.3.
func (e *Extractor) extractFile(ctx context.Context, absPath, relPath string, modTime time.Time, spec *langSpec) ([]types.SymbolRef, error) {
	e.mu.Lock()
	if cached, ok := e.cache[relPath]; ok && cached.modTime.Equal(modTime) {
		e.stats.CacheHits++
		result := cached.symbols
		e.mu.Unlock()
		return result, nil
	}
	e.mu.Unlock()

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	symbols := e.parseSymbols(ctx, content, relPath, spec)

	e.mu.Lock()
	e.stats.ParseCount++
	e.cache[relPath] = cacheEntry{modTime: modTime, symbols: symbols}
	e.mu.Unlock()

	return symbols, nil
}

// parseSymbols runs tree-sitter queries to extract definitions and references.
func (e *Extractor) parseSymbols(ctx context.Context, content []byte, relPath string, spec *langSpec) []types.SymbolRef {
	root, err := sitter.ParseCtx(ctx, content, spec.lang)
	if err != nil || root == nil {
		return nil
	}

	var symbols []types.SymbolRef

	// Extract definitions.
	if spec.defQ != "" {
		defs := runQuery(spec.defQ, spec.lang, root, content)
		for _, d := range defs {
			symbols = append(symbols, types.SymbolRef{
				Name:     d.name,
				FilePath: relPath,
				Line:     d.line,
				Kind:     types.Definition,
			})
		}
	}

	// Extract references.
	if spec.refQ != "" {
		refs := runQuery(spec.refQ, spec.lang, root, content)
		// Deduplicate references that overlap with definitions.
		defSet := make(map[string]bool)
		for _, s := range symbols {
			defSet[s.Name] = true
		}
		for _, r := range refs {
			if !defSet[r.name] {
				symbols = append(symbols, types.SymbolRef{
					Name:     r.name,
					FilePath: relPath,
					Line:     r.line,
					Kind:     types.Reference,
				})
			}
		}
	}

	return symbols
}

// queryResult holds a captured symbol name and its location.
type queryResult struct {
	name string
	line int
}

// runQuery executes a tree-sitter query and returns captured names with locations.
func runQuery(pattern string, lang *sitter.Language, root *sitter.Node, content []byte) []queryResult {
	q, err := sitter.NewQuery([]byte(pattern), lang)
	if err != nil {
		return nil
	}

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	seen := make(map[string]bool) // Deduplicate by name+line.
	var results []queryResult

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			name := c.Node.Content(content)
			line := int(c.Node.StartPoint().Row) + 1 // 0-based to 1-based
			key := name + ":" + strings.Repeat("0", line) // Simple dedup key.
			if name == "" || seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, queryResult{name: name, line: line})
		}
	}

	return results
}

// getSignature returns the full line of the symbol for rendering.
func getSignature(content []byte, line int) string {
	lines := strings.Split(string(content), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	sig := strings.TrimSpace(lines[line-1])
	if len(sig) > 100 {
		sig = sig[:97] + "..."
	}
	return sig
}
