// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSource = `package example

import "context"

// HelperFunc does something useful.
func HelperFunc(ctx context.Context, id string) (*Result, error) {
	return nil, nil
}

// unexported is a private function.
func unexported() {}

// Config holds configuration settings.
type Config struct {
	Name    string
	Timeout int
}

// Processor defines an operation pipeline.
type Processor interface {
	Process(input string) (string, error)
	Reset()
}

// Result holds operation output.
type Result struct {
	Data string
}

// DefaultName is the default configuration name.
const DefaultName = "default"

// MaxRetries limits retry attempts.
var MaxRetries = 3

// Handle is a method on Result.
func (r *Result) Handle() error {
	return nil
}
`

func parseTestSource(t *testing.T) (*token.FileSet, []types.Symbol) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", testSource, parser.ParseComments)
	require.NoError(t, err)
	symbols := ExtractSymbols(fset, "example.go", file)
	return fset, symbols
}

func findSymbol(symbols []types.Symbol, name string) *types.Symbol {
	for i := range symbols {
		if symbols[i].Name == name {
			return &symbols[i]
		}
	}
	return nil
}

func TestExtractSymbols_Functions(t *testing.T) {
	_, symbols := parseTestSource(t)

	tests := []struct {
		name      string
		symName   string
		wantKind  types.SymbolKind
		wantSigSub string
		wantDoc   string
	}{
		{
			name:      "exported function with params and returns",
			symName:   "HelperFunc",
			wantKind:  types.Function,
			wantSigSub: "ctx context.Context, id string",
			wantDoc:   "HelperFunc does something useful.",
		},
		{
			name:      "unexported function",
			symName:   "unexported",
			wantKind:  types.Function,
			wantSigSub: "func()",
			wantDoc:   "unexported is a private function.",
		},
		{
			name:      "method with receiver",
			symName:   "Handle",
			wantKind:  types.Method,
			wantSigSub: "*Result",
			wantDoc:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := findSymbol(symbols, tt.symName)
			require.NotNil(t, sym, "symbol %q not found", tt.symName)
			assert.Equal(t, tt.wantKind, sym.Kind)
			assert.Contains(t, sym.Signature, tt.wantSigSub)
			assert.Equal(t, "example.go", sym.FilePath)
			assert.Greater(t, sym.Line, 0)
			if tt.wantDoc != "" {
				assert.Contains(t, sym.Doc, tt.wantDoc)
			}
		})
	}
}

func TestExtractSymbols_Types(t *testing.T) {
	_, symbols := parseTestSource(t)

	tests := []struct {
		name       string
		symName    string
		wantKind   types.SymbolKind
		wantSigSub string
	}{
		{
			name:       "struct with fields",
			symName:    "Config",
			wantKind:   types.Struct,
			wantSigSub: "Name string",
		},
		{
			name:       "struct fields include Timeout",
			symName:    "Config",
			wantKind:   types.Struct,
			wantSigSub: "Timeout int",
		},
		{
			name:       "interface with methods",
			symName:    "Processor",
			wantKind:   types.Interface,
			wantSigSub: "Process(input string)",
		},
		{
			name:       "simple struct",
			symName:    "Result",
			wantKind:   types.Struct,
			wantSigSub: "Data string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := findSymbol(symbols, tt.symName)
			require.NotNil(t, sym, "symbol %q not found", tt.symName)
			assert.Equal(t, tt.wantKind, sym.Kind)
			assert.Contains(t, sym.Signature, tt.wantSigSub)
		})
	}
}

func TestExtractSymbols_Values(t *testing.T) {
	_, symbols := parseTestSource(t)

	tests := []struct {
		name     string
		symName  string
		wantKind types.SymbolKind
	}{
		{
			name:     "constant",
			symName:  "DefaultName",
			wantKind: types.Constant,
		},
		{
			name:     "variable",
			symName:  "MaxRetries",
			wantKind: types.Variable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sym := findSymbol(symbols, tt.symName)
			require.NotNil(t, sym, "symbol %q not found", tt.symName)
			assert.Equal(t, tt.wantKind, sym.Kind)
			assert.Equal(t, "example.go", sym.FilePath)
			assert.Greater(t, sym.Line, 0)
		})
	}
}

func TestExtractSymbols_AllKindsPresent(t *testing.T) {
	_, symbols := parseTestSource(t)

	kindSet := make(map[types.SymbolKind]bool)
	for _, sym := range symbols {
		kindSet[sym.Kind] = true
	}

	assert.True(t, kindSet[types.Function], "should have Function")
	assert.True(t, kindSet[types.Method], "should have Method")
	assert.True(t, kindSet[types.Struct], "should have Struct")
	assert.True(t, kindSet[types.Interface], "should have Interface")
	assert.True(t, kindSet[types.Variable], "should have Variable")
	assert.True(t, kindSet[types.Constant], "should have Constant")
}

func TestExtractSymbols_DocComments(t *testing.T) {
	_, symbols := parseTestSource(t)

	sym := findSymbol(symbols, "Config")
	require.NotNil(t, sym)
	assert.Contains(t, sym.Doc, "Config holds configuration settings.")

	sym = findSymbol(symbols, "DefaultName")
	require.NotNil(t, sym)
	assert.Contains(t, sym.Doc, "DefaultName is the default configuration name.")
}
