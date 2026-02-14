// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-edit-format R3;
//
//	docs/ARCHITECTURE § Edit Format Parser.
package editformat

import (
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// RouteResult holds the outcome of applying all edits through the router.
type RouteResult struct {
	Applied []*types.ApplyResult // Successful edits
	Errors  []error              // Errors from failed edits (in order)
}

// Router dispatches edits to the appropriate Applier based on file extension.
// Go files (.go) are routed to the AST engine; everything else goes to the
// text editor.
//
// Implements: prd004-edit-format R3.1-R3.5.
type Router struct {
	ASTApplier  types.Applier // Applier for .go files
	TextApplier types.Applier // Applier for everything else
}

// ApplyAll applies each edit through the appropriate engine, in order.
// If one edit fails, the router continues with remaining edits and
// collects all errors.
//
// Implements: prd004-edit-format R3.4, R3.5.
func (r *Router) ApplyAll(edits []types.Edit) *RouteResult {
	result := &RouteResult{}

	for _, edit := range edits {
		applier := r.applierFor(edit.FilePath)
		ar, err := applier.Apply(edit)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Applied = append(result.Applied, ar)
	}

	return result
}

// applierFor returns the appropriate Applier for the given file path.
// .go files use the AST engine; everything else uses the text editor.
//
// Implements: prd004-edit-format R3.1-R3.3.
func (r *Router) applierFor(filePath string) types.Applier {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".go" {
		return r.ASTApplier
	}
	return r.TextApplier
}
