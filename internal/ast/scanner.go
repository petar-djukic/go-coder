// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package ast provides Go source file scanning, parsing, and symbol extraction.
// Implements: prd002-ast-engine R1 (File Scanner), R2 (Symbol Table);
//
//	docs/ARCHITECTURE.md § File Scanner, AST Engine.
package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// skipDirs contains directory names that ScanDir skips by default.
var skipDirs = map[string]bool{
	"vendor":   true,
	".git":     true,
	"testdata": true,
	"node_modules": true,
}

// ScanResult holds the output of a directory scan.
type ScanResult struct {
	FileSet *token.FileSet
	Files   map[string]*ast.File
	Errors  []ScanError
}

// ScanError records a parse failure for a single file.
type ScanError struct {
	FilePath string
	Err      error
}

func (e ScanError) Error() string {
	return fmt.Sprintf("%s: %v", e.FilePath, e.Err)
}

// ScanDir walks the directory tree rooted at dir, finds all .go files,
// and parses them in parallel using a bounded worker pool.
//
// It skips vendor/, .git/, and testdata/ directories. It respects
// .gitignore patterns found in the root directory.
//
// Parse errors for individual files are collected in ScanResult.Errors
// but do not abort the scan. The concurrency parameter controls the
// number of parallel parser goroutines; if <= 0 it defaults to
// runtime.NumCPU().
func ScanDir(dir string, concurrency int) (*ScanResult, error) {
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absDir)
	}

	ignorer := loadGitignore(absDir)

	// Collect all .go file paths.
	var paths []string
	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] && path != absDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		relPath, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			relPath = path
		}
		if ignorer.isIgnored(relPath) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	fset := token.NewFileSet()
	result := &ScanResult{
		FileSet: fset,
		Files:   make(map[string]*ast.File, len(paths)),
	}

	if len(paths) == 0 {
		return result, nil
	}

	// Parse files using a bounded worker pool.
	type parseResult struct {
		path string
		file *ast.File
		err  error
	}

	jobs := make(chan string, len(paths))
	results := make(chan parseResult, len(paths))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				f, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
				results <- parseResult{path: path, file: f, err: parseErr}
			}
		}()
	}

	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for pr := range results {
		relPath, relErr := filepath.Rel(absDir, pr.path)
		if relErr != nil {
			relPath = pr.path
		}

		if pr.err != nil {
			result.Errors = append(result.Errors, ScanError{FilePath: relPath, Err: pr.err})
			// Even with errors, go/parser may return a partial AST.
			if pr.file != nil {
				result.Files[relPath] = pr.file
			}
			continue
		}
		result.Files[relPath] = pr.file
	}

	return result, nil
}

// gitignorer provides simple .gitignore matching.
type gitignorer struct {
	patterns []string
}

// loadGitignore reads .gitignore from the root directory. If no .gitignore
// exists or it cannot be read, returns an ignorer that matches nothing.
func loadGitignore(root string) gitignorer {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return gitignorer{}
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return gitignorer{patterns: patterns}
}

// isIgnored checks whether a relative path matches any .gitignore pattern.
// This implements a simplified subset of gitignore: directory prefixes and
// simple glob patterns via filepath.Match.
func (g gitignorer) isIgnored(relPath string) bool {
	for _, pattern := range g.patterns {
		// Strip trailing slash for directory patterns.
		dirPattern := strings.TrimSuffix(pattern, "/")

		// Check if any path component matches.
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if matched, _ := filepath.Match(dirPattern, part); matched {
				return true
			}
		}

		// Check full path match.
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}
