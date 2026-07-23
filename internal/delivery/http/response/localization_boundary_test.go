package response

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLocalizationConcernsStayInsideHTTPDelivery(t *testing.T) {
	for _, root := range []string{"../../../domain", "../../../usecase", "../../../infra"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}

			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"Accept-Language", "TranslationKey", "TranslationParams"} {
				if strings.Contains(string(source), forbidden) {
					t.Errorf("%s contains %q; localization belongs to HTTP delivery", path, forbidden)
				}
			}

			parsed, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if strings.HasPrefix(importPath, "golang.org/x/text") {
					t.Errorf("%s imports %s; localization belongs to HTTP delivery", path, importPath)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("inspect %s: %v", root, err)
		}
	}
}

func TestHandlersAndMiddlewareDoNotOwnLocalizationHeaders(t *testing.T) {
	for _, root := range []string{"../handlers", "../middleware"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"Accept-Language", "Content-Language"} {
				if strings.Contains(string(source), forbidden) {
					t.Errorf("%s contains %q outside the shared problem writer", path, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("inspect %s: %v", root, err)
		}
	}
}

func TestOnlySharedWriterSerializesProblemResponses(t *testing.T) {
	expectedPath := filepath.Clean("../response/response.go")
	abortCalls := 0

	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "AbortWithStatusJSON" {
					return true
				}

				abortCalls++
				if filepath.Clean(path) != expectedPath || function.Name.Name != "WriteProblem" {
					t.Errorf("%s:%s serializes an aborted JSON response outside the shared Problem writer", path, function.Name.Name)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect HTTP delivery: %v", err)
	}
	if abortCalls != 1 {
		t.Fatalf("AbortWithStatusJSON calls = %d, want exactly one shared Problem writer", abortCalls)
	}
}

func TestHandlersAndMiddlewareUseTranslationKeysForProblems(t *testing.T) {
	localizedWriters := map[string]bool{
		"BadRequest":           true,
		"BadRequestWithParams": true,
		"Unauthorized":         true,
		"Forbidden":            true,
		"TooManyRequests":      true,
	}
	for _, root := range []string{"../handlers", "../middleware"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !localizedWriters[selector.Sel.Name] || len(call.Args) < 2 {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok || pkg.Name != "response" {
					return true
				}
				if literal, ok := call.Args[1].(*ast.BasicLit); ok && literal.Kind == token.STRING {
					t.Errorf("%s passes a hard-coded public message to response.%s", path, selector.Sel.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("inspect %s: %v", root, err)
		}
	}
}

func TestMapErrorDoesNotDependOnGinOrRequestContext(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "error_mapping.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse error_mapping.go: %v", err)
	}
	for _, imported := range parsed.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("decode import: %v", err)
		}
		if importPath == "github.com/gin-gonic/gin" {
			t.Fatalf("MapError imports Gin through %s", importPath)
		}
	}
}
