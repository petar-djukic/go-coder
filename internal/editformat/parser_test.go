// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package editformat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-coder/internal/editor"
	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SingleBlock(t *testing.T) {
	response := `Here is the fix:

internal/editor/apply.go
<<<<<<< SEARCH
func Apply(path string) error {
    return nil
}
=======
func Apply(path string) error {
    return applyEdit(path)
}
>>>>>>> REPLACE`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Edits))
	assert.Equal(t, 1, result.BlocksFound)
	assert.Equal(t, 1, result.BlocksParsed)
	assert.Equal(t, "internal/editor/apply.go", result.Edits[0].FilePath)
	assert.Contains(t, result.Edits[0].OldContent, "return nil")
	assert.Contains(t, result.Edits[0].NewContent, "return applyEdit(path)")
	assert.Contains(t, result.ReasoningText, "Here is the fix")
}

func TestParse_MultipleBlocks(t *testing.T) {
	response := `I will update three files:

pkg/types/edit.go
<<<<<<< SEARCH
type Edit struct{}
=======
type Edit struct {
    FilePath string
}
>>>>>>> REPLACE

internal/editor/apply.go
<<<<<<< SEARCH
return nil
=======
return applyEdit(path)
>>>>>>> REPLACE

config.yaml
<<<<<<< SEARCH
timeout: 30
=======
timeout: 60
>>>>>>> REPLACE`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Edits))
	assert.Equal(t, 3, result.BlocksFound)
	assert.Equal(t, 3, result.BlocksParsed)
	assert.Equal(t, "pkg/types/edit.go", result.Edits[0].FilePath)
	assert.Equal(t, "internal/editor/apply.go", result.Edits[1].FilePath)
	assert.Equal(t, "config.yaml", result.Edits[2].FilePath)
	assert.NotEmpty(t, result.ReasoningText)
}

func TestParse_MarkdownFences(t *testing.T) {
	response := "Here is the change:\n\n```\ninternal/editor/apply.go\n<<<<<<< SEARCH\nreturn nil\n=======\nreturn applyEdit(path)\n>>>>>>> REPLACE\n```"

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Edits))
	assert.Equal(t, "internal/editor/apply.go", result.Edits[0].FilePath)
	assert.Equal(t, "return nil\n", result.Edits[0].OldContent)
	assert.Equal(t, "return applyEdit(path)\n", result.Edits[0].NewContent)
}

func TestParse_EmptyReplacement(t *testing.T) {
	response := `file.go
<<<<<<< SEARCH
dead code
=======
>>>>>>> REPLACE`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Edits))
	assert.Equal(t, "dead code\n", result.Edits[0].OldContent)
	assert.Equal(t, "", result.Edits[0].NewContent)
}

func TestParse_EmptySearch(t *testing.T) {
	response := `file.go
<<<<<<< SEARCH
=======
new content
>>>>>>> REPLACE`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result.Edits))
	assert.Equal(t, "", result.Edits[0].OldContent)
	assert.Equal(t, "new content\n", result.Edits[0].NewContent)
}

func TestParse_MalformedBlock_MissingReplace(t *testing.T) {
	response := `internal/editor/apply.go
<<<<<<< SEARCH
return nil
=======
return applyEdit(path)`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 0, len(result.Edits))
	assert.Equal(t, 1, len(result.ParseErrors))
	assert.Contains(t, result.ParseErrors[0].Message, "unclosed block")
	assert.Contains(t, result.ParseErrors[0].RawText, "return nil")
	assert.Greater(t, result.ParseErrors[0].Position, 0)
}

func TestParse_MalformedBlock_MissingDivider(t *testing.T) {
	response := `file.go
<<<<<<< SEARCH
some content`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 0, len(result.Edits))
	assert.Equal(t, 1, len(result.ParseErrors))
	assert.Contains(t, result.ParseErrors[0].Message, "divider")
}

func TestParse_EmptyResponse(t *testing.T) {
	_, err := Parse("")
	require.Error(t, err)
	assert.IsType(t, &NoEditsFoundError{}, err)
}

func TestParse_NoBlocks(t *testing.T) {
	_, err := Parse("This is just reasoning text with no edit blocks.")
	require.Error(t, err)
	assert.IsType(t, &NoEditsFoundError{}, err)
}

func TestParse_ResponseMetadata(t *testing.T) {
	response := `Let me explain the change.

First, we need to update the config:

config.yaml
<<<<<<< SEARCH
timeout: 30
=======
timeout: 60
>>>>>>> REPLACE

And that should fix the issue.`

	result, err := Parse(response)
	require.NoError(t, err)
	assert.Equal(t, 1, result.BlocksFound)
	assert.Equal(t, 1, result.BlocksParsed)
	assert.Contains(t, result.ReasoningText, "explain the change")
	assert.Contains(t, result.ReasoningText, "fix the issue")
}

