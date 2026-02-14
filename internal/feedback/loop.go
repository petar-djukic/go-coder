// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-feedback-loop R4;
//
//	docs/ARCHITECTURE § Feedback Loop.
package feedback

import (
	"context"
	"fmt"

	"github.com/petar-djukic/go-coder/pkg/types"
)

const defaultMaxRetries = 3

// RetryFunc is called on each retry iteration with the formatted error prompt.
// It should send the prompt to the LLM, parse the response into edits, apply
// them, and return the list of modified files. If it fails, it returns an error.
type RetryFunc func(ctx context.Context, errorPrompt string) (modifiedFiles []string, err error)

// LoopConfig configures the retry loop.
type LoopConfig struct {
	VerifyConfig VerifyConfig // Verification settings
	FormatConfig FormatConfig // Error formatting settings
	MaxRetries   int          // Maximum retry iterations (default 3)
}

// LoopResult holds the outcome of the retry loop.
type LoopResult struct {
	Success       bool              // All verification passed
	Retries       int               // Number of retry iterations performed
	FinalResult   *VerifyResult     // Last verification result
	ModifiedFiles []string          // All files modified across all iterations
	TokenUsage    types.TokenUsage  // Cumulative token usage (if tracked by caller)
}

// Run executes the verify-retry loop. It first runs verification. If
// verification fails, it formats the errors, calls retryFn to get corrections,
// and re-verifies. This continues up to MaxRetries times.
//
// Implements: prd007-feedback-loop R4.1-R4.6.
func Run(ctx context.Context, cfg LoopConfig, initialFiles []string, retryFn RetryFunc) (*LoopResult, error) {
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}

	result := &LoopResult{
		ModifiedFiles: initialFiles,
	}

	// Initial verification.
	vr := Verify(ctx, cfg.VerifyConfig)
	result.FinalResult = vr

	if vr.Success() {
		result.Success = true
		return result, nil
	}

	// Retry loop.
	for i := 0; i < maxRetries; i++ {
		// R4.6: Check context cancellation.
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("context canceled after %d retries: %w", result.Retries, err)
		}

		result.Retries++

		// R3: Format errors into a follow-up prompt.
		errorPrompt := FormatErrors(vr, result.ModifiedFiles, cfg.FormatConfig)

		// R4.1: Send to LLM, parse, apply.
		modifiedFiles, err := retryFn(ctx, errorPrompt)
		if err != nil {
			return result, fmt.Errorf("retry %d failed: %w", result.Retries, err)
		}

		// Track newly modified files.
		result.ModifiedFiles = mergeFiles(result.ModifiedFiles, modifiedFiles)

		// R4.1 continued: Re-verify.
		vr = Verify(ctx, cfg.VerifyConfig)
		result.FinalResult = vr

		// R4.5: Exit on success.
		if vr.Success() {
			result.Success = true
			return result, nil
		}
	}

	// R4.4: All retries exhausted.
	return result, fmt.Errorf("max retries (%d) exhausted with remaining errors", maxRetries)
}

// mergeFiles combines two file lists, deduplicating entries.
func mergeFiles(existing, additional []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, f := range existing {
		seen[f] = true
	}
	merged := make([]string, len(existing))
	copy(merged, existing)
	for _, f := range additional {
		if !seen[f] {
			merged = append(merged, f)
			seen[f] = true
		}
	}
	return merged
}
