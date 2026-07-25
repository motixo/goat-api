package config

import (
	"testing"
	"time"
)

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

func TestValidateBoundsAccessTokenLifetime(t *testing.T) {
	tests := []struct {
		name     string
		lifetime time.Duration
		wantErr  bool
	}{
		{name: "below minimum", lifetime: time.Minute - time.Nanosecond, wantErr: true},
		{name: "minimum", lifetime: time.Minute},
		{name: "default", lifetime: 5 * time.Minute},
		{name: "maximum", lifetime: 15 * time.Minute},
		{name: "above maximum", lifetime: 15*time.Minute + time.Nanosecond, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{Env: "development", JWTExpiration: test.lifetime}
			err := cfg.validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
