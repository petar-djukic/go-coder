// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFixtures creates the test fixture directory structure used by
// the scanner tests. Returns the root path and a cleanup function.
func setupFixtures(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Main package file.
	writeFixture(t, root, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	// Library files.
	writeFixture(t, root, "lib/helper.go", `package lib

import "context"

// HelperFunc does something useful.
func HelperFunc(ctx context.Context, id string) (*Result, error) {
	return nil, nil
}

// Result holds operation output.
type Result struct {
	Data string
}
`)

	writeFixture(t, root, "lib/types.go", `package lib

// Config holds configuration settings.
type Config struct {
	Name    string
	Timeout int
}

// Processor defines an operation pipeline.
type Processor interface {
	Process(input string) (string, error)
}

// DefaultName is the default configuration name.
const DefaultName = "default"

// MaxRetries limits retry attempts.
var MaxRetries = 3
`)

	// Files that should be skipped.
	writeFixture(t, root, "vendor/dep.go", `package dep
func Dep() {}
`)
	writeFixture(t, root, ".git/config.go", `package config
func GitConfig() {}
`)
	writeFixture(t, root, "testdata/nested.go", `package nested
func Nested() {}
`)

	// Broken file for error collection test.
	writeFixture(t, root, "broken.go", `package broken
func broken({
`)

	return root
}

func writeFixture(t *testing.T, root, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(root, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
}

func TestScanDir(t *testing.T) {
	root := setupFixtures(t)

	tests := []struct {
		name  string
		check func(t *testing.T, result *ScanResult)
	}{
		{
			name: "finds all .go files",
			check: func(t *testing.T, result *ScanResult) {
				assert.Contains(t, result.Files, "main.go")
				assert.Contains(t, result.Files, filepath.Join("lib", "helper.go"))
				assert.Contains(t, result.Files, filepath.Join("lib", "types.go"))
				// broken.go may have a partial AST but should be present
				// as either a file or an error.
				foundBroken := false
				if _, ok := result.Files["broken.go"]; ok {
					foundBroken = true
				}
				for _, e := range result.Errors {
					if e.FilePath == "broken.go" {
						foundBroken = true
					}
				}
				assert.True(t, foundBroken, "broken.go should appear in files or errors")
			},
		},
		{
			name: "skips vendor and .git and testdata directories",
			check: func(t *testing.T, result *ScanResult) {
				for path := range result.Files {
					assert.NotContains(t, path, "vendor")
					assert.NotContains(t, path, ".git")
					assert.NotContains(t, path, "testdata")
				}
			},
		},
		{
			name: "collects parse errors without aborting",
			check: func(t *testing.T, result *ScanResult) {
				require.NotEmpty(t, result.Errors, "should have at least one parse error")
				foundBroken := false
				for _, e := range result.Errors {
					if e.FilePath == "broken.go" {
						foundBroken = true
					}
				}
				assert.True(t, foundBroken, "broken.go should produce a parse error")
				// Other valid files should still be parsed.
				assert.GreaterOrEqual(t, len(result.Files), 3,
					"valid files should still be parsed despite errors")
			},
		},
	}

	result, err := ScanDir(root, 4)
	require.NoError(t, err)
	require.NotNil(t, result)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, result)
		})
	}
}

func TestScanDirErrors(t *testing.T) {
	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := ScanDir("/nonexistent/path/12345", 4)
		assert.Error(t, err)
	})

	t.Run("file not directory", func(t *testing.T) {
		f, err := os.CreateTemp("", "scandir-test")
		require.NoError(t, err)
		f.Close()
		defer os.Remove(f.Name())

		_, err = ScanDir(f.Name(), 4)
		assert.Error(t, err)
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		result, err := ScanDir(dir, 4)
		require.NoError(t, err)
		assert.Empty(t, result.Files)
		assert.Empty(t, result.Errors)
	})

	t.Run("default concurrency", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "hello.go", `package hello
func Hello() {}
`)
		result, err := ScanDir(dir, 0)
		require.NoError(t, err)
		assert.Len(t, result.Files, 1)
	})
}

func TestScanDirGitignore(t *testing.T) {
	root := t.TempDir()

	writeFixture(t, root, ".gitignore", "generated/\n*.pb.go\n")
	writeFixture(t, root, "main.go", "package main\nfunc main() {}\n")
	writeFixture(t, root, "generated/output.go", "package generated\nfunc Gen() {}\n")
	writeFixture(t, root, "api.pb.go", "package main\nfunc GRPC() {}\n")

	result, err := ScanDir(root, 4)
	require.NoError(t, err)

	assert.Contains(t, result.Files, "main.go")
	assert.NotContains(t, result.Files, filepath.Join("generated", "output.go"))
	assert.NotContains(t, result.Files, "api.pb.go")
}
