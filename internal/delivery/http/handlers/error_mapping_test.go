package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestHandlersDelegateUseCaseErrorsToCentralMapper(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handlers directory: %v", err)
	}

	totalBranches := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_handler.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports in %s: %v", entry.Name(), err)
		}
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", entry.Name(), err)
			}
			if path == "errors" || path == "github.com/motixo/goat-api/internal/domain/errors" {
				t.Errorf("%s imports %s; handlers must not classify business errors", entry.Name(), path)
			}
		}

		file, err = parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if isPackageCall(call, "response", "DomainError") || isPackageCall(call, "response", "LogoutError") {
				t.Errorf("%s calls an obsolete error renderer; use WriteProblem(MapError(err))", entry.Name())
			}
			return true
		})

		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			totalBranches += verifyUseCaseErrorBranches(t, entry.Name(), function.Body)
		}
	}

	if totalBranches == 0 {
		t.Fatal("no handler use-case error branches were verified")
	}
}

func verifyUseCaseErrorBranches(t *testing.T, filename string, block *ast.BlockStmt) int {
	t.Helper()
	useCaseResults := make(map[string]bool)
	verified := 0

	for _, statement := range block.List {
		switch typed := statement.(type) {
		case *ast.AssignStmt:
			updateUseCaseResults(useCaseResults, typed)
		case *ast.IfStmt:
			branchResults := cloneResults(useCaseResults)
			if assignment, ok := typed.Init.(*ast.AssignStmt); ok {
				updateUseCaseResults(branchResults, assignment)
			}
			tracked := referencedResults(typed.Cond, branchResults)
			if len(tracked) == 0 {
				continue
			}
			verified++
			if !containsMappedProblemWriter(typed.Body, tracked) {
				t.Errorf("%s has a use-case error branch that does not call response.WriteProblem(c, response.MapError(err))", filename)
			}
		}
	}

	return verified
}

func updateUseCaseResults(results map[string]bool, assignment *ast.AssignStmt) {
	fromUseCase := containsUseCaseCall(assignment)
	for _, expression := range assignment.Lhs {
		identifier, ok := expression.(*ast.Ident)
		if ok && identifier.Name != "_" {
			results[identifier.Name] = fromUseCase
		}
	}
}

func containsUseCaseCall(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(candidate ast.Node) bool {
		call, ok := candidate.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := method.X.(*ast.SelectorExpr)
		if ok && receiver.Sel.Name == "usecase" {
			found = true
			return false
		}
		return true
	})
	return found
}

func referencedResults(condition ast.Expr, results map[string]bool) map[string]bool {
	referenced := make(map[string]bool)
	ast.Inspect(condition, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && results[identifier.Name] {
			referenced[identifier.Name] = true
		}
		return true
	})
	return referenced
}

func containsMappedProblemWriter(block *ast.BlockStmt, errorNames map[string]bool) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		writer, ok := node.(*ast.CallExpr)
		if !ok || !isPackageCall(writer, "response", "WriteProblem") || len(writer.Args) != 2 {
			return true
		}
		mapper, ok := writer.Args[1].(*ast.CallExpr)
		if !ok || !isPackageCall(mapper, "response", "MapError") || len(mapper.Args) != 1 {
			return true
		}
		identifier, ok := mapper.Args[0].(*ast.Ident)
		if ok && errorNames[identifier.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

func isPackageCall(call *ast.CallExpr, packageName, functionName string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != functionName {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == packageName
}

func cloneResults(results map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(results))
	for name, value := range results {
		cloned[name] = value
	}
	return cloned
}
