// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"errors"
	"go/format"
	goast "go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mutationTestSource = `package example

import "fmt"

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference.
func Subtract(a, b int) int {
	// subtract logic
	return a - b
}

// Greet prints a greeting.
func Greet(name string) {
	fmt.Println("hello", name)
}

// Config holds settings.
type Config struct {
	Name    string
	Timeout int
}
`

func parseMutationFixture(t *testing.T) (*token.FileSet, *goast.File, goast.CommentMap) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", mutationTestSource, parser.ParseComments)
	require.NoError(t, err)
	cmap := NewCommentMap(fset, file)
	return fset, file, cmap
}

func TestReplaceFunctionBody_Success(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	// Verify the function body changed.
	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "a + b + 1")
	assert.NotContains(t, string(out), "return a + b\n")
}

func TestReplaceFunctionBody_Compiles(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	dir := t.TempDir()
	outPath := filepath.Join(dir, "example.go")
	err = WriteFile(fset, file, outPath)
	require.NoError(t, err)

	// Verify the output is valid Go by re-parsing.
	outFset := token.NewFileSet()
	_, err = parser.ParseFile(outFset, outPath, nil, parser.ParseComments)
	assert.NoError(t, err, "output should be parseable Go")
}

func TestReplaceFunctionBody_DocCommentsPreserved(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "// Add returns the sum of a and b.")
}

func TestReplaceFunctionBody_InlineCommentsPreserved(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	// Replace Add, but Subtract's inline comment should survive.
	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "// subtract logic")
}

func TestReplaceFunctionBody_GofmtCompliant(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)

	formatted, err := format.Source(out)
	require.NoError(t, err)

	assert.Equal(t, string(formatted), string(out),
		"output should be identical to gofmt-formatted output")
}

func TestReplaceFunctionBody_FunctionNotFound(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "DoesNotExist", "return nil")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFunctionNotFound))
	assert.Contains(t, err.Error(), "DoesNotExist")
}

func TestReplaceFunctionBody_InvalidCode(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	// Save the original body for comparison.
	origOut, err := FormatFile(fset, file)
	require.NoError(t, err)

	err = ReplaceFunctionBody(fset, file, cmap, "Add", "return {{{ invalid syntax")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing replacement code")

	// AST should be unchanged.
	afterOut, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Equal(t, string(origOut), string(afterOut), "AST should be unchanged after invalid replacement")
}

func TestWriteFile_AtomicWrite(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := ReplaceFunctionBody(fset, file, cmap, "Add", "return a + b + 1")
	require.NoError(t, err)

	dir := t.TempDir()
	outPath := filepath.Join(dir, "example.go")

	// Write the original first.
	require.NoError(t, os.WriteFile(outPath, []byte(mutationTestSource), 0o644))
	origContent, err := os.ReadFile(outPath)
	require.NoError(t, err)

	// Write to an invalid path (directory instead of file) to trigger error.
	invalidPath := filepath.Join(dir, "subdir")
	require.NoError(t, os.MkdirAll(invalidPath, 0o755))

	err = WriteFile(fset, file, invalidPath)
	assert.Error(t, err)

	// Original file should be unchanged.
	afterContent, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, string(origContent), string(afterContent),
		"original file should be unchanged after failed write")
}

func TestWriteFile_PreservesPermissions(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	dir := t.TempDir()
	outPath := filepath.Join(dir, "example.go")

	// Create a file with specific permissions.
	require.NoError(t, os.WriteFile(outPath, []byte("package x\n"), 0o755))

	err := WriteFile(fset, file, outPath)
	require.NoError(t, err)

	info, err := os.Stat(outPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestAddFunction(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	funcSrc := `// Multiply returns the product.
func Multiply(a, b int) int {
	return a * b
}`
	err := AddFunction(fset, file, cmap, funcSrc)
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "func Multiply(a, b int) int")
	assert.Contains(t, string(out), "return a * b")
}

func TestAddFunction_InvalidSource(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := AddFunction(fset, file, cmap, "func broken({{{}}")
	assert.Error(t, err)
}

func TestRemoveFunction(t *testing.T) {
	fset, file, cmap := parseMutationFixture(t)

	err := RemoveFunction(file, cmap, "Greet")
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "func Greet")
	// Other functions remain.
	assert.Contains(t, string(out), "func Add")
	assert.Contains(t, string(out), "func Subtract")
}

func TestRemoveFunction_NotFound(t *testing.T) {
	_, file, cmap := parseMutationFixture(t)

	err := RemoveFunction(file, cmap, "NonExistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFunctionNotFound))
}

func TestModifyStruct_AddField(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	ops := []StructFieldOp{
		{Action: "add", Name: "Debug", Type: "bool"},
	}
	err := ModifyStruct(fset, file, "Config", ops)
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "Debug")
	assert.Contains(t, string(out), "bool")
	// Existing fields preserved.
	assert.Contains(t, string(out), "Name")
	assert.Contains(t, string(out), "Timeout")
}

func TestModifyStruct_RemoveField(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	ops := []StructFieldOp{
		{Action: "remove", Name: "Timeout"},
	}
	err := ModifyStruct(fset, file, "Config", ops)
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "Timeout")
	assert.Contains(t, string(out), "Name")
}

func TestModifyStruct_RenameField(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	ops := []StructFieldOp{
		{Action: "rename", Name: "Timeout", NewName: "TimeoutMs"},
	}
	err := ModifyStruct(fset, file, "Config", ops)
	require.NoError(t, err)

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), "TimeoutMs")
	assert.NotContains(t, string(out), "\tTimeout ")
}

func TestModifyStruct_NotFound(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	ops := []StructFieldOp{{Action: "add", Name: "X", Type: "int"}}
	err := ModifyStruct(fset, file, "NonExistent", ops)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "struct not found")
}

func TestAddImport(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	AddImport(fset, file, "context")

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"context"`)
}

func TestAddImport_NoDuplicate(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	// "fmt" already imported.
	AddImport(fset, file, "fmt")

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(out), `"fmt"`),
		"should not have duplicate fmt import")
}

func TestRemoveImport(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	RemoveImport(fset, file, "fmt")

	out, err := FormatFile(fset, file)
	require.NoError(t, err)
	assert.NotContains(t, string(out), `"fmt"`)
}

func TestWriteFile_CreatesDirectories(t *testing.T) {
	fset, file, _ := parseMutationFixture(t)

	dir := t.TempDir()
	outPath := filepath.Join(dir, "sub", "deep", "example.go")

	err := WriteFile(fset, file, outPath)
	require.NoError(t, err)

	_, err = os.Stat(outPath)
	assert.NoError(t, err, "file should exist at nested path")
}
