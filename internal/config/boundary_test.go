package config

import (
	"go/build"
	"reflect"
	"strings"
	"testing"
)

func TestPackageDoesNotImportFrameworksOrDatabaseAdapters(t *testing.T) {
	t.Parallel()

	pkg, err := build.Default.ImportDir(".", build.IgnoreVendor)
	if err != nil {
		t.Fatalf("inspect config package imports: %v", err)
	}

	forbidden := map[string]struct{}{
		"github.com/gin-gonic/gin":     {},
		"github.com/jmoiron/sqlx":      {},
		"github.com/redis/go-redis/v9": {},
	}
	imports := append([]string{}, pkg.Imports...)
	imports = append(imports, pkg.TestImports...)
	imports = append(imports, pkg.XTestImports...)
	for _, importPath := range imports {
		if _, found := forbidden[importPath]; found {
			t.Errorf("internal/config imports framework package %q", importPath)
		}
		if strings.HasPrefix(importPath, "github.com/jackc/pgx/") {
			t.Errorf("internal/config imports PostgreSQL driver package %q", importPath)
		}
	}
}

func TestConfigDoesNotExposePostgreSQLDSNConstruction(t *testing.T) {
	t.Parallel()

	if _, found := reflect.TypeOf((*Config)(nil)).MethodByName("DSN"); found {
		t.Fatal("Config exposes PostgreSQL DSN construction")
	}
}
