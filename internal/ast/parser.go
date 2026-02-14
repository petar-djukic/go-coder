// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/petar-djukic/go-coder/pkg/types"
)

// ExtractSymbols extracts all symbols from a parsed Go file.
// It recognizes functions, methods, structs, interfaces, variables,
// and constants. Each symbol includes its name, kind, position,
// signature, and doc comment.
//
// Implements: prd002-ast-engine R2.1 through R2.11.
func ExtractSymbols(fset *token.FileSet, filePath string, file *ast.File) []types.Symbol {
	var symbols []types.Symbol

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			symbols = append(symbols, extractFuncSymbol(fset, filePath, d))
		case *ast.GenDecl:
			symbols = append(symbols, extractGenDeclSymbols(fset, filePath, d)...)
		}
	}

	return symbols
}

// extractFuncSymbol extracts a symbol from a function or method declaration.
func extractFuncSymbol(fset *token.FileSet, filePath string, fn *ast.FuncDecl) types.Symbol {
	pos := fset.Position(fn.Pos())
	kind := types.Function
	if fn.Recv != nil {
		kind = types.Method
	}

	return types.Symbol{
		Name:      fn.Name.Name,
		Kind:      kind,
		FilePath:  filePath,
		Line:      pos.Line,
		Column:    pos.Column,
		Signature: funcSignature(fn),
		Doc:       docText(fn.Doc),
	}
}

// extractGenDeclSymbols extracts symbols from type, var, and const declarations.
func extractGenDeclSymbols(fset *token.FileSet, filePath string, gd *ast.GenDecl) []types.Symbol {
	var symbols []types.Symbol

	for _, spec := range gd.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			sym := extractTypeSymbol(fset, filePath, gd, s)
			symbols = append(symbols, sym)
		case *ast.ValueSpec:
			syms := extractValueSymbols(fset, filePath, gd, s)
			symbols = append(symbols, syms...)
		}
	}

	return symbols
}

// extractTypeSymbol extracts a symbol from a type declaration (struct, interface, or alias).
func extractTypeSymbol(fset *token.FileSet, filePath string, gd *ast.GenDecl, ts *ast.TypeSpec) types.Symbol {
	pos := fset.Position(ts.Pos())

	var kind types.SymbolKind
	var sig string

	switch t := ts.Type.(type) {
	case *ast.StructType:
		kind = types.Struct
		sig = structSignature(t)
	case *ast.InterfaceType:
		kind = types.Interface
		sig = interfaceSignature(t)
	default:
		// Type alias or other type definition.
		kind = types.Struct
		sig = fmt.Sprintf("type %s", exprString(ts.Type))
	}

	// Prefer the spec's doc comment; fall back to the GenDecl doc.
	doc := docText(ts.Doc)
	if doc == "" {
		doc = docText(gd.Doc)
	}

	return types.Symbol{
		Name:      ts.Name.Name,
		Kind:      kind,
		FilePath:  filePath,
		Line:      pos.Line,
		Column:    pos.Column,
		Signature: sig,
		Doc:       doc,
	}
}

// extractValueSymbols extracts symbols from var or const declarations.
func extractValueSymbols(fset *token.FileSet, filePath string, gd *ast.GenDecl, vs *ast.ValueSpec) []types.Symbol {
	kind := types.Variable
	if gd.Tok == token.CONST {
		kind = types.Constant
	}

	// Prefer the spec's doc comment; fall back to the GenDecl doc.
	doc := docText(vs.Doc)
	if doc == "" && len(gd.Specs) == 1 {
		doc = docText(gd.Doc)
	}

	var symbols []types.Symbol
	for _, name := range vs.Names {
		pos := fset.Position(name.Pos())
		sig := ""
		if vs.Type != nil {
			sig = exprString(vs.Type)
		}
		symbols = append(symbols, types.Symbol{
			Name:      name.Name,
			Kind:      kind,
			FilePath:  filePath,
			Line:      pos.Line,
			Column:    pos.Column,
			Signature: sig,
			Doc:       doc,
		})
	}

	return symbols
}

// funcSignature builds a human-readable signature string for a function or method.
func funcSignature(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func")

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(exprString(fn.Recv.List[0].Type))
		b.WriteString(") ")
	}

	b.WriteString("(")
	b.WriteString(fieldListString(fn.Type.Params))
	b.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := fieldListString(fn.Type.Results)
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			b.WriteString(" ")
			b.WriteString(results)
		} else {
			b.WriteString(" (")
			b.WriteString(results)
			b.WriteString(")")
		}
	}

	return b.String()
}

// structSignature builds a signature listing field names and types.
func structSignature(st *ast.StructType) string {
	if st.Fields == nil || len(st.Fields.List) == 0 {
		return "struct{}"
	}

	var parts []string
	for _, field := range st.Fields.List {
		typeStr := exprString(field.Type)
		if len(field.Names) == 0 {
			// Embedded field.
			parts = append(parts, typeStr)
		} else {
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}
	}
	return "struct { " + strings.Join(parts, "; ") + " }"
}

// interfaceSignature builds a signature listing method signatures.
func interfaceSignature(iface *ast.InterfaceType) string {
	if iface.Methods == nil || len(iface.Methods.List) == 0 {
		return "interface{}"
	}

	var parts []string
	for _, method := range iface.Methods.List {
		if len(method.Names) > 0 {
			// Named method.
			if ft, ok := method.Type.(*ast.FuncType); ok {
				sig := method.Names[0].Name + "(" + fieldListString(ft.Params) + ")"
				if ft.Results != nil && len(ft.Results.List) > 0 {
					results := fieldListString(ft.Results)
					if len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0 {
						sig += " " + results
					} else {
						sig += " (" + results + ")"
					}
				}
				parts = append(parts, sig)
			}
		} else {
			// Embedded interface.
			parts = append(parts, exprString(method.Type))
		}
	}
	return "interface { " + strings.Join(parts, "; ") + " }"
}

// fieldListString renders a field list as a comma-separated string.
func fieldListString(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}

	var parts []string
	for _, field := range fl.List {
		typeStr := exprString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeStr)
		} else {
			for _, name := range field.Names {
				parts = append(parts, name.Name+" "+typeStr)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// exprString renders an AST expression as a string.
func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprString(e.Elt)
		}
		return "[" + exprString(e.Len) + "]" + exprString(e.Elt)
	case *ast.MapType:
		return "map[" + exprString(e.Key) + "]" + exprString(e.Value)
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		sig := "func(" + fieldListString(e.Params) + ")"
		if e.Results != nil && len(e.Results.List) > 0 {
			sig += " (" + fieldListString(e.Results) + ")"
		}
		return sig
	case *ast.Ellipsis:
		return "..." + exprString(e.Elt)
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + exprString(e.Value)
		case ast.RECV:
			return "<-chan " + exprString(e.Value)
		default:
			return "chan " + exprString(e.Value)
		}
	case *ast.BasicLit:
		return e.Value
	case *ast.ParenExpr:
		return "(" + exprString(e.X) + ")"
	case *ast.IndexExpr:
		return exprString(e.X) + "[" + exprString(e.Index) + "]"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// docText extracts the text from a comment group, trimming whitespace.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}
