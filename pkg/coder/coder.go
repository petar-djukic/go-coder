// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package coder defines the public interface for go-coder, an AST-driven
// Go coding agent library.
// Implements: prd001-coder-interface R1, R2, R3, R6;
//
//	docs/ARCHITECTURE § Coder Interface.
package coder

import (
	"context"
	"errors"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// Error types for the Coder API.
//
// Implements: prd001-coder-interface R6.1-R6.3.
var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrLLMFailure    = errors.New("LLM call failed")
	ErrParseFailure  = errors.New("failed to parse LLM response into edits")
)

// Config configures a Coder instance.
//
// Implements: prd001-coder-interface R1.1-R1.10.
type Config struct {
	WorkDir        string // Repository root (required)
	Model          string // Bedrock model ID (required)
	Region         string // AWS region (required)
	MaxRetries     int    // Maximum feedback loop iterations (default 3)
	TestCmd        string // Test command (empty = skip tests)
	MapTokenBudget int    // Token budget for repository map (default 2048)
	MaxTokens      int    // Maximum tokens for LLM response (default 4096)
	NoGit          bool   // Disable git operations
}

// Result holds the outcome of a Coder.Run invocation.
//
// Implements: prd001-coder-interface R3.1-R3.5.
type Result struct {
	ModifiedFiles []string         // Paths of files changed
	Errors        []string         // Remaining errors after all retries
	TokensUsed    types.TokenUsage // Total tokens consumed
	Retries       int              // Number of retry iterations performed
	Success       bool             // True if no errors remain
}

// Coder runs a coding task against a repository.
//
// Implements: prd001-coder-interface R2.1-R2.4.
type Coder interface {
	// Run executes the full coding lifecycle: index the repository, build
	// a map, send the prompt to the LLM, parse edits, apply them, verify,
	// retry on failure, and return the result.
	Run(ctx context.Context, prompt string) (*Result, error)
}
