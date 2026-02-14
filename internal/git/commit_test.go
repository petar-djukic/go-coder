// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDirty_CleanRepo(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, DirtyCommit: true})
	require.NoError(t, err)

	// Clean repo: HandleDirty should be a no-op.
	require.NoError(t, repo.HandleDirty())

	// Commit count should still be 1 (only the initial commit).
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestHandleDirty_CommitsDirtyFiles(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, DirtyCommit: true})
	require.NoError(t, err)

	// Create a dirty file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("package main\n"), 0o644))

	require.NoError(t, repo.HandleDirty())

	// Should now be clean.
	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.False(t, dirty)

	// Commit count should be 2.
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// The dirty commit message should match the expected message.
	msg, err := repo.lastCommitMessage()
	require.NoError(t, err)
	assert.Equal(t, dirtyCommitMsg, msg)
}

func TestHandleDirty_ReturnsErrorWhenDisabled(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, DirtyCommit: false})
	require.NoError(t, err)

	// Create a dirty file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("package main\n"), 0o644))

	err = repo.HandleDirty()
	assert.ErrorIs(t, err, ErrDirtyWorkTree)
}

func TestAutoCommit_StagesAndCommits(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, AutoCommit: true})
	require.NoError(t, err)

	// Create files that the agent "modified".
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n\nfunc Feature() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helper.go"), []byte("package main\n\nfunc Helper() {}\n"), 0o644))

	err = repo.AutoCommit([]string{"feature.go", "helper.go"}, "Add a feature and helper")
	require.NoError(t, err)

	// Repo should be clean.
	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.False(t, dirty)

	// Commit message should contain the Co-Authored-By trailer.
	msg, err := repo.lastCommitMessage()
	require.NoError(t, err)
	assert.Contains(t, msg, coAuthorTrailer)

	// Commit message should contain the conventional commit type.
	assert.Contains(t, msg, "feat:")
}

func TestAutoCommit_OnlyStagesSpecifiedFiles(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, AutoCommit: true})
	require.NoError(t, err)

	// Create two files, but only commit one.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package main\n"), 0o644))

	err = repo.AutoCommit([]string{"tracked.go"}, "Add tracked file")
	require.NoError(t, err)

	// Repo should still be dirty (untracked.go is not committed).
	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.True(t, dirty)
}

func TestAutoCommit_DisabledIsNoop(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, AutoCommit: false})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0o644))

	err = repo.AutoCommit([]string{"feature.go"}, "Add feature")
	require.NoError(t, err)

	// Should still be dirty since AutoCommit is disabled.
	dirty, err := repo.IsDirty()
	require.NoError(t, err)
	assert.True(t, dirty)

	// Commit count should still be 1.
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUndo_RevertsGoCoderCommit(t *testing.T) {
	dir := initTestRepo(t)

	// Add a go-coder commit.
	addFileAndCommit(t, dir, "feature.go", "package main\n\nfunc Feature() {}\n", "feat: add feature\n\n"+coAuthorTrailer)

	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	// Verify we have 2 commits.
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Undo should succeed.
	require.NoError(t, repo.Undo())

	// Back to 1 commit.
	count, err = repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// The feature file should still exist in the working tree (soft reset).
	_, err = os.Stat(filepath.Join(dir, "feature.go"))
	assert.NoError(t, err)
}

func TestUndo_RefusesNonGoCoderCommit(t *testing.T) {
	dir := initTestRepo(t)

	// The initial commit from initTestRepo doesn't have the trailer.
	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	err = repo.Undo()
	assert.ErrorIs(t, err, ErrNotGoCoderCommit)

	// Commit count should remain unchanged.
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUndo_PreservesChangesInWorkTree(t *testing.T) {
	dir := initTestRepo(t)

	// Add a go-coder commit that modifies main.go.
	addFileAndCommit(t, dir, "main.go", "package main\n\nfunc main() { /* modified */ }\n", "feat: modify main\n\n"+coAuthorTrailer)

	repo, err := Open(Config{WorkDir: dir})
	require.NoError(t, err)

	require.NoError(t, repo.Undo())

	// The modified content should still be in the working tree.
	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "modified")
}

func TestAutoCommit_IntegrationWithHandleDirty(t *testing.T) {
	dir := initTestRepo(t)
	repo, err := Open(Config{WorkDir: dir, AutoCommit: true, DirtyCommit: true})
	require.NoError(t, err)

	// Create a pre-existing dirty file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.go"), []byte("package main\n"), 0o644))

	// HandleDirty commits the dirty file.
	require.NoError(t, repo.HandleDirty())

	// Now simulate agent work.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.go"), []byte("package main\n\nfunc Agent() {}\n"), 0o644))

	err = repo.AutoCommit([]string{"agent.go"}, "Add agent function")
	require.NoError(t, err)

	// Should have 3 commits: initial, dirty save, agent commit.
	count, err := repo.commitCount()
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Last commit should be the go-coder commit.
	isGoCoder, err := repo.IsGoCoderCommit()
	require.NoError(t, err)
	assert.True(t, isGoCoder)
}
