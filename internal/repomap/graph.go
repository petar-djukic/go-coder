// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-repo-map R2;
//
//	docs/ARCHITECTURE § Repository Map.
package repomap

import (
	"github.com/petar-djukic/go-coder/pkg/types"
)

const (
	longNameThreshold = 8
	longNameWeight    = 1.0
	shortNameWeight   = 0.5
	underscoreWeight  = 0.1
	commonThreshold   = 5
	commonFactor      = 0.1
)

// Edge represents a directed edge in the dependency graph.
type Edge struct {
	From      string  // Source file path
	To        string  // Target file path (where symbol is defined)
	Reference string  // Symbol name
	Weight    float64 // Edge weight based on identifier quality
}

// Graph is a directed multigraph where nodes are files and edges
// represent cross-file symbol references.
//
// Implements: prd006-repo-map R2.1-R2.5.
type Graph struct {
	Nodes []string          // All file paths
	Edges []Edge            // All edges
	defs  map[string][]string // symbol name → list of files where it is defined
}

// BuildGraph constructs the dependency graph from extracted symbols.
//
// Implements: prd006-repo-map R2.1-R2.5.
func BuildGraph(symbols []types.SymbolRef) *Graph {
	g := &Graph{
		defs: make(map[string][]string),
	}

	nodeSet := make(map[string]bool)

	// Index definitions: symbol name → defining files.
	for _, s := range symbols {
		nodeSet[s.FilePath] = true
		if s.Kind == types.Definition {
			g.defs[s.Name] = append(g.defs[s.Name], s.FilePath)
		}
	}

	// Build node list.
	for f := range nodeSet {
		g.Nodes = append(g.Nodes, f)
	}

	// Build edges: for each reference, create edges to all files that define it.
	// Count references per (from, to, symbol) to weight edges.
	type edgeKey struct {
		from, to, ref string
	}
	edgeCounts := make(map[edgeKey]int)

	for _, s := range symbols {
		if s.Kind != types.Reference {
			continue
		}
		defFiles, ok := g.defs[s.Name]
		if !ok {
			continue
		}
		for _, defFile := range defFiles {
			if defFile == s.FilePath {
				continue // Skip self-references.
			}
			key := edgeKey{from: s.FilePath, to: defFile, ref: s.Name}
			edgeCounts[key]++
		}
	}

	// Convert edge counts to weighted edges.
	for key, count := range edgeCounts {
		weight := float64(count) * identifierWeight(key.ref) * commonWeight(key.ref, g.defs)
		g.Edges = append(g.Edges, Edge{
			From:      key.from,
			To:        key.to,
			Reference: key.ref,
			Weight:    weight,
		})
	}

	return g
}

// identifierWeight scores a symbol name based on length and prefix.
//
// Implements: prd006-repo-map R2.4.
func identifierWeight(name string) float64 {
	if len(name) > 0 && name[0] == '_' {
		return underscoreWeight
	}
	if len(name) >= longNameThreshold {
		return longNameWeight
	}
	return shortNameWeight
}

// commonWeight reduces weight for symbols defined in many files.
//
// Implements: prd006-repo-map R2.5.
func commonWeight(name string, defs map[string][]string) float64 {
	if len(defs[name]) >= commonThreshold {
		return commonFactor
	}
	return 1.0
}
