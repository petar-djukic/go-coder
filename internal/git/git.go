// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package git provides auto-commit, dirty file handling, and undo for
// AI-generated edits.
// Implements: prd008-git-integration R1, R2, R4, R5;
//
//	docs/ARCHITECTURE § Git Integration.
package git

import (
	"errors"
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	coAuthorTrailer = "Co-Authored-By: go-coder <noreply@go-coder>"
	dirtyCommitMsg  = "go-coder: save uncommitted changes before edit"
)

// ErrNotGoCoderCommit is returned when undo targets a commit not made by go-coder.
var ErrNotGoCoderCommit = errors.New("not a go-coder commit")

// ErrDirtyWorkTree is returned when uncommitted changes exist and DirtyCommit is false.
var ErrDirtyWorkTree = errors.New("uncommitted changes exist")

// ErrNoGit is returned when the working directory is not a git repository.
var ErrNoGit = errors.New("not a git repository")

// Config configures git integration behavior.
type Config struct {
	WorkDir     string // Repository working directory
	AutoCommit  bool   // Create commits after edits (default true)
	DirtyCommit bool   // Commit dirty files before edits (default true)
}

// Repo wraps a go-git repository for the operations we need.
type Repo struct {
	repo *gogit.Repository
	cfg  Config
}

// Open opens an existing git repository at the configured work directory.
// Returns ErrNoGit if the directory is not a git repository.
//
// Implements: prd008-git-integration R1.6, R5.1.
func Open(cfg Config) (*Repo, error) {
	r, err := gogit.PlainOpen(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoGit, err)
	}
	return &Repo{repo: r, cfg: cfg}, nil
}

// IsDirty returns true if the working tree has uncommitted changes
// (either staged or unstaged).
//
// Implements: prd008-git-integration R2.1, R2.5.
func (r *Repo) IsDirty() (bool, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("getting worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("getting status: %w", err)
	}

	return !status.IsClean(), nil
}

// IsGoCoderCommit checks whether the HEAD commit was made by go-coder
// by looking for the Co-Authored-By trailer.
//
// Implements: prd008-git-integration R4.2.
func (r *Repo) IsGoCoderCommit() (bool, error) {
	head, err := r.repo.Head()
	if err != nil {
		return false, fmt.Errorf("getting HEAD: %w", err)
	}

	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return false, fmt.Errorf("getting commit: %w", err)
	}

	return strings.Contains(commit.Message, coAuthorTrailer), nil
}

// lastCommitMessage returns the message of the HEAD commit.
func (r *Repo) lastCommitMessage() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", err
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}
	return commit.Message, nil
}

// commitCount returns the total number of commits reachable from HEAD.
func (r *Repo) commitCount() (int, error) {
	iter, err := r.repo.Log(&gogit.LogOptions{})
	if err != nil {
		return 0, err
	}
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	return count, err
}