func TestRouter_RoutesGoToAST(t *testing.T) {
	var astCalled, textCalled bool
	astApplier := &mockApplier{onApply: func(e types.Edit) {
		astCalled = true
	}}
	textApplier := &mockApplier{onApply: func(e types.Edit) {
		textCalled = true
	}}

	router := &Router{
		ASTApplier:  astApplier,
		TextApplier: textApplier,
	}

	result := router.ApplyAll([]types.Edit{
		{FilePath: "main.go", OldContent: "old", NewContent: "new"},
	})

	assert.True(t, astCalled)
	assert.False(t, textCalled)
	assert.Equal(t, 1, len(result.Applied))
	assert.Empty(t, result.Errors)
}

func TestRouter_RoutesNonGoToText(t *testing.T) {
	var astCalled, textCalled bool
	astApplier := &mockApplier{onApply: func(e types.Edit) {
		astCalled = true
	}}
	textApplier := &mockApplier{onApply: func(e types.Edit) {
		textCalled = true
	}}

	router := &Router{
		ASTApplier:  astApplier,
		TextApplier: textApplier,
	}

	result := router.ApplyAll([]types.Edit{
		{FilePath: "config.yaml", OldContent: "old", NewContent: "new"},
	})

	assert.False(t, astCalled)
	assert.True(t, textCalled)
	assert.Equal(t, 1, len(result.Applied))
}

func TestRouter_MixedEdits(t *testing.T) {
	var goFiles, otherFiles []string
	astApplier := &mockApplier{onApply: func(e types.Edit) {
		goFiles = append(goFiles, e.FilePath)
	}}
	textApplier := &mockApplier{onApply: func(e types.Edit) {
		otherFiles = append(otherFiles, e.FilePath)
	}}

	router := &Router{
		ASTApplier:  astApplier,
		TextApplier: textApplier,
	}

	edits := []types.Edit{
		{FilePath: "main.go"},
		{FilePath: "config.yaml"},
		{FilePath: "internal/ast/scan.go"},
		{FilePath: "README.md"},
	}

	result := router.ApplyAll(edits)
	assert.Equal(t, 4, len(result.Applied))
	assert.Equal(t, []string{"main.go", "internal/ast/scan.go"}, goFiles)
	assert.Equal(t, []string{"config.yaml", "README.md"}, otherFiles)
}

func TestRouter_ContinuesOnError(t *testing.T) {
	callCount := 0
	failApplier := &mockApplier{
		onApply: func(e types.Edit) { callCount++ },
		failOn:  map[string]bool{"fail.go": true},
	}
	textApplier := &mockApplier{
		onApply: func(e types.Edit) { callCount++ },
	}

	router := &Router{
		ASTApplier:  failApplier,
		TextApplier: textApplier,
	}

	edits := []types.Edit{
		{FilePath: "fail.go"},
		{FilePath: "config.yaml"},
	}

	result := router.ApplyAll(edits)
	assert.Equal(t, 1, len(result.Errors))
	assert.Equal(t, 1, len(result.Applied))
}

func TestRouter_IntegrationWithTextEditor(t *testing.T) {
	dir := t.TempDir()

	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte("timeout: 30\nretries: 3\n"), 0o644))

	textEditor := &editor.TextEditor{}
	// Use a mock for AST since we don't need real AST here.
	astApplier := &mockApplier{}

	router := &Router{
		ASTApplier:  astApplier,
		TextApplier: textEditor,
	}

	edits := []types.Edit{
		{
			FilePath:   yamlPath,
			OldContent: "timeout: 30\n",
			NewContent: "timeout: 60\n",
		},
	}

	result := router.ApplyAll(edits)
	assert.Empty(t, result.Errors)
	require.Equal(t, 1, len(result.Applied))
	assert.Equal(t, types.StageExact, result.Applied[0].Stage)

	got, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.Equal(t, "timeout: 60\nretries: 3\n", string(got))
}

// mockApplier is a test double for types.Applier.
type mockApplier struct {
	onApply func(types.Edit)
	failOn  map[string]bool
}

func (m *mockApplier) Apply(edit types.Edit) (*types.ApplyResult, error) {
	if m.failOn != nil && m.failOn[edit.FilePath] {
		return nil, &types.Diagnostic{
			FilePath:   edit.FilePath,
			SearchText: edit.OldContent,
		}
	}
	if m.onApply != nil {
		m.onApply(edit)
	}
	return &types.ApplyResult{
		FilePath:   edit.FilePath,
		Stage:      types.StageExact,
		Similarity: 1.0,
	}, nil
}
