// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package feedback

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_BuildErrorDetected(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {
    x :=
}
`,
	})

	result := Verify(context.Background(), VerifyConfig{WorkDir: dir})

	assert.False(t, result.BuildOK)
	assert.False(t, result.Success())
	assert.NotEmpty(t, result.BuildOut)
	require.NotEmpty(t, result.Errors)

	// The error should reference main.go with a line number.
	found := false
	for _, e := range result.Errors {
		if e.FilePath == "main.go" || filepath.Base(e.FilePath) == "main.go" {
			found = true
			assert.Greater(t, e.Line, 0)
			assert.NotEmpty(t, e.Message)
		}
	}
	assert.True(t, found, "expected error in main.go, got: %v", result.Errors)
}

func TestVerify_VetSkippedOnBuildFailure(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {
    x :=
}
`,
	})

	result := Verify(context.Background(), VerifyConfig{WorkDir: dir})

	assert.False(t, result.BuildOK)
	assert.False(t, result.VetOK)
	assert.Empty(t, result.VetOut, "vet should be skipped when build fails")
}

func TestVerify_VetWarningCaptured(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

import "fmt"

func main() {
    return
    fmt.Println("unreachable")
}
`,
	})

	result := Verify(context.Background(), VerifyConfig{WorkDir: dir})

	assert.True(t, result.BuildOK, "build should succeed: %s", result.BuildOut)
	assert.False(t, result.VetOK, "vet should catch unreachable code")
	assert.Contains(t, result.VetOut, "unreachable")
}

func TestVerify_TestFailureDetected(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"math.go": `package main

func Add(a, b int) int { return a - b }
`,
		"math_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Fatal("expected 5")
    }
}
`,
		"main.go": `package main

func main() {}
`,
	})

	result := Verify(context.Background(), VerifyConfig{
		WorkDir: dir,
		TestCmd: "go test ./...",
	})

	assert.True(t, result.BuildOK)
	assert.True(t, result.VetOK)
	assert.False(t, result.TestOK)
	assert.Contains(t, result.TestOutput, "FAIL")
	assert.False(t, result.Success())
}

func TestVerify_TestSkippedWhenEmpty(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})

	result := Verify(context.Background(), VerifyConfig{
		WorkDir: dir,
		TestCmd: "",
	})

	assert.True(t, result.BuildOK)
	assert.True(t, result.VetOK)
	assert.True(t, result.TestOK)
	assert.True(t, result.Success())
}

func TestVerify_SuccessfulBuild(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

import "fmt"

func main() {
    fmt.Println("hello")
}
`,
	})

	result := Verify(context.Background(), VerifyConfig{WorkDir: dir})

	assert.True(t, result.BuildOK)
	assert.True(t, result.VetOK)
	assert.True(t, result.Success())
	assert.Empty(t, result.Errors)
}

func TestVerify_ContextCancellation(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result := Verify(ctx, VerifyConfig{WorkDir: dir})

	// Build should fail because context is cancelled.
	assert.False(t, result.BuildOK)
}

func TestParseCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantLen int
		check   func(t *testing.T, errors []CompileError)
	}{
		{
			name:    "standard error with column",
			output:  "./main.go:4:5: expected operand, found '}'",
			wantLen: 1,
			check: func(t *testing.T, errors []CompileError) {
				assert.Equal(t, "./main.go", errors[0].FilePath)
				assert.Equal(t, 4, errors[0].Line)
				assert.Equal(t, 5, errors[0].Column)
				assert.Contains(t, errors[0].Message, "expected operand")
			},
		},
		{
			name:    "error without column",
			output:  "main.go:10: undefined: foo",
			wantLen: 1,
			check: func(t *testing.T, errors []CompileError) {
				assert.Equal(t, "main.go", errors[0].FilePath)
				assert.Equal(t, 10, errors[0].Line)
				assert.Equal(t, 0, errors[0].Column)
			},
		},
		{
			name:    "multiple errors",
			output:  "a.go:1:1: syntax error\nb.go:2:3: undefined: x\n",
			wantLen: 2,
			check:   nil,
		},
		{
			name:    "non-error lines ignored",
			output:  "# command-line-arguments\n./main.go:4:5: error\n",
			wantLen: 1,
			check:   nil,
		},
		{
			name:    "empty output",
			output:  "",
			wantLen: 0,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := parseCompileErrors(tt.output)
			assert.Len(t, errors, tt.wantLen)
			if tt.check != nil {
				tt.check(t, errors)
			}
		})
	}
}

func TestCompileError_String(t *testing.T) {
	t.Run("with column", func(t *testing.T) {
		e := CompileError{FilePath: "main.go", Line: 4, Column: 5, Message: "expected operand"}
		assert.Equal(t, "main.go:4:5: expected operand", e.String())
	})

	t.Run("without column", func(t *testing.T) {
		e := CompileError{FilePath: "main.go", Line: 10, Message: "undefined: foo"}
		assert.Equal(t, "main.go:10: undefined: foo", e.String())
	})
}

func TestVerifyResult_Success(t *testing.T) {
	tests := []struct {
		name string
		vr   VerifyResult
		want bool
	}{
		{"all ok", VerifyResult{BuildOK: true, VetOK: true, TestOK: true}, true},
		{"build failed", VerifyResult{BuildOK: false, VetOK: true, TestOK: true}, false},
		{"vet failed", VerifyResult{BuildOK: true, VetOK: false, TestOK: true}, false},
		{"test failed", VerifyResult{BuildOK: true, VetOK: true, TestOK: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.vr.Success())
		})
	}
}

// setupGoModule creates a temporary Go module with the given files.
func setupGoModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	// Create go.mod.
	goMod := "module testmod\n\ngo 1.25\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644))

	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	return dir
}
