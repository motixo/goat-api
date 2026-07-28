package service

import (
	"go/build"
	"testing"
)

func TestPasswordHashingAdapterDoesNotImportApplicationConfiguration(t *testing.T) {
	t.Parallel()

	pkg, err := build.Default.ImportDir(".", build.IgnoreVendor)
	if err != nil {
		t.Fatalf("inspect password-hashing adapter imports: %v", err)
	}
	for _, importPath := range pkg.Imports {
		if importPath == "github.com/motixo/goat-api/internal/config" {
			t.Fatal("password-hashing adapter imports complete application configuration")
		}
	}
}
