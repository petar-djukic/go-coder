// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-edit-engine R1, R2, R3, R4, R5;
//
//	docs/ARCHITECTURE § Text Editor.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// TextEditor applies search/replace edits to non-Go text files using
// multi-stage matching. It implements the types.Applier interface.
type TextEditor struct {
	// FuzzyThreshold is the minimum similarity score for fuzzy matching.
	// Defaults to 0.8 if zero.
	FuzzyThreshold float64
}

// Verify interface compliance at compile time.
var _ types.Applier = (*TextEditor)(nil)

// Apply applies a single edit to the target file. For file creation edits
// (IsCreate=true), it creates the file. For search/replace edits, it reads
// the file, finds the search text through multi-stage matching, performs
// the replacement, and writes the result atomically.
//
// Implements: prd003-edit-engine R1.1, R1.2, R1.4, R1.5, R5.1.
func (e *TextEditor) Apply(edit types.Edit) (*types.ApplyResult, error) {
	if edit.IsCreate {
		return e.createFile(edit)
	}

	if edit.OldContent == "" && edit.NewContent != "" {
		return e.appendFile(edit)
	}

	if edit.OldContent == "" && edit.NewContent == "" {
		return nil, fmt.Errorf("edit has empty search and replacement text for %s", edit.FilePath)
	}

	content, err := os.ReadFile(edit.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", edit.FilePath, err)
	}

	threshold := e.fuzzyThreshold()
	m := findMatch(string(content), edit.OldContent, threshold)
	if m == nil {
		return nil, e.buildDiagnostic(edit.FilePath, string(content), edit.OldContent)
	}

	// Replace the matched region with the new content.
	result := string(content[:m.start]) + edit.NewContent + string(content[m.end:])

	if err := atomicWrite(edit.FilePath, []byte(result)); err != nil {
		return nil, fmt.Errorf("writing %s: %w", edit.FilePath, err)
	}

	return &types.ApplyResult{
		FilePath:   edit.FilePath,
		Stage:      m.stage,
		Similarity: m.similarity,
	}, nil
}

// CreateFile creates a new file with the given content.
// Returns an error if the file already exists.
//
// Implements: prd003-edit-engine R4.1, R4.2.
func (e *TextEditor) createFile(edit types.Edit) (*types.ApplyResult, error) {
	if _, err := os.Stat(edit.FilePath); err == nil {
		return nil, fmt.Errorf("file already exists: %s", edit.FilePath)
	}

	dir := filepath.Dir(edit.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := atomicWrite(edit.FilePath, []byte(edit.NewContent)); err != nil {
		return nil, fmt.Errorf("creating %s: %w", edit.FilePath, err)
	}

	return &types.ApplyResult{
		FilePath:   edit.FilePath,
		Stage:      types.StageExact,
		Similarity: 1.0,
	}, nil
}

// appendFile appends content to an existing file (empty search, non-empty replace).
func (e *TextEditor) appendFile(edit types.Edit) (*types.ApplyResult, error) {
	content, err := os.ReadFile(edit.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", edit.FilePath, err)
	}

	result := string(content) + edit.NewContent
	if err := atomicWrite(edit.FilePath, []byte(result)); err != nil {
		return nil, fmt.Errorf("writing %s: %w", edit.FilePath, err)
	}

	return &types.ApplyResult{
		FilePath:   edit.FilePath,
		Stage:      types.StageExact,
		Similarity: 1.0,
	}, nil
}

// ReplaceFile overwrites an existing file with new content.
//
// Implements: prd003-edit-engine R4.3.
func ReplaceFile(path string, content []byte) error {
	return atomicWrite(path, content)
}

// DeleteFile removes a file from disk.
//
// Implements: prd003-edit-engine R4.4.
func DeleteFile(path string) error {
	return os.Remove(path)
}

// buildDiagnostic constructs a structured diagnostic when all matching
// stages fail.
//
// Implements: prd003-edit-engine R3.1-R3.4.
func (e *TextEditor) buildDiagnostic(filePath, content, search string) *types.Diagnostic {
	closest, sim, lineStart, lineEnd := findClosestMatch(content, search)
	return &types.Diagnostic{
		FilePath:         filePath,
		SearchText:       search,
		ClosestMatch:     closest,
		Similarity:       sim,
		ClosestLineStart: lineStart,
		ClosestLineEnd:   lineEnd,
	}
}

func (e *TextEditor) fuzzyThreshold() float64 {
	if e.FuzzyThreshold > 0 {
		return e.FuzzyThreshold
	}
	return defaultFuzzyThreshold
}

// atomicWrite writes data to a temp file in the same directory, then renames
// it to the target path. This prevents partial writes from corrupting files.
//
// Implements: prd003-edit-engine R1.5, R4.5.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)

	// Preserve original file permissions if the file exists.
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	f, err := os.CreateTemp(dir, ".go-coder-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// countLines returns the number of newlines before a byte offset, plus 1
// (for 1-based line numbering).
func countLines(s string, offset int) int {
	return strings.Count(s[:offset], "\n") + 1
}
