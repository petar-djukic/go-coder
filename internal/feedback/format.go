// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-feedback-loop R3;
//
//	docs/ARCHITECTURE § Feedback Loop.
package feedback

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultContextLines = 5
	defaultMaxTestOutput = 4096
)

// FormatConfig configures error formatting.
type FormatConfig struct {
	ContextLines  int // Lines of context above/below each error (default 5)
	MaxTestOutput int // Maximum characters of test output to include (default 4096)
}

// FormatErrors produces a follow-up prompt for the LLM from a VerifyResult.
// The formatted message includes compiler errors with surrounding code context,
// test output, and a preamble instructing the LLM to fix the errors.
//
// Implements: prd007-feedback-loop R3.1-R3.5.
func FormatErrors(result *VerifyResult, modifiedFiles []string, cfg FormatConfig) string {
	contextLines := cfg.ContextLines
	if contextLines == 0 {
		contextLines = defaultContextLines
	}
	maxTestOutput := cfg.MaxTestOutput
	if maxTestOutput == 0 {
		maxTestOutput = defaultMaxTestOutput
	}

	var buf strings.Builder

	// R3.3: Preamble.
	buf.WriteString("The previous edits produced errors. Please fix them using the same search/replace edit format.\n\n")

	// R3.4: List modified files.
	if len(modifiedFiles) > 0 {
		buf.WriteString("## Modified Files\n\n")
		for _, f := range modifiedFiles {
			buf.WriteString(fmt.Sprintf("- %s\n", f))
		}
		buf.WriteString("\n")
	}

	// R3.1: Compiler/vet errors with code context.
	if len(result.Errors) > 0 {
		buf.WriteString("## Compiler Errors\n\n")
		for _, e := range result.Errors {
			buf.WriteString(fmt.Sprintf("### %s\n\n", e.String()))
			context := getCodeContext(e.FilePath, e.Line, contextLines)
			if context != "" {
				buf.WriteString("```\n")
				buf.WriteString(context)
				buf.WriteString("```\n\n")
			}
		}
	}

	// R3.1 continued: Raw build output if we have errors but no parsed ones.
	if !result.BuildOK && len(result.Errors) == 0 && result.BuildOut != "" {
		buf.WriteString("## Build Output\n\n```\n")
		buf.WriteString(result.BuildOut)
		buf.WriteString("```\n\n")
	}

	// Vet output.
	if !result.VetOK && result.VetOut != "" {
		buf.WriteString("## Vet Output\n\n```\n")
		buf.WriteString(result.VetOut)
		buf.WriteString("```\n\n")
	}

	// R3.2: Test output.
	if !result.TestOK && result.TestOutput != "" {
		testOut := result.TestOutput
		if len(testOut) > maxTestOutput {
			testOut = testOut[:maxTestOutput] + "\n... (truncated)"
		}
		buf.WriteString("## Test Output\n\n```\n")
		buf.WriteString(testOut)
		buf.WriteString("```\n\n")
	}

	return buf.String()
}

// getCodeContext reads a file and extracts lines around the error location.
// Returns numbered lines with contextLines above and below the error line.
//
// Implements: prd007-feedback-loop R3.1 (surrounding code context).
func getCodeContext(filePath string, errorLine, contextLines int) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	start := errorLine - contextLines - 1 // Convert to 0-based
	if start < 0 {
		start = 0
	}
	end := errorLine + contextLines // Already accounts for 0-based + context
	if end > len(lines) {
		end = len(lines)
	}

	var buf strings.Builder
	for i := start; i < end; i++ {
		lineNum := i + 1
		marker := "  "
		if lineNum == errorLine {
			marker = "> "
		}
		buf.WriteString(fmt.Sprintf("%s%4d │ %s\n", marker, lineNum, lines[i]))
	}

	return buf.String()
}
