// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd008-git-integration R3;
//
//	docs/ARCHITECTURE § Git Integration.
package git

import (
	"fmt"
	"strings"
	"unicode"
)

const maxSubjectLength = 72

// commitType maps prompt keywords to conventional commit types.
var commitTypes = []struct {
	keywords []string
	prefix   string
}{
	{[]string{"fix", "bug", "repair", "patch", "resolve", "correct"}, "fix"},
	{[]string{"refactor", "restructure", "reorganize", "clean up", "simplify"}, "refactor"},
	{[]string{"test", "spec", "coverage"}, "test"},
	{[]string{"doc", "comment", "readme", "documentation"}, "docs"},
	{[]string{"style", "format", "lint", "whitespace"}, "style"},
	{[]string{"perf", "performance", "optimize", "speed"}, "perf"},
	{[]string{"ci", "pipeline", "workflow", "github action"}, "ci"},
	{[]string{"build", "dependency", "deps", "module"}, "build"},
	{[]string{"chore", "cleanup", "maintain"}, "chore"},
	// "feat" is the default, so it comes last with broad keywords.
	{[]string{"add", "create", "implement", "new", "feature", "introduce"}, "feat"},
}

// GenerateMessage creates a conventional commit message from the task prompt
// and list of modified files.
//
// Implements: prd008-git-integration R3.1-R3.5.
func GenerateMessage(prompt string, modifiedFiles []string) string {
	commitType := inferCommitType(prompt)
	subject := buildSubject(commitType, prompt)
	body := buildBody(modifiedFiles)

	msg := subject
	if body != "" {
		msg += "\n\n" + body
	}
	msg += "\n\n" + coAuthorTrailer

	return msg
}

// inferCommitType determines the conventional commit type from prompt keywords.
//
// Implements: prd008-git-integration R3.4, R3.5.
func inferCommitType(prompt string) string {
	lower := strings.ToLower(prompt)
	for _, ct := range commitTypes {
		for _, kw := range ct.keywords {
			if containsWord(lower, kw) {
				return ct.prefix
			}
		}
	}
	return "feat"
}

// containsWord checks whether text contains keyword as a whole word
// (bounded by non-letter characters or string edges). For multi-word
// keywords like "clean up", it falls back to substring matching.
func containsWord(text, keyword string) bool {
	if strings.Contains(keyword, " ") {
		return strings.Contains(text, keyword)
	}
	idx := 0
	for {
		i := strings.Index(text[idx:], keyword)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(keyword)
		leftOK := start == 0 || !unicode.IsLetter(rune(text[start-1]))
		rightOK := end == len(text) || !unicode.IsLetter(rune(text[end]))
		if leftOK && rightOK {
			return true
		}
		idx = start + 1
	}
}

// buildSubject creates the first line of the commit message.
// Format: "type: summary" (max 72 chars).
//
// Implements: prd008-git-integration R3.1.
func buildSubject(commitType, prompt string) string {
	// Clean up the prompt for use as a summary.
	summary := strings.TrimSpace(prompt)
	summary = strings.ToLower(summary[:1]) + summary[1:] // lowercase first char

	// Remove trailing period.
	summary = strings.TrimRight(summary, ".")

	subject := fmt.Sprintf("%s: %s", commitType, summary)
	if len(subject) > maxSubjectLength {
		subject = subject[:maxSubjectLength-3] + "..."
	}

	return subject
}

// buildBody creates the commit body listing modified files.
//
// Implements: prd008-git-integration R3.2.
func buildBody(modifiedFiles []string) string {
	if len(modifiedFiles) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("Modified files:\n")
	for _, f := range modifiedFiles {
		buf.WriteString(fmt.Sprintf("- %s\n", f))
	}
	return buf.String()
}
