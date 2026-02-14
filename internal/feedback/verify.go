// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package feedback runs the compiler and test verification loop after edits
// are applied, formats errors for LLM follow-up, and manages retries.
// Implements: prd007-feedback-loop R1, R2, R5;
//
//	docs/ARCHITECTURE § Feedback Loop.
package feedback

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCmdTimeout  = 60 * time.Second
	defaultTestTimeout = 120 * time.Second
)

// CompileError represents a single compiler or vet error.
//
// Implements: prd007-feedback-loop R5.2.
type CompileError struct {
	FilePath string // Source file path
	Line     int    // Line number (1-based)
	Column   int    // Column number (1-based, 0 if not available)
	Message  string // Error message text
}

func (e CompileError) String() string {
	if e.Column > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.FilePath, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s", e.FilePath, e.Line, e.Message)
}

// VerifyResult holds the outcome of running build, vet, and test.
//
// Implements: prd007-feedback-loop R5.1.
type VerifyResult struct {
	BuildOK    bool           // go build succeeded
	VetOK      bool           // go vet succeeded (false if build failed and vet was skipped)
	TestOK     bool           // test command succeeded (true if no test command configured)
	Errors     []CompileError // Parsed compiler/vet errors
	BuildOut   string         // Raw build output (stdout+stderr)
	VetOut     string         // Raw vet output (stdout+stderr)
	TestOutput string         // Raw test output (stdout+stderr)
}

// Success returns true when all verification steps passed.
//
// Implements: prd007-feedback-loop R5.3.
func (r *VerifyResult) Success() bool {
	return r.BuildOK && r.VetOK && r.TestOK
}

// VerifyConfig configures the verifier.
type VerifyConfig struct {
	WorkDir     string        // Working directory (module root)
	TestCmd     string        // Test command (empty to skip tests)
	CmdTimeout  time.Duration // Timeout for build/vet commands (default 60s)
	TestTimeout time.Duration // Timeout for test command (default 120s)
}

// Verify runs go build, go vet, and the test command in sequence. It returns
// a VerifyResult summarizing the outcome.
//
// Implements: prd007-feedback-loop R1.1-R1.6, R2.1-R2.6.
func Verify(ctx context.Context, cfg VerifyConfig) *VerifyResult {
	result := &VerifyResult{TestOK: true} // Default TestOK=true for no test command case.

	cmdTimeout := cfg.CmdTimeout
	if cmdTimeout == 0 {
		cmdTimeout = defaultCmdTimeout
	}
	testTimeout := cfg.TestTimeout
	if testTimeout == 0 {
		testTimeout = defaultTestTimeout
	}

	// R1.1: Run go build ./...
	buildOut, buildErr := runCommand(ctx, cfg.WorkDir, cmdTimeout, "go", "build", "./...")
	result.BuildOut = buildOut
	result.BuildOK = buildErr == nil

	if !result.BuildOK {
		result.Errors = parseCompileErrors(buildOut)
		// R1.6: Skip vet if build fails.
		return result
	}

	// R1.2: Run go vet ./...
	vetOut, vetErr := runCommand(ctx, cfg.WorkDir, cmdTimeout, "go", "vet", "./...")
	result.VetOut = vetOut
	result.VetOK = vetErr == nil

	if !result.VetOK {
		result.Errors = append(result.Errors, parseCompileErrors(vetOut)...)
	}

	// R2.1, R2.2: Run test command only if build and vet passed.
	if cfg.TestCmd == "" {
		return result
	}

	if !result.VetOK {
		result.TestOK = false
		return result
	}

	// R2.1: Split the test command into parts.
	testParts := strings.Fields(cfg.TestCmd)
	testOut, testErr := runCommand(ctx, cfg.WorkDir, testTimeout, testParts[0], testParts[1:]...)
	result.TestOutput = testOut
	result.TestOK = testErr == nil

	return result
}

// runCommand executes a command with a timeout and captures combined output.
func runCommand(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return buf.String(), err
}

// goErrorRegex matches Go compiler/vet error output lines:
// file.go:10:5: error message
// file.go:10: error message
var goErrorRegex = regexp.MustCompile(`^(.+?\.go):(\d+)(?::(\d+))?: (.+)$`)

// parseCompileErrors extracts CompileError structs from Go compiler or vet output.
//
// Implements: prd007-feedback-loop R1.4.
func parseCompileErrors(output string) []CompileError {
	var errors []CompileError
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := goErrorRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		lineNum, _ := strconv.Atoi(matches[2])
		colNum := 0
		if matches[3] != "" {
			colNum, _ = strconv.Atoi(matches[3])
		}

		errors = append(errors, CompileError{
			FilePath: matches[1],
			Line:     lineNum,
			Column:   colNum,
			Message:  matches[4],
		})
	}
	return errors
}
