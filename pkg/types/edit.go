// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-coder-interface R5.1, R5.2 (Edit struct);
//
//	prd003-edit-engine R5.1 (Applier interface);
//	prd003-edit-engine R2.5, R3 (MatchStage, Diagnostic).
package types

import "fmt"

// Edit represents a single file edit extracted from an LLM response.
type Edit struct {
	FilePath   string // Target file path relative to the repository root
	OldContent string // Text to search for (empty for create/append)
	NewContent string // Replacement text (empty for delete)
	IsCreate   bool   // True if this edit creates a new file
}

// MatchStage identifies which matching strategy succeeded.
type MatchStage int

const (
	StageExact               MatchStage = iota // Byte-for-byte match
	StageWhitespaceNormalized                   // Whitespace-collapsed match
	StageFuzzy                                  // Similarity-threshold match
	StageNone                                   // No match found
)

func (s MatchStage) String() string {
	switch s {
	case StageExact:
		return "exact"
	case StageWhitespaceNormalized:
		return "whitespace_normalized"
	case StageFuzzy:
		return "fuzzy"
	case StageNone:
		return "none"
	default:
		return "unknown"
	}
}

// ApplyResult describes the outcome of applying a single edit.
type ApplyResult struct {
	FilePath   string     // File that was modified
	Stage      MatchStage // Which matching stage succeeded
	Similarity float64    // Fuzzy match similarity score (0.0-1.0; 1.0 for exact/whitespace)
}

// Diagnostic describes why a match failed, with enough detail for the
// feedback loop to format a useful message for the LLM.
type Diagnostic struct {
	FilePath       string  // File where the match was attempted
	SearchText     string  // What we searched for
	ClosestMatch   string  // Best partial match found (empty if none)
	Similarity     float64 // Similarity score of closest match
	ClosestLineStart int   // Starting line of the closest match (1-based)
	ClosestLineEnd   int   // Ending line of the closest match (1-based)
}

func (d Diagnostic) Error() string {
	if d.ClosestMatch == "" {
		return fmt.Sprintf("no match found in %s", d.FilePath)
	}
	return fmt.Sprintf("no match in %s (closest match at lines %d-%d, similarity %.2f)",
		d.FilePath, d.ClosestLineStart, d.ClosestLineEnd, d.Similarity)
}

// Applier applies an Edit to a file. Both the AST engine (for .go files) and
// the text editor (for everything else) implement this interface.
type Applier interface {
	Apply(edit Edit) (*ApplyResult, error)
}
