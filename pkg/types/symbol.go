// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package types defines shared types used across go-coder packages.
// Implements: prd001-coder-interface R5 (shared types).
package types

// SymbolKind identifies the category of a code symbol.
type SymbolKind int

const (
	Function  SymbolKind = iota // Function declaration
	Method                      // Method declaration (has receiver)
	Struct                      // Struct type declaration
	Interface                   // Interface type declaration
	Variable                    // Package-level variable declaration
	Constant                    // Package-level constant declaration
)

// String returns the human-readable name of the symbol kind.
func (k SymbolKind) String() string {
	switch k {
	case Function:
		return "Function"
	case Method:
		return "Method"
	case Struct:
		return "Struct"
	case Interface:
		return "Interface"
	case Variable:
		return "Variable"
	case Constant:
		return "Constant"
	default:
		return "Unknown"
	}
}

// Symbol represents a code symbol extracted from a Go source file.
// Implements: prd001-coder-interface R5.3; prd002-ast-engine R2.8.
type Symbol struct {
	Name      string     // Symbol name
	Kind      SymbolKind // Category (function, struct, etc.)
	FilePath  string     // Source file path
	Line      int        // Line number (1-based)
	Column    int        // Column number (1-based)
	Signature string     // Type signature (function signature, struct fields, etc.)
	Doc       string     // Doc comment text
}
