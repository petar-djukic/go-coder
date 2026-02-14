// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd008-git-integration R1, R2, R4;
//
//	docs/ARCHITECTURE § Git Integration.
package git

import (
	"fmt"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	authorName  = "go-coder"
	authorEmail = "noreply@go-coder"
)

// HandleDirty checks for uncommitted changes and either commits them
// separately or returns an error, depending on Config.DirtyCommit.
//
// Implements: prd008-git-integration R2.1-R2.5.
func (r *Repo) HandleDirty() error {
	dirty, err := r.IsDirty()
	if err != nil {
		return err
	}

	if !dirty {
		return nil
	}

	if !r.cfg.DirtyCommit {
		return ErrDirtyWorkTree
	}

	// Commit all dirty files as a pre-edit commit.
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	// Stage all changes.
	if _, err := wt.Add("."); err != nil {
		return fmt.Errorf("staging dirty files: %w", err)
	}

	_, err = wt.Commit(dirtyCommitMsg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("committing dirty files: %w", err)
	}

	return nil
}

// AutoCommit stages the specified files and creates a commit with the
// generated message and Co-Authored-By trailer.
//
// Implements: prd008-git-integration R1.1-R1.5.
func (r *Repo) AutoCommit(modifiedFiles []string, prompt string) error {
	if !r.cfg.AutoCommit {
		return nil
	}

	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	// R1.5: Stage only the files modified by the agent.
	for _, f := range modifiedFiles {
		if _, err := wt.Add(f); err != nil {
			return fmt.Errorf("staging %s: %w", f, err)
		}
	}

	// R3: Generate commit message from prompt.
	msg := GenerateMessage(prompt, modifiedFiles)

	_, err = wt.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

// Undo reverts the last commit if it was made by go-coder (identified by
// the Co-Authored-By trailer). Uses git reset --soft HEAD~1 to preserve
// changes in the working tree.
//
// Implements: prd008-git-integration R4.1-R4.4.
func (r *Repo) Undo() error {
	isGoCoder, err := r.IsGoCoderCommit()
	if err != nil {
		return err
	}
	if !isGoCoder {
		return ErrNotGoCoderCommit
	}

	// Get the parent commit hash.
	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("getting HEAD: %w", err)
	}

	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("getting commit: %w", err)
	}

	if commit.NumParents() == 0 {
		return fmt.Errorf("cannot undo: HEAD is the initial commit")
	}

	parent, err := commit.Parent(0)
	if err != nil {
		return fmt.Errorf("getting parent commit: %w", err)
	}

	// Reset --soft to parent: moves HEAD back but keeps changes staged.
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	err = wt.Reset(&gogit.ResetOptions{
		Commit: parent.Hash,
		Mode:   gogit.SoftReset,
	})
	if err != nil {
		return fmt.Errorf("resetting to parent: %w", err)
	}

	return nil
}
