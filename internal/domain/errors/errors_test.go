package errors

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDomainAndApplicationPackagesDoNotImportHTTP(t *testing.T) {
	for _, root := range []string{"..", "../../usecase"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
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
				if importPath == "net/http" {
					t.Errorf("%s imports net/http; protocol mapping belongs to HTTP delivery", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("inspect %s: %v", root, err)
		}
	}
}
