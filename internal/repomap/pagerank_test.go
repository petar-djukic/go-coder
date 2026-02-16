// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package repomap

import (
	"testing"

	"github.com/petar-djukic/go-coder/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGraph_CrossFileEdges(t *testing.T) {
	symbols := []types.SymbolRef{
		{Name: "Add", FilePath: "pkg/math/math.go", Line: 3, Kind: types.Definition},
		{Name: "Multiply", FilePath: "pkg/math/math.go", Line: 5, Kind: types.Definition},
		{Name: "main", FilePath: "cmd/main.go", Line: 7, Kind: types.Definition},
		{Name: "Add", FilePath: "cmd/main.go", Line: 9, Kind: types.Reference},
		{Name: "Multiply", FilePath: "cmd/main.go", Line: 10, Kind: types.Reference},
	}

	g := BuildGraph(symbols)

	assert.GreaterOrEqual(t, len(g.Edges), 2)

	// Check that edges go from main.go to math.go.
	var addEdge, mulEdge *Edge
	for i := range g.Edges {
		if g.Edges[i].Reference == "Add" {
			addEdge = &g.Edges[i]
		}
		if g.Edges[i].Reference == "Multiply" {
			mulEdge = &g.Edges[i]
		}
	}

	require.NotNil(t, addEdge, "expected edge for Add")
	assert.Equal(t, "cmd/main.go", addEdge.From)
	assert.Equal(t, "pkg/math/math.go", addEdge.To)

	require.NotNil(t, mulEdge, "expected edge for Multiply")
	assert.Equal(t, "cmd/main.go", mulEdge.From)
	assert.Equal(t, "pkg/math/math.go", mulEdge.To)
}

func TestBuildGraph_NoSelfEdges(t *testing.T) {
	symbols := []types.SymbolRef{
		{Name: "Add", FilePath: "math.go", Line: 1, Kind: types.Definition},
		{Name: "Add", FilePath: "math.go", Line: 5, Kind: types.Reference},
	}

	g := BuildGraph(symbols)
	assert.Empty(t, g.Edges, "self-references should not create edges")
}

func TestIdentifierWeight(t *testing.T) {
	tests := []struct {
		name string
		want float64
	}{
		{"Calculator", longNameWeight},       // 10 chars, >= 8
		{"Add", shortNameWeight},             // 3 chars, < 8
		{"_private", underscoreWeight},       // underscore prefix
		{"longFunc", longNameWeight},          // 8 chars, exactly threshold
		{"ab", shortNameWeight},               // 2 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, identifierWeight(tt.name))
		})
	}
}

func TestCommonWeight(t *testing.T) {
	defs := map[string][]string{
		"String": {"a.go", "b.go", "c.go", "d.go", "e.go"}, // 5 files, at threshold
		"Unique": {"a.go"},
	}

	assert.Equal(t, commonFactor, commonWeight("String", defs))
	assert.Equal(t, 1.0, commonWeight("Unique", defs))
}

func TestRank_PersonalizationBiasesRelevantFiles(t *testing.T) {
	symbols := []types.SymbolRef{
		{Name: "Add", FilePath: "pkg/math/math.go", Line: 3, Kind: types.Definition},
		{Name: "FormatNumber", FilePath: "pkg/util/format.go", Line: 3, Kind: types.Definition},
		{Name: "main", FilePath: "cmd/main.go", Line: 7, Kind: types.Definition},
		{Name: "Add", FilePath: "cmd/main.go", Line: 9, Kind: types.Reference},
		{Name: "FormatNumber", FilePath: "cmd/main.go", Line: 10, Kind: types.Reference},
	}

	g := BuildGraph(symbols)
	ranked := Rank(g, symbols, RankConfig{
		PersonalizedFiles: []string{"pkg/math/math.go"},
	})

	require.NotEmpty(t, ranked)

	// The personalized file (math.go) should appear before format.go.
	mathIdx := -1
	formatIdx := -1
	for i, r := range ranked {
		if r.FilePath == "pkg/math/math.go" && mathIdx == -1 {
			mathIdx = i
		}
		if r.FilePath == "pkg/util/format.go" && formatIdx == -1 {
			formatIdx = i
		}
	}

	require.NotEqual(t, -1, mathIdx, "math.go should be in ranked results")
	require.NotEqual(t, -1, formatIdx, "format.go should be in ranked results")
	assert.Less(t, mathIdx, formatIdx, "personalized file should rank higher")
}

func TestRank_EmptyGraph(t *testing.T) {
	g := &Graph{}
	ranked := Rank(g, nil, RankConfig{})
	assert.Empty(t, ranked)
}

func TestRank_ConvergesForSimpleGraph(t *testing.T) {
	symbols := []types.SymbolRef{
		{Name: "A", FilePath: "a.go", Line: 1, Kind: types.Definition},
		{Name: "B", FilePath: "b.go", Line: 1, Kind: types.Definition},
		{Name: "A", FilePath: "b.go", Line: 3, Kind: types.Reference},
		{Name: "B", FilePath: "a.go", Line: 3, Kind: types.Reference},
	}

	g := BuildGraph(symbols)
	ranked := Rank(g, symbols, RankConfig{})

	require.Len(t, ranked, 2)
	// Both files reference each other symmetrically, scores should be equal.
	assert.InDelta(t, ranked[0].Score, ranked[1].Score, 0.01)
}
