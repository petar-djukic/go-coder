// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	goast "go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helperSource = `package lib

import "context"

// HelperFunc does something useful.
func HelperFunc(ctx context.Context, id string) (*Result, error) {
	return nil, nil
}

type Result struct {
	Data string
}
`

const typesSource = `package lib

// Config holds configuration settings.
type Config struct {
	Name    string
	Timeout int
}

// Processor defines an operation pipeline.
type Processor interface {
	Process(input string) (string, error)
}

// DefaultName is the default configuration name.
const DefaultName = "default"

// MaxRetries limits retry attempts.
var MaxRetries = 3

func (c *Config) Validate() error { return nil }
`

func buildTestSymbolTable(t *testing.T) *SymbolTable {
	t.Helper()
	fset := token.NewFileSet()

	helperFile, err := parser.ParseFile(fset, "lib/helper.go", helperSource, parser.ParseComments)
	require.NoError(t, err)

	typesFile, err := parser.ParseFile(fset, "lib/types.go", typesSource, parser.ParseComments)
	require.NoError(t, err)

	files := map[string]*goast.File{
		"lib/helper.go": helperFile,
		"lib/types.go":  typesFile,
	}

	return BuildSymbolTable(fset, files)
}

func TestSymbolTable_ByName(t *testing.T) {
	st := buildTestSymbolTable(t)

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantName  string
	}{
		{
			name:      "existing function",
			query:     "HelperFunc",
			wantCount: 1,
			wantName:  "HelperFunc",
		},
		{
			name:      "existing struct",
			query:     "Config",
			wantCount: 1,
			wantName:  "Config",
		},
		{
			name:      "nonexistent name",
			query:     "DoesNotExist",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := st.ByName(tt.query)
			assert.Len(t, results, tt.wantCount)
			for _, sym := range results {
				assert.Equal(t, tt.wantName, sym.Name)
			}
		})
	}
}

func TestSymbolTable_ByFile(t *testing.T) {
	st := buildTestSymbolTable(t)

	tests := []struct {
		name         string
		filePath     string
		wantMinCount int
		wantFile     string
		notFile      string
	}{
		{
			name:         "helper.go symbols",
			filePath:     "lib/helper.go",
			wantMinCount: 1,
			wantFile:     "lib/helper.go",
			notFile:      "lib/types.go",
		},
		{
			name:         "types.go symbols",
			filePath:     "lib/types.go",
			wantMinCount: 1,
			wantFile:     "lib/types.go",
			notFile:      "lib/helper.go",
		},
		{
			name:         "nonexistent file",
			filePath:     "does/not/exist.go",
			wantMinCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := st.ByFile(tt.filePath)
			assert.GreaterOrEqual(t, len(results), tt.wantMinCount)
			for _, sym := range results {
				assert.Equal(t, tt.wantFile, sym.FilePath)
				if tt.notFile != "" {
					assert.NotEqual(t, tt.notFile, sym.FilePath)
				}
			}
		})
	}
}

func TestSymbolTable_ByKind(t *testing.T) {
	st := buildTestSymbolTable(t)

	tests := []struct {
		name         string
		kind         types.SymbolKind
		wantMinCount int
		notKinds     []types.SymbolKind
	}{
		{
			name:         "functions only",
			kind:         types.Function,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Struct, types.Interface, types.Variable, types.Constant},
		},
		{
			name:         "structs only",
			kind:         types.Struct,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Function, types.Interface},
		},
		{
			name:         "interfaces only",
			kind:         types.Interface,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Function, types.Struct},
		},
		{
			name:         "constants only",
			kind:         types.Constant,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Function, types.Variable},
		},
		{
			name:         "variables only",
			kind:         types.Variable,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Function, types.Constant},
		},
		{
			name:         "methods only",
			kind:         types.Method,
			wantMinCount: 1,
			notKinds:     []types.SymbolKind{types.Function, types.Struct},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := st.ByKind(tt.kind)
			assert.GreaterOrEqual(t, len(results), tt.wantMinCount)
			for _, sym := range results {
				assert.Equal(t, tt.kind, sym.Kind)
				for _, notKind := range tt.notKinds {
					assert.NotEqual(t, notKind, sym.Kind,
						"symbol %q should not be kind %s", sym.Name, notKind)
				}
			}
		})
	}
}

func TestSymbolTable_All(t *testing.T) {
	st := buildTestSymbolTable(t)

	all := st.All()
	assert.Equal(t, st.Len(), len(all))
	assert.Greater(t, len(all), 0)
}

func TestSymbolTable_Len(t *testing.T) {
	st := buildTestSymbolTable(t)

	// We expect: HelperFunc, Result (from helper.go),
	// Config, Processor, DefaultName, MaxRetries, Validate (from types.go).
	assert.GreaterOrEqual(t, st.Len(), 7)
}

func TestSymbolTable_Empty(t *testing.T) {
	fset := token.NewFileSet()
	st := BuildSymbolTable(fset, nil)

	assert.Equal(t, 0, st.Len())
	assert.Empty(t, st.All())
	assert.Empty(t, st.ByName("anything"))
	assert.Empty(t, st.ByFile("any.go"))
	assert.Empty(t, st.ByKind(types.Function))
}
