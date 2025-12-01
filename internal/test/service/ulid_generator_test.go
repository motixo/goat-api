package service_test

import (
	"testing"

	"github.com/mot0x0/goth-api/internal/domain/service"
)

func TestULIDGenerator_New(t *testing.T) {
	gen := service.NewULIDGenerator()

	id1 := gen.New()
	id2 := gen.New()

	if id1 == "" || id2 == "" {
		t.Fatal("ULID should not be empty")
	}

	if len(id1) != 26 || len(id2) != 26 {
		t.Fatalf("expected ULID length 26, got %d and %d", len(id1), len(id2))
	}

	if id1 == id2 {
		t.Fatal("two ULIDs generated consecutively should not be equal")
	}
}
