package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

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
			cfg := validConfig()
			cfg.JWTExpiration = test.lifetime
			err := cfg.validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateBoundsDatabaseStartupTimeouts(t *testing.T) {
	tests := []struct {
		name                  string
		connectionTimeout     time.Duration
		initializationTimeout time.Duration
		wantErr               bool
	}{
		{
			name:                  "valid defaults",
			connectionTimeout:     5 * time.Second,
			initializationTimeout: 2 * time.Minute,
		},
		{
			name:                  "connection timeout must be positive",
			connectionTimeout:     0,
			initializationTimeout: 2 * time.Minute,
			wantErr:               true,
		},
		{
			name:                  "connection timeout rejects unreasonable value",
			connectionTimeout:     maximumDBConnectionTimeout + time.Nanosecond,
			initializationTimeout: 2 * time.Minute,
			wantErr:               true,
		},
		{
			name:                  "initialization timeout must be positive",
			connectionTimeout:     5 * time.Second,
			initializationTimeout: -time.Second,
			wantErr:               true,
		},
		{
			name:                  "initialization timeout rejects unreasonable value",
			connectionTimeout:     5 * time.Second,
			initializationTimeout: maximumDBInitializationTimeout + time.Nanosecond,
			wantErr:               true,
		},
		{
			name:                  "maximum values remain valid",
			connectionTimeout:     maximumDBConnectionTimeout,
			initializationTimeout: maximumDBInitializationTimeout,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.DBConnectionTimeout = test.connectionTimeout
			cfg.DBInitializationTimeout = test.initializationTimeout
			err := cfg.validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateBoundsRedisConnectionTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{name: "zero", timeout: 0, wantErr: true},
		{name: "negative", timeout: -time.Second, wantErr: true},
		{name: "default", timeout: 5 * time.Second},
		{name: "maximum", timeout: maximumRedisConnectionTimeout},
		{
			name:    "above maximum",
			timeout: maximumRedisConnectionTimeout + time.Nanosecond,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.RedisConnectionTimeout = test.timeout
			err := cfg.validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestValidateBoundsPasswordHashConcurrency(t *testing.T) {
	tests := []struct {
		name           string
		maxConcurrency int
		wantErr        bool
	}{
		{name: "negative", maxConcurrency: -1, wantErr: true},
		{name: "zero", maxConcurrency: 0, wantErr: true},
		{name: "minimum", maxConcurrency: 1},
		{name: "default", maxConcurrency: 2},
		{name: "maximum", maxConcurrency: maximumPasswordHashConcurrency},
		{name: "above maximum", maxConcurrency: maximumPasswordHashConcurrency + 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PasswordHashMaxConcurrency = test.maxConcurrency
			err := cfg.validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestLoadAppliesDatabaseStartupTimeoutDefaults(t *testing.T) {
	setValidLoadEnvironment(t)
	unsetEnvironment(t, "DB_CONNECTION_TIMEOUT")
	unsetEnvironment(t, "DB_INITIALIZATION_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DBConnectionTimeout != 5*time.Second {
		t.Fatalf("DBConnectionTimeout = %s, want 5s", cfg.DBConnectionTimeout)
	}
	if cfg.DBInitializationTimeout != 2*time.Minute {
		t.Fatalf("DBInitializationTimeout = %s, want 2m", cfg.DBInitializationTimeout)
	}
}

func TestLoadAppliesRedisConnectionTimeoutDefault(t *testing.T) {
	setValidLoadEnvironment(t)
	unsetEnvironment(t, "REDIS_CONNECTION_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RedisConnectionTimeout != 5*time.Second {
		t.Fatalf("RedisConnectionTimeout = %s, want 5s", cfg.RedisConnectionTimeout)
	}
}

func TestLoadRejectsMalformedDatabaseStartupTimeout(t *testing.T) {
	setValidLoadEnvironment(t)
	t.Setenv("DB_CONNECTION_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want malformed DB_CONNECTION_TIMEOUT error")
	}
	if !strings.Contains(err.Error(), "DBConnectionTimeout") {
		t.Fatalf("Load() error = %v, want DBConnectionTimeout context", err)
	}
}

func TestLoadRejectsMalformedRedisConnectionTimeout(t *testing.T) {
	setValidLoadEnvironment(t)
	t.Setenv("REDIS_CONNECTION_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want malformed REDIS_CONNECTION_TIMEOUT error")
	}
	if !strings.Contains(err.Error(), "RedisConnectionTimeout") {
		t.Fatalf("Load() error = %v, want RedisConnectionTimeout context", err)
	}
}

func setValidLoadEnvironment(t *testing.T) {
	t.Helper()
	clearConfigEnvironment(t)
	for name, value := range map[string]string{
		"ENV":                       "development",
		"DB_HOST":                   "127.0.0.1",
		"DB_USER":                   "postgres",
		"DB_PASSWORD":               "postgres",
		"DB_NAME":                   "goat",
		"JWT_SECRET":                "config-test-secret",
		"JWT_EXPIRATION":            "5m",
		"PASSWORD_PEPPER":           "config-test-pepper",
		"GIN_MODE":                  "debug",
		"DB_CONNECTION_TIMEOUT":     "5s",
		"DB_INITIALIZATION_TIMEOUT": "2m",
		"REDIS_CONNECTION_TIMEOUT":  "5s",
	} {
		t.Setenv(name, value)
	}
}

func validConfig() *Config {
	return &Config{
		Env:                        "development",
		ServerPort:                 8080,
		DBHost:                     "127.0.0.1",
		DBPort:                     5432,
		DBUser:                     "postgres",
		DBPassword:                 "postgres",
		DBName:                     "goat",
		DBConnectionTimeout:        5 * time.Second,
		DBInitializationTimeout:    2 * time.Minute,
		JWTSecret:                  "config-test-secret",
		PasswordPepper:             "config-test-pepper",
		PasswordHashMaxConcurrency: 2,
		RedisHost:                  "127.0.0.1",
		RedisPort:                  6379,
		RedisConnectionTimeout:     5 * time.Second,
		JWTExpiration:              5 * time.Minute,
		RefreshTokenExpiration:     7 * 24 * time.Hour,
		SessionExpiration:          30 * 24 * time.Hour,
		GinMode:                    "debug",
		Seed:                       false,
		RateLimitAuthLimit:         5,
		RateLimitAuthWindow:        time.Minute,
		RateLimitPublicLimit:       100,
		RateLimitPublicWindow:      time.Minute,
		RateLimitPrivateLimit:      60,
		RateLimitPrivateWindow:     time.Minute,
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range configEnvironmentNames() {
		unsetEnvironment(t, name)
	}
}

func configEnvironmentNames() []string {
	configType := reflect.TypeOf(Config{})
	names := make([]string, 0, configType.NumField())
	for index := range configType.NumField() {
		name, _, _ := strings.Cut(configType.Field(index).Tag.Get("env"), ",")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func unsetEnvironment(t *testing.T, name string) {
	t.Helper()
	original, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(name, original)
		} else {
			err = os.Unsetenv(name)
		}
		if err != nil {
			t.Errorf("restore %s: %v", name, err)
		}
	})
}
