package main

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

func TestProductionHasNoAuthorizationCacheOrInvalidationEventReferences(t *testing.T) {
	t.Parallel()

	forbiddenImports := map[string]struct{}{
		"github.com/motixo/goat-api/internal/domain/event":           {},
		"github.com/motixo/goat-api/internal/infra/cache/permission": {},
		"github.com/motixo/goat-api/internal/infra/cache/user":       {},
		"github.com/motixo/goat-api/internal/infra/event":            {},
	}
	forbiddenIdentifiers := map[string]struct{}{
		"InMemoryPublisher":             {},
		"NewCachedRepository":           {},
		"PermissionUpdatedEvent":        {},
		"PermCacheService":              {},
		"RecordCacheHit":                {},
		"RecordEventPublicationFailure": {},
		"UserCacheService":              {},
		"UserUpdatedEvent":              {},
		"newConfiguredEventBus":         {},
		"permissionCacheRecord":         {},
		"userCacheRecord":               {},
	}
	forbiddenSourceFragments := []string{
		`RedisKey("perm", "role"`,
		`RedisKey("user", "id"`,
		"cache_hits_total",
		"event_publication_failures_total",
	}

	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(
			path string,
			entry fs.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() ||
				!strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}

			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, fragment := range forbiddenSourceFragments {
				if strings.Contains(string(source), fragment) {
					t.Errorf("%s references removed authorization infrastructure %q", path, fragment)
				}
			}

			parsed, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if _, forbidden := forbiddenImports[importPath]; forbidden {
					t.Errorf("%s imports removed authorization infrastructure %s", path, importPath)
				}
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				identifier, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				if _, forbidden := forbiddenIdentifiers[identifier.Name]; forbidden {
					t.Errorf("%s references removed authorization infrastructure %s", path, identifier.Name)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("inspect production source under %s: %v", root, err)
		}
	}
}
