// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-repo-map R3;
//
//	docs/ARCHITECTURE § Repository Map.
package repomap

import (
	"math"
	"sort"

	"github.com/petar-djukic/go-coder/pkg/types"
)

const (
	defaultDamping    = 0.85
	defaultMaxIter    = 100
	defaultTolerance  = 1e-6
	personalizeFactor = 100.0
)

// RankConfig configures PageRank computation.
type RankConfig struct {
	Damping            float64  // Damping factor (default 0.85)
	MaxIterations      int      // Maximum iterations (default 100)
	Tolerance          float64  // Convergence tolerance (default 1e-6)
	PersonalizedFiles  []string // Files that receive 100x personalization weight
}

// Rank runs PageRank on the dependency graph and returns symbols ranked
// by score, highest first.
//
// Implements: prd006-repo-map R3.1-R3.5.
func Rank(g *Graph, symbols []types.SymbolRef, cfg RankConfig) []types.RankedSymbol {
	damping := cfg.Damping
	if damping == 0 {
		damping = defaultDamping
	}
	maxIter := cfg.MaxIterations
	if maxIter == 0 {
		maxIter = defaultMaxIter
	}
	tolerance := cfg.Tolerance
	if tolerance == 0 {
		tolerance = defaultTolerance
	}

	n := len(g.Nodes)
	if n == 0 {
		return nil
	}

	// Map file paths to indices.
	idx := make(map[string]int, n)
	for i, node := range g.Nodes {
		idx[node] = i
	}

	// Build personalization vector.
	personalSet := make(map[string]bool, len(cfg.PersonalizedFiles))
	for _, f := range cfg.PersonalizedFiles {
		personalSet[f] = true
	}

	personalization := make([]float64, n)
	totalPersonal := 0.0
	for i, node := range g.Nodes {
		if personalSet[node] {
			personalization[i] = personalizeFactor
		} else {
			personalization[i] = 1.0
		}
		totalPersonal += personalization[i]
	}
	// Normalize.
	for i := range personalization {
		personalization[i] /= totalPersonal
	}

	// Build adjacency: outgoing edges from each node with total weight.
	type outEdge struct {
		to     int
		weight float64
	}
	outEdges := make([][]outEdge, n)
	outWeight := make([]float64, n)

	for _, e := range g.Edges {
		fromIdx, okF := idx[e.From]
		toIdx, okT := idx[e.To]
		if !okF || !okT {
			continue
		}
		outEdges[fromIdx] = append(outEdges[fromIdx], outEdge{to: toIdx, weight: e.Weight})
		outWeight[fromIdx] += e.Weight
	}

	// Initialize PageRank uniformly.
	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1.0 / float64(n)
	}

	// Iterate.
	newRank := make([]float64, n)
	for iter := 0; iter < maxIter; iter++ {
		// Teleportation component.
		for i := range newRank {
			newRank[i] = (1.0 - damping) * personalization[i]
		}

		// Link contribution.
		for i := 0; i < n; i++ {
			if outWeight[i] == 0 {
				// Dangling node: distribute rank evenly via personalization.
				for j := range newRank {
					newRank[j] += damping * rank[i] * personalization[j]
				}
				continue
			}
			for _, e := range outEdges[i] {
				share := rank[i] * (e.weight / outWeight[i])
				newRank[e.to] += damping * share
			}
		}

		// Check convergence.
		diff := 0.0
		for i := range rank {
			diff += math.Abs(newRank[i] - rank[i])
		}
		copy(rank, newRank)
		if diff < tolerance {
			break
		}
	}

	// Build ranked symbol list: assign file rank to each definition.
	defsByFile := make(map[string][]types.SymbolRef)
	for _, s := range symbols {
		if s.Kind == types.Definition {
			defsByFile[s.FilePath] = append(defsByFile[s.FilePath], s)
		}
	}

	var ranked []types.RankedSymbol
	for file, defs := range defsByFile {
		fileIdx, ok := idx[file]
		if !ok {
			continue
		}
		score := rank[fileIdx]
		for _, d := range defs {
			ranked = append(ranked, types.RankedSymbol{
				FilePath: d.FilePath,
				Name:     d.Name,
				Line:     d.Line,
				Score:    score,
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if ranked[i].FilePath != ranked[j].FilePath {
			return ranked[i].FilePath < ranked[j].FilePath
		}
		return ranked[i].Line < ranked[j].Line
	})

	return ranked
}
