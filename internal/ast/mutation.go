// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

// ErrFunctionNotFound is returned when a function name does not exist in the AST.
var ErrFunctionNotFound = fmt.Errorf("function not found")

// NewCommentMap builds an ast.CommentMap that tracks comment attachment
// before any mutation. This map should be built once before mutations
// and used to re-attach comments afterward.
//
// Implements: prd002-ast-engine R5.2.
func NewCommentMap(fset *token.FileSet, file *ast.File) ast.CommentMap {
	return ast.NewCommentMap(fset, file, file.Comments)
}

// ReplaceFunctionBody locates the named function in the file and replaces
// its body with the provided Go statements. The replacement code is parsed
// as the body of a wrapper function, then the resulting statements are
// injected into the target function.
//
// Returns ErrFunctionNotFound if the function does not exist. Returns a
// parse error if the replacement code is invalid Go.
//
// Implements: prd002-ast-engine R3.1, R3.2, R3.3, R3.4, R3.11.
func ReplaceFunctionBody(fset *token.FileSet, file *ast.File, cmap ast.CommentMap, funcName string, newBody string) error {
	funcDecl := findFunc(file, funcName)
	if funcDecl == nil {
		return fmt.Errorf("%w: %s", ErrFunctionNotFound, funcName)
	}

	stmts, err := parseStatements(fset, newBody)
	if err != nil {
		return fmt.Errorf("parsing replacement code: %w", err)
	}

	// Remove comments attached to the old body statements.
	removeBodyComments(cmap, funcDecl.Body)

	funcDecl.Body.List = stmts

	// Re-attach the comment map to the file.
	file.Comments = cmap.Filter(file).Comments()

	return nil
}

// AddFunction parses a complete function source string and appends it to
// the file's declaration list.
//
// Implements: prd002-ast-engine R3.5, R3.6.
func AddFunction(fset *token.FileSet, file *ast.File, cmap ast.CommentMap, funcSource string) error {
	// Wrap in a package declaration so parser.ParseFile works.
	wrapped := "package _\n\n" + funcSource
	parsed, err := parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing function source: %w", err)
	}

	for _, decl := range parsed.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			file.Decls = append(file.Decls, fd)
			// Bring over doc comments.
			if fd.Doc != nil {
				file.Comments = append(file.Comments, fd.Doc)
			}
		}
	}

	return nil
}

// RemoveFunction removes a function declaration by name from the file.
// Returns ErrFunctionNotFound if the function does not exist.
//
// Implements: prd002-ast-engine R3.7.
func RemoveFunction(file *ast.File, cmap ast.CommentMap, funcName string) error {
	idx := -1
	for i, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == funcName {
			idx = i
			// Remove associated comments from the comment map.
			delete(cmap, fd)
			if fd.Doc != nil {
				removeCommentGroup(file, fd.Doc)
			}
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("%w: %s", ErrFunctionNotFound, funcName)
	}

	file.Decls = append(file.Decls[:idx], file.Decls[idx+1:]...)
	file.Comments = cmap.Filter(file).Comments()

	return nil
}

// StructFieldOp describes an operation to perform on a struct field.
type StructFieldOp struct {
	Action string // "add", "remove", or "rename"
	Name   string // Field name (target for remove/rename, new field for add)
	Type   string // Go type expression (for add)
	Tag    string // Struct tag (for add, optional)
	NewName string // New name (for rename)
}

// ModifyStruct applies field operations to a named struct type.
// Returns an error if the struct is not found.
//
// Implements: prd002-ast-engine R3.8.
func ModifyStruct(fset *token.FileSet, file *ast.File, structName string, ops []StructFieldOp) error {
	structType := findStruct(file, structName)
	if structType == nil {
		return fmt.Errorf("struct not found: %s", structName)
	}

	for _, op := range ops {
		switch op.Action {
		case "add":
			if err := addStructField(fset, structType, op); err != nil {
				return fmt.Errorf("adding field %s: %w", op.Name, err)
			}
		case "remove":
			removeStructField(structType, op.Name)
		case "rename":
			renameStructField(structType, op.Name, op.NewName)
		default:
			return fmt.Errorf("unknown field operation: %s", op.Action)
		}
	}

	return nil
}

// AddImport adds an import path to the file. Uses astutil.AddImport which
// handles deduplication.
//
// Implements: prd002-ast-engine R3.9.
func AddImport(fset *token.FileSet, file *ast.File, path string) {
	astutil.AddImport(fset, file, path)
}

// RemoveImport removes an import path from the file. Uses astutil.DeleteImport
// which handles cleanup of empty import groups.
//
// Implements: prd002-ast-engine R3.10.
func RemoveImport(fset *token.FileSet, file *ast.File, path string) {
	astutil.DeleteImport(fset, file, path)
	ast.SortImports(fset, file)
}

// findFunc locates a function or method declaration by name.
func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

// findStruct locates a struct type declaration by name.
func findStruct(file *ast.File, name string) *ast.StructType {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

// parseStatements parses a string of Go statements by wrapping them in a
// function body. Returns the parsed statements.
func parseStatements(fset *token.FileSet, code string) ([]ast.Stmt, error) {
	wrapped := "package _\nfunc _() {\n" + code + "\n}"
	f, err := parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			return fd.Body.List, nil
		}
	}

	return nil, fmt.Errorf("no statements found in replacement code")
}

// removeBodyComments removes comment groups attached to statements inside
// a function body from the comment map.
func removeBodyComments(cmap ast.CommentMap, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	for _, stmt := range body.List {
		delete(cmap, stmt)
	}
}

// removeCommentGroup removes a specific comment group from the file's Comments slice.
func removeCommentGroup(file *ast.File, cg *ast.CommentGroup) {
	for i, c := range file.Comments {
		if c == cg {
			file.Comments = append(file.Comments[:i], file.Comments[i+1:]...)
			return
		}
	}
}

// addStructField adds a new field to a struct type.
func addStructField(fset *token.FileSet, st *ast.StructType, op StructFieldOp) error {
	typeExpr, err := parseTypeExpr(fset, op.Type)
	if err != nil {
		return err
	}

	field := &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(op.Name)},
		Type:  typeExpr,
	}
	if op.Tag != "" {
		field.Tag = &ast.BasicLit{
			Kind:  token.STRING,
			Value: op.Tag,
		}
	}

	st.Fields.List = append(st.Fields.List, field)
	return nil
}

// removeStructField removes a field by name from a struct type.
func removeStructField(st *ast.StructType, name string) {
	fields := make([]*ast.Field, 0, len(st.Fields.List))
	for _, field := range st.Fields.List {
		keep := true
		for _, ident := range field.Names {
			if ident.Name == name {
				keep = false
				break
			}
		}
		if keep {
			fields = append(fields, field)
		}
	}
	st.Fields.List = fields
}

// renameStructField renames a field in a struct type.
func renameStructField(st *ast.StructType, oldName, newName string) {
	for _, field := range st.Fields.List {
		for _, ident := range field.Names {
			if ident.Name == oldName {
				ident.Name = newName
				return
			}
		}
	}
}

// parseTypeExpr parses a Go type expression string into an ast.Expr.
func parseTypeExpr(fset *token.FileSet, typeStr string) (ast.Expr, error) {
	src := "package _\nvar _ " + typeStr
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing type %q: %w", typeStr, err)
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			return vs.Type, nil
		}
	}

	return nil, fmt.Errorf("failed to parse type expression: %s", typeStr)
}
