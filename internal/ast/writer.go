// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
)

// WriteFile renders an *ast.File back to disk using go/format.Node,
// producing gofmt-compliant output. It uses an atomic write strategy:
// write to a temp file in the same directory, then rename. This prevents
// corruption from partial writes.
//
// The original file's permissions are preserved. If the file does not
// exist yet, permissions default to 0644.
//
// Implements: prd002-ast-engine R4.1, R4.2, R4.3, R4.4.
func WriteFile(fset *token.FileSet, file *ast.File, path string) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("formatting AST: %w", err)
	}

	// Determine file permissions from existing file, or default.
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Atomic write: temp file then rename.
	tmp, err := os.CreateTemp(dir, ".go-coder-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Clean up temp file on any error.
	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}

	success = true
	return nil
}

// FormatFile renders an *ast.File to a byte slice using go/format.Node.
// This is useful for in-memory formatting without writing to disk.
func FormatFile(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("formatting AST: %w", err)
	}
	return buf.Bytes(), nil
}
