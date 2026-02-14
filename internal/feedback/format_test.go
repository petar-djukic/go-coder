// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package feedback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatErrors_CompilerErrorsWithContext(t *testing.T) {
	dir := t.TempDir()
	mainGo := filepath.Join(dir, "main.go")
	content := `package main

import "fmt"

func helper() string {
    return "ok"
}

func main() {
    x :=
    fmt.Println(helper())
}
`
	require.NoError(t, os.WriteFile(mainGo, []byte(content), 0o644))

	result := &VerifyResult{
		BuildOK: false,
		Errors: []CompileError{
			{FilePath: mainGo, Line: 10, Column: 5, Message: "expected operand"},
		},
		BuildOut: fmt.Sprintf("%s:10:5: expected operand", mainGo),
	}

	formatted := FormatErrors(result, []string{"main.go"}, FormatConfig{ContextLines: 5})

	assert.Contains(t, formatted, "fix them")
	assert.Contains(t, formatted, "Modified Files")
	assert.Contains(t, formatted, "main.go")
	assert.Contains(t, formatted, "Compiler Errors")
	assert.Contains(t, formatted, "expected operand")
	// Context should include surrounding lines.
	assert.Contains(t, formatted, "func helper()")
	assert.Contains(t, formatted, "func main()")
	// Error line marker.
	assert.Contains(t, formatted, "> ")
}

func TestFormatErrors_TestOutputTruncated(t *testing.T) {
	longOutput := strings.Repeat("x", 5000)
	result := &VerifyResult{
		BuildOK:    true,
		VetOK:      true,
		TestOK:     false,
		TestOutput: longOutput,
	}

	formatted := FormatErrors(result, nil, FormatConfig{MaxTestOutput: 4096})

	assert.Contains(t, formatted, "Test Output")
	assert.Contains(t, formatted, "truncated")
	// The test output in the formatted result should be truncated.
	assert.Less(t, len(formatted), len(longOutput))
}

func TestFormatErrors_TestOutputNotTruncatedWhenShort(t *testing.T) {
	result := &VerifyResult{
		BuildOK:    true,
		VetOK:      true,
		TestOK:     false,
		TestOutput: "--- FAIL: TestAdd\n    expected 5\nFAIL\n",
	}

	formatted := FormatErrors(result, nil, FormatConfig{})

	assert.Contains(t, formatted, "FAIL")
	assert.NotContains(t, formatted, "truncated")
}

func TestFormatErrors_VetOutput(t *testing.T) {
	result := &VerifyResult{
		BuildOK: true,
		VetOK:   false,
		TestOK:  true,
		VetOut:  "main.go:7:2: unreachable code\n",
	}

	formatted := FormatErrors(result, nil, FormatConfig{})

	assert.Contains(t, formatted, "Vet Output")
	assert.Contains(t, formatted, "unreachable")
}

func TestFormatErrors_EmptyErrors(t *testing.T) {
	result := &VerifyResult{
		BuildOK: false,
		BuildOut: "some raw output\n",
	}

	formatted := FormatErrors(result, nil, FormatConfig{})

	assert.Contains(t, formatted, "Build Output")
	assert.Contains(t, formatted, "some raw output")
}

func TestFormatErrors_ModifiedFilesListed(t *testing.T) {
	result := &VerifyResult{
		BuildOK: false,
		Errors:  []CompileError{{FilePath: "a.go", Line: 1, Message: "err"}},
	}

	formatted := FormatErrors(result, []string{"a.go", "b.go"}, FormatConfig{})

	assert.Contains(t, formatted, "- a.go")
	assert.Contains(t, formatted, "- b.go")
}

func TestGetCodeContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644))

	context := getCodeContext(path, 10, 5)

	// Should include lines 5-15.
	assert.Contains(t, context, "line 5")
	assert.Contains(t, context, "line 10")
	assert.Contains(t, context, "line 15")
	// Error line should be marked.
	assert.Contains(t, context, ">   10")
}

func TestGetCodeContext_EdgeCases(t *testing.T) {
	t.Run("error at start of file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.go")
		require.NoError(t, os.WriteFile(path, []byte("line 1\nline 2\nline 3\n"), 0o644))

		context := getCodeContext(path, 1, 5)
		assert.Contains(t, context, "line 1")
		assert.Contains(t, context, ">    1")
	})

	t.Run("nonexistent file", func(t *testing.T) {
		context := getCodeContext("/nonexistent/file.go", 5, 5)
		assert.Empty(t, context)
	})
}

func TestRetryLoop_StopsOnSuccess(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {
    x :=
}
`,
	})

	callCount := 0
	retryFn := func(ctx context.Context, errorPrompt string) ([]string, error) {
		callCount++
		// Fix the file on first retry.
		fixedContent := "package main\n\nfunc main() {}\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(fixedContent), 0o644))
		return []string{"main.go"}, nil
	}

	result, err := Run(context.Background(), LoopConfig{
		VerifyConfig: VerifyConfig{WorkDir: dir},
		MaxRetries:   3,
	}, []string{"main.go"}, retryFn)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.Retries)
	assert.Equal(t, 1, callCount)
}

func TestRetryLoop_StopsAfterMaxRetries(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {
    x :=
}
`,
	})

	callCount := 0
	retryFn := func(ctx context.Context, errorPrompt string) ([]string, error) {
		callCount++
		// Never fix the error.
		return []string{"main.go"}, nil
	}

	result, err := Run(context.Background(), LoopConfig{
		VerifyConfig: VerifyConfig{WorkDir: dir},
		MaxRetries:   2,
	}, []string{"main.go"}, retryFn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
	assert.False(t, result.Success)
	assert.Equal(t, 2, result.Retries)
	assert.Equal(t, 2, callCount)
}

func TestRetryLoop_ContextCancellation(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {
    x :=
}
`,
	})

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	retryFn := func(ctx context.Context, errorPrompt string) ([]string, error) {
		callCount++
		cancel() // Cancel after first retry.
		return []string{"main.go"}, nil
	}

	result, err := Run(ctx, LoopConfig{
		VerifyConfig: VerifyConfig{WorkDir: dir},
		MaxRetries:   5,
	}, []string{"main.go"}, retryFn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.Retries)
}

func TestRetryLoop_AlreadySuccessful(t *testing.T) {
	dir := setupGoModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})

	retryFn := func(ctx context.Context, errorPrompt string) ([]string, error) {
		t.Fatal("retry should not be called for already-successful code")
		return nil, nil
	}

	result, err := Run(context.Background(), LoopConfig{
		VerifyConfig: VerifyConfig{WorkDir: dir},
		MaxRetries:   3,
	}, []string{"main.go"}, retryFn)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.Retries)
}

func TestMergeFiles(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		add      []string
		want     []string
	}{
		{"no overlap", []string{"a.go"}, []string{"b.go"}, []string{"a.go", "b.go"}},
		{"with overlap", []string{"a.go", "b.go"}, []string{"b.go", "c.go"}, []string{"a.go", "b.go", "c.go"}},
		{"empty additional", []string{"a.go"}, nil, []string{"a.go"}},
		{"empty existing", nil, []string{"a.go"}, []string{"a.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeFiles(tt.existing, tt.add)
			assert.Equal(t, tt.want, got)
		})
	}
}
