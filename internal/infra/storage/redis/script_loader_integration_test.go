package redis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestRuntimeScriptValidationLoadsValidScriptsAndRejectsInvalidLua(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis script validation integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := goredis.NewClient(&goredis.Options{Addr: address})
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	if err := ValidateRuntimeScripts(ctx, client); err != nil {
		t.Fatalf("ValidateRuntimeScripts() error = %v", err)
	}

	const invalidName ScriptName = "invalid_test_script"
	registry, err := newScriptRegistry(
		[]scriptAsset{{name: invalidName, source: "this is not valid Lua"}},
		[]ScriptName{invalidName},
	)
	if err != nil {
		t.Fatalf("newScriptRegistry() error = %v", err)
	}
	err = validateScriptRegistry(ctx, client, registry, []ScriptName{invalidName})
	if !errors.Is(err, errRuntimeScriptValidation) {
		t.Fatalf("validateScriptRegistry() error = %v, want runtime-validation identity", err)
	}
}
