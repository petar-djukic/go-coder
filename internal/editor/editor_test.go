// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextEditor_Apply(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		edit        types.Edit
		wantContent string
		wantStage   types.MatchStage
		wantErr     bool
	}{
		{
			name:        "exact match replaces first occurrence",
			fileContent: "timeout: 30\nretries: 3\n",
			edit: types.Edit{
				OldContent: "retries: 3\n",
				NewContent: "retries: 5\n",
			},
			wantContent: "timeout: 30\nretries: 5\n",
			wantStage:   types.StageExact,
		},
		{
			name:        "exact match replaces only first of multiple",
			fileContent: "a: 1\nb: 2\na: 1\n",
			edit: types.Edit{
				OldContent: "a: 1\n",
				NewContent: "a: 99\n",
			},
			wantContent: "a: 99\nb: 2\na: 1\n",
			wantStage:   types.StageExact,
		},
		{
			name:        "whitespace normalized match handles indentation",
			fileContent: "timeout: 30\nretries: 3\n",
			edit: types.Edit{
				OldContent: "  timeout:   30\n  retries:  3\n",
				NewContent: "timeout: 60\nretries: 5\n",
			},
			wantContent: "timeout: 60\nretries: 5\n",
			wantStage:   types.StageWhitespaceNormalized,
		},
		{
			name:        "fuzzy match catches minor variations",
			fileContent: "This is a Go library coding agent\n",
			edit: types.Edit{
				OldContent: "This is a Go library coding agent.\n",
				NewContent: "This is the go-coder library agent.\n",
			},
			wantContent: "This is the go-coder library agent.\n",
			wantStage:   types.StageFuzzy,
		},
		{
			name:        "no match returns diagnostic error",
			fileContent: "completely different content\n",
			edit: types.Edit{
				OldContent: "this text does not exist anywhere in the file at all\n",
				NewContent: "replacement\n",
			},
			wantErr: true,
		},
		{
			name:        "empty search with non-empty replace appends",
			fileContent: "existing content\n",
			edit: types.Edit{
				OldContent: "",
				NewContent: "appended content\n",
			},
			wantContent: "existing content\nappended content\n",
			wantStage:   types.StageExact,
		},
		{
			name:        "empty search and empty replace returns error",
			fileContent: "some content\n",
			edit: types.Edit{
				OldContent: "",
				NewContent: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "testfile.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.fileContent), 0o644))

			tt.edit.FilePath = path
			editor := &TextEditor{}
			result, err := editor.Apply(tt.edit)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantStage, result.Stage)
			assert.Equal(t, path, result.FilePath)

			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tt.wantContent, string(got))
		})
	}
}

func TestTextEditor_CreateFile(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "subdir", "new.yaml")

		editor := &TextEditor{}
		result, err := editor.Apply(types.Edit{
			FilePath:   path,
			NewContent: "key: value\n",
			IsCreate:   true,
		})

		require.NoError(t, err)
		assert.Equal(t, path, result.FilePath)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "key: value\n", string(got))
	})

	t.Run("fails if file exists", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "existing.yaml")
		require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

		editor := &TextEditor{}
		_, err := editor.Apply(types.Edit{
			FilePath:   path,
			NewContent: "new",
			IsCreate:   true,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestReplaceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("old content"), 0o644))

	err := ReplaceFile(path, []byte("new content"))
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))
}

func TestDeleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))

	err := DeleteFile(path)
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestDiagnosticError(t *testing.T) {
	d := &types.Diagnostic{
		FilePath:         "config.yaml",
		SearchText:       "missing text",
		ClosestMatch:     "similar text",
		Similarity:       0.75,
		ClosestLineStart: 5,
		ClosestLineEnd:   6,
	}

	assert.Contains(t, d.Error(), "config.yaml")
	assert.Contains(t, d.Error(), "0.75")
}

func TestMatcher_ExactMatch(t *testing.T) {
	content := "line one\nline two\nline three\n"

	m := exactMatch(content, "line two\n")
	require.NotNil(t, m)
	assert.Equal(t, types.StageExact, m.stage)
	assert.Equal(t, 1.0, m.similarity)
	assert.Equal(t, "line two\n", content[m.start:m.end])
}

func TestMatcher_WhitespaceNormalized(t *testing.T) {
	content := "timeout: 30\nretries: 3\n"
	search := "  timeout:   30\n  retries:  3\n"

	m := whitespaceNormalizedMatch(content, search)
	require.NotNil(t, m)
	assert.Equal(t, types.StageWhitespaceNormalized, m.stage)
}

func TestMatcher_FuzzyMatch(t *testing.T) {
	content := "This is a Go library coding agent\n"
	search := "This is a Go library coding agent.\n"

	m := fuzzyMatch(content, search, 0.8)
	require.NotNil(t, m)
	assert.Equal(t, types.StageFuzzy, m.stage)
	assert.GreaterOrEqual(t, m.similarity, 0.8)
}

func TestMatcher_FuzzyBelowThreshold(t *testing.T) {
	content := "completely unrelated text"
	search := "something entirely different from the content"

	m := fuzzyMatch(content, search, 0.8)
	assert.Nil(t, m)
}

func TestFindClosestMatch(t *testing.T) {
	content := "line one\nline two\nline three\n"
	search := "line twoo"

	closest, sim, lineStart, lineEnd := findClosestMatch(content, search)
	assert.NotEmpty(t, closest)
	assert.Greater(t, sim, 0.0)
	assert.Greater(t, lineStart, 0)
	assert.Greater(t, lineEnd, 0)
}

func TestSimilarity(t *testing.T) {
	assert.Equal(t, 1.0, similarity("hello", "hello"))
	assert.Equal(t, 0.0, similarity("", "hello"))
	assert.Equal(t, 0.0, similarity("hello", ""))

	sim := similarity("hello world", "hello worl")
	assert.Greater(t, sim, 0.8)
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"collapses spaces", "a   b   c", "a b c"},
		{"trims lines", "  hello  ", "hello"},
		{"handles tabs", "a\t\tb", "a b"},
		{"preserves newlines", "a  b\n  c  d", "a b\nc d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeWhitespace(tt.in))
		})
	}
}

func TestAtomicWrite_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o755))

	err := atomicWrite(path, []byte("new"))
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}
