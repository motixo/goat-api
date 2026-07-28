package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestConfigurationIsLoadedOnceAtTheProcessBoundary(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read composition-root files: %v", err)
	}

	loadCalls := 0
	fileset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, parseErr := parser.ParseFile(fileset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		configAliases := make(map[string]struct{})
		for _, importSpec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(importSpec.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("parse import in %s: %v", path, unquoteErr)
			}
			if importPath != "github.com/motixo/goat-api/internal/config" {
				continue
			}
			alias := "config"
			if importSpec.Name != nil {
				alias = importSpec.Name.Name
			}
			configAliases[alias] = struct{}{}
		}
		if len(configAliases) == 0 {
			continue
		}

		file, parseErr = parser.ParseFile(fileset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Load" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, isConfigPackage := configAliases[identifier.Name]; isConfigPackage {
				loadCalls++
			}
			return true
		})
	}

	if loadCalls != 1 {
		t.Fatalf("config.Load call count = %d, want exactly 1 at the process boundary", loadCalls)
	}
}
