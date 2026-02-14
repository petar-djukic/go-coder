// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"go/ast"
	"go/token"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// SymbolTable holds all symbols extracted from a set of parsed Go files
// and provides lookup operations by name, file, and kind.
//
// Implements: prd002-ast-engine R2.5 through R2.7.
type SymbolTable struct {
	symbols []types.Symbol
	byName  map[string][]int
	byFile  map[string][]int
	byKind  map[types.SymbolKind][]int
}

// BuildSymbolTable creates a SymbolTable by extracting symbols from every
// parsed file in the AST map.
//
// Implements: prd002-ast-engine R2.1 through R2.4.
func BuildSymbolTable(fset *token.FileSet, files map[string]*ast.File) *SymbolTable {
	st := &SymbolTable{
		byName: make(map[string][]int),
		byFile: make(map[string][]int),
		byKind: make(map[types.SymbolKind][]int),
	}

	for filePath, file := range files {
		syms := ExtractSymbols(fset, filePath, file)
		for _, sym := range syms {
			idx := len(st.symbols)
			st.symbols = append(st.symbols, sym)
			st.byName[sym.Name] = append(st.byName[sym.Name], idx)
			st.byFile[sym.FilePath] = append(st.byFile[sym.FilePath], idx)
			st.byKind[sym.Kind] = append(st.byKind[sym.Kind], idx)
		}
	}

	return st
}

// All returns every symbol in the table.
func (st *SymbolTable) All() []types.Symbol {
	result := make([]types.Symbol, len(st.symbols))
	copy(result, st.symbols)
	return result
}

// ByName returns all symbols with the given name.
func (st *SymbolTable) ByName(name string) []types.Symbol {
	return st.lookup(st.byName[name])
}

// ByFile returns all symbols declared in the given file path.
func (st *SymbolTable) ByFile(filePath string) []types.Symbol {
	return st.lookup(st.byFile[filePath])
}

// ByKind returns all symbols of the given kind.
func (st *SymbolTable) ByKind(kind types.SymbolKind) []types.Symbol {
	return st.lookup(st.byKind[kind])
}

// Len returns the total number of symbols.
func (st *SymbolTable) Len() int {
	return len(st.symbols)
}

// lookup returns symbols at the given indices.
func (st *SymbolTable) lookup(indices []int) []types.Symbol {
	if len(indices) == 0 {
		return nil
	}
	result := make([]types.Symbol, len(indices))
	for i, idx := range indices {
		result[i] = st.symbols[idx]
	}
	return result
}
