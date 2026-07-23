package config

import "testing"

func TestRedisOptionsRespectContextTimeouts(t *testing.T) {
	cfg := &Config{
		RedisHost: "redis.internal",
		RedisPort: "6380",
	}

	options := cfg.RedisOptions()

	if !options.ContextTimeoutEnabled {
		t.Fatal("RedisOptions().ContextTimeoutEnabled = false, want true")
	}
}
