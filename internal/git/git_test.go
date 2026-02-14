// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_ValidRepo(t *testing.T) {
	dir := initTestRepo(t)

	repo, err := Open(Config{WorkDir: dir, AutoCommit: true, DirtyCommit: true})
	require.NoError(t, err)
	assert.NotNil(t, repo)
}

func TestOpen_NotARepo(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(Config{WorkDir: dir})
	assert.ErrorIs(t, err, ErrNoGit)
}

func TestIsDirty_CleanRepo(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.False(t, dirty)
}

func TestIsDirty_WithUnstagedChanges(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	// Modify a tracked file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { /* modified */ }\n"), 0o644))

	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.True(t, dirty)
}

func TestIsDirty_WithUntrackedFiles(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	// Create a new untracked file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0o644))

	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.True(t, dirty)
}

func TestIsGoCoderCommit(t *testing.T) {
	t.Run("go-coder commit", func(t *testing.T) {
		dir := initTestRepo(t)
		addFileAndCommit(t, dir, "test.go", "package main\n", "feat: test\n\n"+coAuthorTrailer)

		repo, err := Open(Config{WorkDir: dir})
		require.NoError(t, err)

		isGoCoder, err := repo.IsGoCoderCommit()
		require.NoError(t, err)
		assert.True(t, isGoCoder)
	})

	t.Run("non-go-coder commit", func(t *testing.T) {
		dir := initTestRepo(t)
		// The initial commit from initTestRepo doesn't have the trailer.

		repo, err := Open(Config{WorkDir: dir})
		require.NoError(t, err)

		isGoCoder, err := repo.IsGoCoderCommit()
		require.NoError(t, err)
		assert.False(t, isGoCoder)
	})
}

func TestGenerateMessage(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		files      []string
		wantPrefix string
		wantTrailer bool
	}{
		{
			name:       "add feature",
			prompt:     "Add a fibonacci function",
			files:      []string{"math.go"},
			wantPrefix: "feat:",
			wantTrailer: true,
		},
		{
			name:       "fix bug",
			prompt:     "Fix the nil pointer dereference in handler",
			files:      []string{"handler.go"},
			wantPrefix: "fix:",
			wantTrailer: true,
		},
		{
			name:       "refactor code",
			prompt:     "Refactor the database layer",
			files:      []string{"db.go", "model.go"},
			wantPrefix: "refactor:",
			wantTrailer: true,
		},
		{
			name:       "test keyword",
			prompt:     "Add test coverage for the parser",
			files:      []string{"parser_test.go"},
			wantPrefix: "test:",
			wantTrailer: true,
		},
		{
			name:       "default to feat",
			prompt:     "Make the thing work better",
			files:      []string{"thing.go"},
			wantPrefix: "feat:",
			wantTrailer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GenerateMessage(tt.prompt, tt.files)
			assert.Contains(t, msg, tt.wantPrefix)
			if tt.wantTrailer {
				assert.Contains(t, msg, coAuthorTrailer)
			}
			// First line should be <= 72 chars.
			firstLine := msg[:len(msg)-len(msg)+len(firstLineOf(msg))]
			assert.LessOrEqual(t, len(firstLine), maxSubjectLength)
		})
	}
}

func TestGenerateMessage_LongPromptTruncated(t *testing.T) {
	longPrompt := "Add a very long feature that does many things and should be truncated because the commit message subject line must not exceed seventy-two characters"
	msg := GenerateMessage(longPrompt, []string{"long.go"})

	firstLine := firstLineOf(msg)
	assert.LessOrEqual(t, len(firstLine), maxSubjectLength)
	assert.Contains(t, firstLine, "...")
}

func TestGenerateMessage_IncludesFiles(t *testing.T) {
	msg := GenerateMessage("Add feature", []string{"a.go", "b.go"})
	assert.Contains(t, msg, "- a.go")
	assert.Contains(t, msg, "- b.go")
	assert.Contains(t, msg, "Modified files:")
}

func TestInferCommitType(t *testing.T) {
	tests := []struct {
		prompt string
		want   string
	}{
		{"fix the bug", "fix"},
		{"add a feature", "feat"},
		{"refactor the handler", "refactor"},
		{"update documentation", "docs"},
		{"optimize performance", "perf"},
		{"something generic", "feat"},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			assert.Equal(t, tt.want, inferCommitType(tt.prompt))
		})
	}
}

// initTestRepo creates a temp dir with a git repo, an initial commit, and
// returns the directory path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	r, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)

	wt, err := r.Worktree()
	require.NoError(t, err)

	// Create an initial file and commit.
	mainGo := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n\nfunc main() {}\n"), 0o644))

	_, err = wt.Add("main.go")
	require.NoError(t, err)

	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	return dir
}

// addFileAndCommit adds a file and creates a commit with the given message.
func addFileAndCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()

	r, err := gogit.PlainOpen(dir)
	require.NoError(t, err)

	wt, err := r.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))

	_, err = wt.Add(name)
	require.NoError(t, err)

	_, err = wt.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
}

func firstLineOf(s string) string {
	idx := 0
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
		idx = i
	}
	_ = idx
	return s
}
