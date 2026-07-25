package postgres

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPGXTypesRemainInsidePostgreSQLAdapter(t *testing.T) {
	t.Parallel()

	for _, root := range []string{
		"../../../../cmd",
		"../../../domain",
		"../../../usecase",
		"../../../delivery",
	} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() ||
				!strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}

			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range parsed.Imports {
				importPath, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if strings.HasPrefix(importPath, "github.com/jackc/pgx/") {
					t.Errorf("%s leaks PostgreSQL driver type through import %s", path, importPath)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("inspect production source under %s: %v", root, err)
		}
	}
}
