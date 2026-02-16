// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-repo-map R4;
//
//	docs/ARCHITECTURE § Repository Map.
package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
)

const (
	defaultTokenRatio   = 0.25
	defaultTokenBudget  = 4096
	maxLineLength       = 100
)

// RenderConfig configures map rendering.
type RenderConfig struct {
	TokenBudget float64 // Maximum tokens for the map (default 4096)
	TokenRatio  float64 // Characters per token (default 0.25 tokens/char)
	WorkDir     string  // Repository root for reading signatures
}

// Render produces a compact text representation of the repository map from
// the ranked symbols, fitting within the token budget.
//
// Implements: prd006-repo-map R4.1-R4.5.
func Render(ranked []types.RankedSymbol, totalFiles, totalSyms int, cfg RenderConfig) *types.RepoMapResult {
	budget := cfg.TokenBudget
	if budget == 0 {
		budget = defaultTokenBudget
	}
	ratio := cfg.TokenRatio
	if ratio == 0 {
		ratio = defaultTokenRatio
	}

	// Group symbols by file, preserving rank order for files.
	type fileSym struct {
		name string
		line int
		sig  string
	}
	fileOrder := make([]string, 0)
	fileSyms := make(map[string][]fileSym)
	fileSeen := make(map[string]bool)

	for _, rs := range ranked {
		if !fileSeen[rs.FilePath] {
			fileSeen[rs.FilePath] = true
			fileOrder = append(fileOrder, rs.FilePath)
		}
		sig := rs.Signature
		if sig == "" && cfg.WorkDir != "" {
			sig = readSignature(cfg.WorkDir, rs.FilePath, rs.Line)
		}
		fileSyms[rs.FilePath] = append(fileSyms[rs.FilePath], fileSym{
			name: rs.Name,
			line: rs.Line,
			sig:  sig,
		})
	}

	// Build the map text, adding files until we hit the budget.
	var buf strings.Builder
	headerPlaceholder := strings.Repeat(" ", 80) + "\n" // Reserve space for header.
	buf.WriteString(headerPlaceholder)

	tokensUsed := float64(len(headerPlaceholder)) * ratio
	filesShown := 0
	symsShown := 0

	for _, file := range fileOrder {
		// Build file section.
		var section strings.Builder
		section.WriteString(file + "\n")

		syms := fileSyms[file]
		for _, s := range syms {
			line := fmt.Sprintf("  %s", s.sig)
			if line == "  " {
				line = fmt.Sprintf("  %s", s.name)
			}
			if len(line) > maxLineLength {
				line = line[:maxLineLength-3] + "..."
			}
			section.WriteString(line + "\n")
		}

		sectionText := section.String()
		sectionTokens := float64(len(sectionText)) * ratio

		if tokensUsed+sectionTokens > budget {
			break
		}

		buf.WriteString(sectionText)
		tokensUsed += sectionTokens
		filesShown++
		symsShown += len(syms)
	}

	// Replace the header placeholder with the actual header.
	header := fmt.Sprintf("Repository map (%d/%d files, %d/%d symbols)", filesShown, totalFiles, symsShown, totalSyms)
	mapText := header + "\n" + buf.String()[len(headerPlaceholder):]

	// Recalculate token count with actual header.
	tokensUsed = float64(len(mapText)) * ratio

	return &types.RepoMapResult{
		Map:        mapText,
		FileCount:  filesShown,
		TotalFiles: totalFiles,
		SymCount:   symsShown,
		TotalSyms:  totalSyms,
		TokensUsed: tokensUsed,
	}
}

// BuildMap is a convenience function that runs the full pipeline: extract,
// build graph, rank, and render.
func BuildMap(ctx context.Context, workDir string, personalizedFiles []string, tokenBudget float64) (*types.RepoMapResult, error) {
	ext := NewExtractor()
	symbols, stats, err := ext.ExtractAll(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("extracting symbols: %w", err)
	}

	graph := BuildGraph(symbols)
	ranked := Rank(graph, symbols, RankConfig{
		PersonalizedFiles: personalizedFiles,
	})

	// Load signatures for ranked symbols.
	for i := range ranked {
		if ranked[i].Signature == "" {
			ranked[i].Signature = readSignature(workDir, ranked[i].FilePath, ranked[i].Line)
		}
	}

	result := Render(ranked, stats.FilesProcessed+stats.FilesSkipped, len(filterDefs(symbols)), RenderConfig{
		TokenBudget: tokenBudget,
		WorkDir:     workDir,
	})

	return result, nil
}

// filterDefs returns only definition symbols.
func filterDefs(symbols []types.SymbolRef) []types.SymbolRef {
	var defs []types.SymbolRef
	for _, s := range symbols {
		if s.Kind == types.Definition {
			defs = append(defs, s)
		}
	}
	return defs
}

// readSignature reads the source line at the given line number.
func readSignature(workDir, relPath string, line int) string {
	absPath := filepath.Join(workDir, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	return getSignature(content, line)
}
