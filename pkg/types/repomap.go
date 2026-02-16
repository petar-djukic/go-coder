// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-repo-map R1.4, R3.5, R4;
//
//	docs/ARCHITECTURE § Repository Map.
package types

// SymbolRef represents a symbol extracted from a source file, used by the
// repository map to build the dependency graph.
type SymbolRef struct {
	Name     string // Symbol name
	FilePath string // Source file path (relative to repo root)
	Line     int    // Line number (1-based)
	Kind     RefKind
}

// RefKind distinguishes symbol definitions from references.
type RefKind int

const (
	Definition RefKind = iota
	Reference
)

// RankedSymbol is a symbol with its PageRank score.
type RankedSymbol struct {
	FilePath  string
	Name      string
	Line      int
	Signature string
	Score     float64
}

// RepoMapResult holds the rendered repository map and metadata.
type RepoMapResult struct {
	Map        string  // Rendered map text
	FileCount  int     // Number of files in the map
	TotalFiles int     // Total files in the repository
	SymCount   int     // Number of symbols in the map
	TotalSyms  int     // Total symbols extracted
	TokensUsed float64 // Estimated token count of the map
}
