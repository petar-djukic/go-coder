// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package repomap

import (
	"context"
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_FitsWithinBudget(t *testing.T) {
	ranked := []types.RankedSymbol{
		{FilePath: "a.go", Name: "FuncA", Line: 1, Signature: "func FuncA()", Score: 0.9},
		{FilePath: "a.go", Name: "FuncB", Line: 2, Signature: "func FuncB()", Score: 0.9},
		{FilePath: "b.go", Name: "FuncC", Line: 1, Signature: "func FuncC()", Score: 0.5},
		{FilePath: "c.go", Name: "FuncD", Line: 1, Signature: "func FuncD()", Score: 0.3},
	}

	result := Render(ranked, 3, 4, RenderConfig{TokenBudget: 100, TokenRatio: 0.25})

	assert.LessOrEqual(t, result.TokensUsed, 100.0)
	assert.True(t, result.FileCount > 0)
	assert.True(t, result.SymCount > 0)
}

func TestRender_ExcludesLowRankedWhenBudgetTight(t *testing.T) {
	ranked := []types.RankedSymbol{
		{FilePath: "a.go", Name: "ImportantFunc", Line: 1, Signature: "func ImportantFunc()", Score: 0.9},
		{FilePath: "b.go", Name: "LessImportant", Line: 1, Signature: "func LessImportant()", Score: 0.5},
		{FilePath: "c.go", Name: "LeastImportant", Line: 1, Signature: "func LeastImportant()", Score: 0.1},
	}

	// Tight budget: should fit header + first file but not all three.
	result := Render(ranked, 3, 3, RenderConfig{TokenBudget: 30, TokenRatio: 0.25})

	assert.Less(t, result.FileCount, 3)
	assert.Contains(t, result.Map, "a.go")
}

func TestRender_HeaderShowsCounts(t *testing.T) {
	ranked := []types.RankedSymbol{
		{FilePath: "a.go", Name: "Func", Line: 1, Signature: "func Func()", Score: 0.9},
	}

	result := Render(ranked, 5, 10, RenderConfig{TokenBudget: 1000})

	assert.Contains(t, result.Map, "Repository map")
	assert.Contains(t, result.Map, "1/5 files")
	assert.Contains(t, result.Map, "1/10 symbols")
}

func TestRender_LongLinesTruncated(t *testing.T) {
	longSig := "func VeryLongFunctionNameThatExceedsTheMaximumLineLengthForRenderingPurposesInTheRepoMapOutput(a, b, c, d, e, f, g int) (string, error)"
	ranked := []types.RankedSymbol{
		{FilePath: "a.go", Name: "VeryLong", Line: 1, Signature: longSig, Score: 0.9},
	}

	result := Render(ranked, 1, 1, RenderConfig{TokenBudget: 1000})

	// Each rendered line should be <= 100 chars.
	for _, line := range splitLines(result.Map) {
		assert.LessOrEqual(t, len(line), maxLineLength, "line too long: %s", line)
	}
}

func TestRender_EmptyRanked(t *testing.T) {
	result := Render(nil, 0, 0, RenderConfig{TokenBudget: 1000})

	assert.Contains(t, result.Map, "0/0 files")
	assert.Equal(t, 0, result.FileCount)
	assert.Equal(t, 0, result.SymCount)
}

func TestBuildMap_Integration(t *testing.T) {
	dir := setupTestRepo(t, map[string]string{
		"math.go": `package main

func Add(a, b int) int { return a + b }

func Subtract(a, b int) int { return a - b }
`,
		"main.go": `package main

func main() {
	Add(1, 2)
	Subtract(3, 1)
}
`,
	})

	result, err := BuildMap(context.Background(), dir, []string{"math.go"}, 1000)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Map)
	assert.True(t, result.FileCount > 0)
	assert.True(t, result.SymCount > 0)
	assert.LessOrEqual(t, result.TokensUsed, 1000.0)
	assert.Contains(t, result.Map, "Repository map")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
