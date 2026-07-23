package permcache

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPermissionCacheJSONEncodingRemainsCompatible(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	want := []*entity.Permission{{
		ID:        "33333333-3333-4333-8333-333333333333",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: createdAt,
	}}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal cached permissions: %v", err)
	}
	const wantJSON = `[{"ID":"33333333-3333-4333-8333-333333333333","Role":3,"Action":"full_access","CreatedAt":"2026-07-23T16:00:00Z"}]`
	if string(encoded) != wantJSON {
		t.Fatalf("cached JSON = %s, want %s", encoded, wantJSON)
	}
}
