// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package editformat parses LLM response text into Edit structs and routes
// them to the appropriate edit engine.
// Implements: prd004-edit-format R1, R2, R4, R5;
//
//	docs/ARCHITECTURE § Edit Format Parser.
package editformat

import (
	"fmt"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
)

const (
	markerSearch  = "<<<<<<< SEARCH"
	markerDivider = "======="
	markerReplace = ">>>>>>> REPLACE"
)

// ParseError describes a malformed edit block in the LLM response.
//
// Implements: prd004-edit-format R4.1-R4.3.
type ParseError struct {
	Position int    // Line number where the block starts (1-based)
	RawText  string // The raw text of the malformed block
	Message  string // What went wrong
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at line %d: %s", e.Position, e.Message)
}

// NoEditsFoundError is returned when the response contains no edit blocks.
//
// Implements: prd004-edit-format R4.4.
type NoEditsFoundError struct{}

func (e *NoEditsFoundError) Error() string {
	return "no edit blocks found in response"
}

// ParseResult holds the outcome of parsing an LLM response.
//
// Implements: prd004-edit-format R5.1-R5.3.
type ParseResult struct {
	Edits         []types.Edit // Successfully parsed edits
	ParseErrors   []*ParseError // Errors from malformed blocks
	ReasoningText string       // Non-edit text from the response
	BlocksFound   int          // Total blocks attempted
	BlocksParsed  int          // Blocks that produced valid edits
}

// Parse extracts edit blocks from an LLM response. It recognizes
// SEARCH/REPLACE blocks and returns them as Edit structs. Malformed
// blocks produce ParseErrors. When no blocks are found at all, returns
// a NoEditsFoundError.
//
// Implements: prd004-edit-format R1.1-R1.7, R4.1-R4.4, R5.1-R5.3.
func Parse(response string) (*ParseResult, error) {
	if strings.TrimSpace(response) == "" {
		return nil, &NoEditsFoundError{}
	}

	result := &ParseResult{}
	lines := strings.Split(response, "\n")
	var reasoning strings.Builder
	i := 0

	for i < len(lines) {
		// Look for a SEARCH marker.
		searchIdx := -1
		for j := i; j < len(lines); j++ {
			if isMarker(lines[j], markerSearch) {
				searchIdx = j
				break
			}
		}

		if searchIdx < 0 {
			// No more blocks. Rest is reasoning.
			for ; i < len(lines); i++ {
				appendReasoning(&reasoning, lines[i])
			}
			break
		}

		// Everything before this block is reasoning text.
		// But the line immediately before SEARCH is the file path.
		filePathLine := searchIdx - 1
		for ; i < filePathLine; i++ {
			appendReasoning(&reasoning, lines[i])
		}

		// Extract file path from the line before <<<<<<< SEARCH.
		// Strip markdown fences and whitespace.
		filePath := ""
		if filePathLine >= 0 {
			filePath = extractFilePath(lines[filePathLine])
		}

		// Skip past the SEARCH marker.
		i = searchIdx + 1
		result.BlocksFound++

		// Collect search text until ======= divider.
		var searchText strings.Builder
		foundDivider := false
		for i < len(lines) {
			if isMarker(lines[i], markerDivider) {
				foundDivider = true
				i++
				break
			}
			if searchText.Len() > 0 {
				searchText.WriteByte('\n')
			}
			searchText.WriteString(lines[i])
			i++
		}

		if !foundDivider {
			result.ParseErrors = append(result.ParseErrors, &ParseError{
				Position: searchIdx + 1,
				RawText:  reconstructBlock(lines, searchIdx, i),
				Message:  "unclosed block: missing ======= divider",
			})
			continue
		}

		// Collect replacement text until >>>>>>> REPLACE marker.
		var replaceText strings.Builder
		foundReplace := false
		for i < len(lines) {
			if isMarker(lines[i], markerReplace) {
				foundReplace = true
				i++
				break
			}
			if replaceText.Len() > 0 {
				replaceText.WriteByte('\n')
			}
			replaceText.WriteString(lines[i])
			i++
		}

		if !foundReplace {
			result.ParseErrors = append(result.ParseErrors, &ParseError{
				Position: searchIdx + 1,
				RawText:  reconstructBlock(lines, searchIdx, i),
				Message:  "unclosed block: missing >>>>>>> REPLACE marker",
			})
			continue
		}

		// Skip any trailing markdown fence (```) after the REPLACE marker.
		if i < len(lines) && isMarkdownFence(lines[i]) {
			i++
		}

		if filePath == "" {
			result.ParseErrors = append(result.ParseErrors, &ParseError{
				Position: searchIdx + 1,
				RawText:  reconstructBlock(lines, searchIdx, i),
				Message:  "missing file path before <<<<<<< SEARCH marker",
			})
			continue
		}

		old := searchText.String()
		new := replaceText.String()

		// Add trailing newline if content is non-empty (the block format
		// does not include the final newline before the marker).
		if old != "" {
			old += "\n"
		}
		if new != "" {
			new += "\n"
		}

		edit := types.Edit{
			FilePath:   filePath,
			OldContent: old,
			NewContent: new,
		}

		result.Edits = append(result.Edits, edit)
		result.BlocksParsed++
	}

	result.ReasoningText = strings.TrimSpace(reasoning.String())

	if result.BlocksFound == 0 {
		return nil, &NoEditsFoundError{}
	}

	return result, nil
}

// extractFilePath cleans a file path line, stripping markdown fences,
// backticks, and leading/trailing whitespace.
func extractFilePath(line string) string {
	s := strings.TrimSpace(line)

	// Strip markdown fence openers (``` or ```language).
	if isMarkdownFence(s) {
		return ""
	}

	// Strip inline backticks.
	s = strings.Trim(s, "`")
	s = strings.TrimSpace(s)

	// If the line looks like reasoning text (contains spaces after stripping),
	// it is not a file path.
	if strings.ContainsAny(s, " \t") && !strings.Contains(s, "/") {
		return ""
	}

	return s
}

// isMarker checks if a line matches a marker, allowing leading/trailing whitespace.
func isMarker(line, marker string) bool {
	return strings.TrimSpace(line) == marker
}

// isMarkdownFence checks if a line is a markdown fence (``` with optional language).
func isMarkdownFence(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```")
}

// reconstructBlock joins lines from start to end for error reporting.
func reconstructBlock(lines []string, start, end int) string {
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

// appendReasoning adds a line to the reasoning text builder, skipping
// markdown fences that wrap edit blocks.
func appendReasoning(b *strings.Builder, line string) {
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(line)
}
