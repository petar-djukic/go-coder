// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package editor implements text-based search/replace for non-Go files.
// Implements: prd003-edit-engine R1, R2, R3;
//
//	docs/ARCHITECTURE § Text Editor.
package editor

import (
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const defaultFuzzyThreshold = 0.8

// matchResult holds the outcome of a single match attempt.
type matchResult struct {
	start      int              // Byte offset of the match start in the content
	end        int              // Byte offset of the match end in the content
	stage      types.MatchStage // Which stage found the match
	similarity float64          // Similarity score (1.0 for exact and whitespace)
}

// findMatch runs the three matching stages in order against content,
// returning the first successful match. Returns nil if no stage matches.
//
// Implements: prd003-edit-engine R2.1-R2.4.
func findMatch(content, search string, fuzzyThreshold float64) *matchResult {
	if m := exactMatch(content, search); m != nil {
		return m
	}
	if m := whitespaceNormalizedMatch(content, search); m != nil {
		return m
	}
	return fuzzyMatch(content, search, fuzzyThreshold)
}

// exactMatch attempts a byte-for-byte substring match.
// Implements: prd003-edit-engine R2.1.
func exactMatch(content, search string) *matchResult {
	idx := strings.Index(content, search)
	if idx < 0 {
		return nil
	}
	return &matchResult{
		start:      idx,
		end:        idx + len(search),
		stage:      types.StageExact,
		similarity: 1.0,
	}
}

// whitespaceNormalizedMatch collapses runs of whitespace in both content
// and search text, then finds the match by comparing normalized lines.
// When found, it maps back to the original content line boundaries.
//
// Implements: prd003-edit-engine R2.2.
func whitespaceNormalizedMatch(content, search string) *matchResult {
	normSearchLines := normalizeLines(search)
	if len(normSearchLines) == 0 {
		return nil
	}

	contentLines := strings.Split(content, "\n")
	normContentLines := make([]string, len(contentLines))
	for i, line := range contentLines {
		normContentLines[i] = collapseSpaces(strings.TrimSpace(line))
	}

	// Slide a window of len(normSearchLines) over normContentLines.
	searchLen := len(normSearchLines)
	for i := 0; i <= len(normContentLines)-searchLen; i++ {
		match := true
		for j := 0; j < searchLen; j++ {
			if normContentLines[i+j] != normSearchLines[j] {
				match = false
				break
			}
		}
		if match {
			start := byteOffsetOfLine(contentLines, i)
			end := byteOffsetOfLine(contentLines, i+searchLen)
			return &matchResult{
				start:      start,
				end:        end,
				stage:      types.StageWhitespaceNormalized,
				similarity: 1.0,
			}
		}
	}

	return nil
}

// normalizeLines splits text into lines and normalizes each line by
// trimming whitespace and collapsing runs of spaces.
func normalizeLines(s string) []string {
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from a terminal newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = collapseSpaces(strings.TrimSpace(line))
	}
	return result
}

// fuzzyMatch scans content for the region most similar to search,
// returning a match only if the similarity meets the threshold.
//
// Implements: prd003-edit-engine R2.3.
func fuzzyMatch(content, search string, threshold float64) *matchResult {
	if search == "" || content == "" {
		return nil
	}

	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")
	searchLen := len(searchLines)

	if searchLen > len(contentLines) {
		// Try the whole content as a single candidate.
		sim := similarity(content, search)
		if sim >= threshold {
			return &matchResult{
				start:      0,
				end:        len(content),
				stage:      types.StageFuzzy,
				similarity: sim,
			}
		}
		return nil
	}

	var best *matchResult
	for i := 0; i <= len(contentLines)-searchLen; i++ {
		candidate := strings.Join(contentLines[i:i+searchLen], "\n")
		sim := similarity(candidate, search)
		if sim >= threshold && (best == nil || sim > best.similarity) {
			// Compute byte offset of this line range.
			start := byteOffsetOfLine(contentLines, i)
			end := start + len(candidate)
			best = &matchResult{
				start:      start,
				end:        end,
				stage:      types.StageFuzzy,
				similarity: sim,
			}
		}
	}

	return best
}

// findClosestMatch finds the best partial match in content for diagnostics.
// Returns the closest match text, its similarity, and line range.
func findClosestMatch(content, search string) (closest string, sim float64, lineStart, lineEnd int) {
	if search == "" || content == "" {
		return "", 0, 0, 0
	}

	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")
	searchLen := len(searchLines)

	if searchLen > len(contentLines) {
		searchLen = len(contentLines)
	}

	var bestSim float64
	var bestStart int

	for i := 0; i <= len(contentLines)-searchLen; i++ {
		candidate := strings.Join(contentLines[i:i+searchLen], "\n")
		s := similarity(candidate, search)
		if s > bestSim {
			bestSim = s
			bestStart = i
		}
	}

	if bestSim > 0 {
		closest = strings.Join(contentLines[bestStart:bestStart+searchLen], "\n")
		return closest, bestSim, bestStart + 1, bestStart + searchLen
	}

	return "", 0, 0, 0
}

// similarity computes the Levenshtein-based similarity ratio between two strings
// using the go-diff library. Returns a value between 0.0 and 1.0.
func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a, b, false)
	distance := dmp.DiffLevenshtein(diffs)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	return 1.0 - float64(distance)/float64(maxLen)
}

// normalizeWhitespace collapses runs of whitespace characters into a single
// space and trims leading/trailing whitespace from each line.
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = collapseSpaces(strings.TrimSpace(line))
	}
	return strings.Join(lines, "\n")
}


// collapseSpaces replaces runs of spaces and tabs with a single space.
func collapseSpaces(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	return b.String()
}

// byteOffsetOfLine returns the byte offset of the start of line idx
// in the content reconstructed from lines.
func byteOffsetOfLine(lines []string, idx int) int {
	offset := 0
	for i := 0; i < idx; i++ {
		offset += len(lines[i]) + 1 // +1 for newline
	}
	return offset
}
