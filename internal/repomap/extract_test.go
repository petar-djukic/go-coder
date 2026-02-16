// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package repomap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAll_GoDefinitions(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"pkg/math/math.go": `package math

type Calculator struct{}

func (c *Calculator) Add(a, b int) int { return a + b }

func Multiply(a, b int) int { return a * b }
`,
		"pkg/util/format.go": `package util

func FormatNumber(n int) string { return "" }
`,
	})

	ext := NewExtractor()
	symbols, stats, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)

	assert.Equal(t, 2, stats.FilesProcessed)

	// Check definitions were extracted.
	defs := filterByKind(symbols, types.Definition)
	defNames := symbolNames(defs)

	assert.Contains(t, defNames, "Calculator")
	assert.Contains(t, defNames, "Add")
	assert.Contains(t, defNames, "Multiply")
	assert.Contains(t, defNames, "FormatNumber")
	assert.GreaterOrEqual(t, len(defs), 4)
}

func TestExtractAll_PythonDefinitions(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"app.py": `
class Calculator:
    def add(self, a, b):
        return a + b

def multiply(a, b):
    return a * b
`,
	})

	ext := NewExtractor()
	symbols, stats, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.FilesProcessed)

	defs := filterByKind(symbols, types.Definition)
	defNames := symbolNames(defs)
	assert.Contains(t, defNames, "Calculator")
	assert.Contains(t, defNames, "add")
	assert.Contains(t, defNames, "multiply")
}

func TestExtractAll_UnsupportedFilesSkipped(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"main.go": `package main

func Add(a, b int) int { return a + b }
`,
		"logo.png": "binary data",
		"data.bin": "binary data",
	})

	ext := NewExtractor()
	_, stats, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.FilesProcessed)
	assert.Equal(t, 2, stats.FilesSkipped)
}

func TestExtractAll_CacheSkipsUnchangedFiles(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"main.go": `package main

func Hello() string { return "hello" }
`,
	})

	ext := NewExtractor()

	// First extraction: parses the file.
	_, stats1, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, stats1.ParseCount)
	assert.Equal(t, 0, stats1.CacheHits)

	// Second extraction without changes: should hit cache.
	_, stats2, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 0, stats2.ParseCount)
	assert.Equal(t, 1, stats2.CacheHits)
}

func TestExtractAll_CacheInvalidatedOnChange(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"main.go": `package main

func Hello() string { return "hello" }
`,
	})

	ext := NewExtractor()

	_, _, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)

	// Modify the file (need to change mod time).
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "main.go"),
		[]byte("package main\n\nfunc Goodbye() string { return \"bye\" }\n"),
		0o644,
	))

	_, stats, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.ParseCount, "modified file should be re-parsed")
}

func TestExtractAll_ContextCancellation(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"main.go": `package main

func Hello() {}
`,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ext := NewExtractor()
	_, _, err := ext.ExtractAll(ctx, dir)
	assert.Error(t, err)
}

func TestExtractAll_GitDirSkipped(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"main.go": `package main

func Hello() {}
`,
	})
	// Create a .git directory with a Go file inside.
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "internal.go"), []byte("package internal\n"), 0o644))

	ext := NewExtractor()
	_, stats, err := ext.ExtractAll(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.FilesProcessed, "should only process main.go, not .git files")
}

// --- Test helpers ---

func setupTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return dir
}

func filterByKind(symbols []types.SymbolRef, kind types.RefKind) []types.SymbolRef {
	var result []types.SymbolRef
	for _, s := range symbols {
		if s.Kind == kind {
			result = append(result, s)
		}
	}
	return result
}

func symbolNames(symbols []types.SymbolRef) []string {
	var names []string
	for _, s := range symbols {
		names = append(names, s.Name)
	}
	return names
}
