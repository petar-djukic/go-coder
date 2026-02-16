// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package coder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/petar-djukic/go-coder/pkg/types"
)

// mockPrompter implements Prompter for testing.
type mockPrompter struct {
	responses []string // Responses to return in order.
	callCount int
	usage     types.TokenUsage
}

func (m *mockPrompter) Generate(_ context.Context, _ []brtypes.SystemContentBlock, _ []brtypes.Message) (string, error) {
	if m.callCount >= len(m.responses) {
		return "", fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.callCount]
	m.callCount++
	m.usage.InputTokens += 500
	m.usage.OutputTokens += 200
	return resp, nil
}

func (m *mockPrompter) Usage() types.TokenUsage {
	return m.usage
}

func TestRunner_SuccessfulEdit(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	// Mock LLM returns an edit that adds a function.
	mock := &mockPrompter{
		responses: []string{
			fmt.Sprintf(`Here is the edit:

%s/main.go
<<<<<<< SEARCH
func main() {}
=======
func main() {}

func Hello() string { return "hello" }
>>>>>>> REPLACE
`, dir+"/"),
		},
	}

	// Use absolute path for the edit file path.
	absMainGo := filepath.Join(dir, "main.go")
	mock.responses[0] = fmt.Sprintf(`Here is the edit:

%s
<<<<<<< SEARCH
func main() {}
=======
func main() {}

func Hello() string { return "hello" }
>>>>>>> REPLACE
`, absMainGo)

	runner := NewRunner(Deps{
		Prompter:       mock,
		WorkDir:        dir,
		MaxRetries:     1,
		MapTokenBudget: 1000,
		NoGit:          true,
	})

	result, err := runner.Run(context.Background(), "add hello function")
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.NotEmpty(t, result.ModifiedFiles)
	assert.Equal(t, 700, result.TokensUsed.Total())
}

func TestRunner_ParseFailure(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	// Mock LLM returns text with no edit blocks.
	mock := &mockPrompter{
		responses: []string{
			"I'm not sure what to edit. Can you clarify?",
		},
	}

	runner := NewRunner(Deps{
		Prompter:       mock,
		WorkDir:        dir,
		MaxRetries:     1,
		MapTokenBudget: 1000,
		NoGit:          true,
	})

	_, err := runner.Run(context.Background(), "do something")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestRunner_ContextCancellation(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := NewRunner(Deps{
		Prompter:       &mockPrompter{responses: []string{"anything"}},
		WorkDir:        dir,
		MaxRetries:     1,
		MapTokenBudget: 1000,
		NoGit:          true,
	})

	_, err := runner.Run(ctx, "add feature")
	assert.Error(t, err)
}

func TestRunner_TokenUsageReported(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	absMainGo := filepath.Join(dir, "main.go")
	mock := &mockPrompter{
		responses: []string{
			fmt.Sprintf(`Edit:

%s
<<<<<<< SEARCH
func main() {}
=======
func main() {}

func Add(a, b int) int { return a + b }
>>>>>>> REPLACE
`, absMainGo),
		},
	}

	runner := NewRunner(Deps{
		Prompter:       mock,
		WorkDir:        dir,
		MaxRetries:     1,
		MapTokenBudget: 1000,
		NoGit:          true,
	})

	result, err := runner.Run(context.Background(), "add function")
	require.NoError(t, err)

	assert.Greater(t, result.TokensUsed.InputTokens, 0)
	assert.Greater(t, result.TokensUsed.OutputTokens, 0)
	assert.Greater(t, result.TokensUsed.Total(), 0)
}

func TestRunner_NoLLMClient(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})

	runner := NewRunner(Deps{
		WorkDir:        dir,
		MaxRetries:     1,
		MapTokenBudget: 1000,
		NoGit:          true,
	})

	_, err := runner.Run(context.Background(), "add feature")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM client")
}

func TestReadRelevantFiles(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go":    "package main\n",
		"lib/lib.go": "package lib\n",
		"data.bin":   "binary data",
	})

	files := readRelevantFiles(dir)

	// Should include .go files but not .bin.
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	assert.Contains(t, paths, "main.go")
	assert.Contains(t, paths, filepath.Join("lib", "lib.go"))

	// .bin should not be included.
	for _, p := range paths {
		assert.NotEqual(t, "data.bin", p)
	}
}

func TestReadRelevantFiles_SkipsGitDir(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": "package main\n",
	})
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("package git\n"), 0o644))

	files := readRelevantFiles(dir)
	for _, f := range files {
		assert.NotContains(t, f.Path, ".git")
	}
}

// setupGoModule creates a temp dir with go.mod and the given files.
func setupGoModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	goMod := "module testmod\n\ngo 1.25\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))

	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	return dir
}
