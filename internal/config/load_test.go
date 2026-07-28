package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadParsesCompleteTypedConfiguration(t *testing.T) {
	clearConfigEnvironment(t)
	setEnvironmentValues(t, map[string]string{
		"ENV":                           "development",
		"SERVER_PORT":                   "9090",
		"HTTP_READ_HEADER_TIMEOUT":      "6s",
		"HTTP_READ_TIMEOUT":             "20s",
		"HTTP_WRITE_TIMEOUT":            "40s",
		"HTTP_IDLE_TIMEOUT":             "90s",
		"HTTP_MAX_HEADER_BYTES":         "32768",
		"HTTP_MAX_BODY_BYTES":           "2097152",
		"HTTP_TRUSTED_PROXIES":          "127.0.0.1,10.0.0.0/8,::1",
		"DB_HOST":                       "postgres.internal",
		"DB_PORT":                       "5544",
		"DB_USER":                       "goat_user",
		"DB_PASSWORD":                   "database-secret",
		"DB_NAME":                       "goat_service",
		"DB_CONNECTION_TIMEOUT":         "7s",
		"DB_INITIALIZATION_TIMEOUT":     "3m",
		"JWT_SECRET":                    "jwt-secret-value",
		"PASSWORD_PEPPER":               "password-pepper-value",
		"PASSWORD_HASH_MAX_CONCURRENCY": "3",
		"REDIS_HOST":                    "redis.internal",
		"REDIS_PORT":                    "6380",
		"REDIS_PASSWORD":                "redis-secret",
		"REDIS_DB":                      "2",
		"REDIS_CONNECTION_TIMEOUT":      "8s",
		"JWT_EXPIRATION":                "10m",
		"REFRESH_TOKEN_EXPIRATION":      "48h",
		"SESSION_EXPIRATION":            "240h",
		"GIN_MODE":                      "release",
		"SEED":                          "false",
		"ADMIN_EMAIL":                   "operator@example.com",
		"ADMIN_PASSWORD":                "A-Strong-Password1!",
		"RATE_LIMIT_AUTH_LIMIT":         "7",
		"RATE_LIMIT_AUTH_WINDOW":        "2m",
		"RATE_LIMIT_PUBLIC_LIMIT":       "250",
		"RATE_LIMIT_PUBLIC_WINDOW":      "3m",
		"RATE_LIMIT_PRIVATE_LIMIT":      "75",
		"RATE_LIMIT_PRIVATE_WINDOW":     "4m",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := &Config{
		Env:                        "development",
		ServerPort:                 9090,
		HTTPReadHeaderTimeout:      6 * time.Second,
		HTTPReadTimeout:            20 * time.Second,
		HTTPWriteTimeout:           40 * time.Second,
		HTTPIdleTimeout:            90 * time.Second,
		HTTPMaxHeaderBytes:         32768,
		HTTPMaxBodyBytes:           2097152,
		HTTPTrustedProxies:         []string{"127.0.0.1", "10.0.0.0/8", "::1"},
		DBHost:                     "postgres.internal",
		DBPort:                     5544,
		DBUser:                     "goat_user",
		DBPassword:                 "database-secret",
		DBName:                     "goat_service",
		DBConnectionTimeout:        7 * time.Second,
		DBInitializationTimeout:    3 * time.Minute,
		JWTSecret:                  "jwt-secret-value",
		PasswordPepper:             "password-pepper-value",
		PasswordHashMaxConcurrency: 3,
		RedisHost:                  "redis.internal",
		RedisPort:                  6380,
		RedisPassword:              "redis-secret",
		RedisDB:                    2,
		RedisConnectionTimeout:     8 * time.Second,
		JWTExpiration:              10 * time.Minute,
		RefreshTokenExpiration:     48 * time.Hour,
		SessionExpiration:          240 * time.Hour,
		GinMode:                    "release",
		Seed:                       false,
		AdminEmail:                 "operator@example.com",
		AdminPassword:              "A-Strong-Password1!",
		RateLimitAuthLimit:         7,
		RateLimitAuthWindow:        2 * time.Minute,
		RateLimitPublicLimit:       250,
		RateLimitPublicWindow:      3 * time.Minute,
		RateLimitPrivateLimit:      75,
		RateLimitPrivateWindow:     4 * time.Minute,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("Load() config = %#v, want %#v", cfg, want)
	}
	if got := cfg.ServerAddress(); got != ":9090" {
		t.Fatalf("ServerAddress() = %q, want %q", got, ":9090")
	}
}

func TestLoadAppliesIntentionalDefaults(t *testing.T) {
	clearConfigEnvironment(t)
	setEnvironmentValues(t, requiredEnvironment())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Env != "development" ||
		cfg.ServerPort != 8080 ||
		cfg.DBPort != 5432 ||
		cfg.RedisHost != "localhost" ||
		cfg.RedisPort != 6379 ||
		cfg.RedisDB != 0 ||
		cfg.RedisPassword != "" ||
		cfg.PasswordHashMaxConcurrency != 2 ||
		cfg.HTTPMaxHeaderBytes != 64<<10 ||
		cfg.HTTPMaxBodyBytes != 1<<20 ||
		len(cfg.HTTPTrustedProxies) != 0 ||
		cfg.GinMode != "debug" ||
		!cfg.Seed ||
		cfg.AdminEmail != defaultAdminEmail ||
		cfg.AdminPassword != defaultAdminPassword {
		t.Fatalf("Load() defaults were not preserved: %#v", cfg)
	}
	for name, values := range map[string]struct {
		got  time.Duration
		want time.Duration
	}{
		"DB_CONNECTION_TIMEOUT":     {got: cfg.DBConnectionTimeout, want: 5 * time.Second},
		"DB_INITIALIZATION_TIMEOUT": {got: cfg.DBInitializationTimeout, want: 2 * time.Minute},
		"HTTP_READ_HEADER_TIMEOUT":  {got: cfg.HTTPReadHeaderTimeout, want: 5 * time.Second},
		"HTTP_READ_TIMEOUT":         {got: cfg.HTTPReadTimeout, want: 15 * time.Second},
		"HTTP_WRITE_TIMEOUT":        {got: cfg.HTTPWriteTimeout, want: 30 * time.Second},
		"HTTP_IDLE_TIMEOUT":         {got: cfg.HTTPIdleTimeout, want: time.Minute},
		"REDIS_CONNECTION_TIMEOUT":  {got: cfg.RedisConnectionTimeout, want: 5 * time.Second},
		"JWT_EXPIRATION":            {got: cfg.JWTExpiration, want: 5 * time.Minute},
		"REFRESH_TOKEN_EXPIRATION":  {got: cfg.RefreshTokenExpiration, want: 168 * time.Hour},
		"SESSION_EXPIRATION":        {got: cfg.SessionExpiration, want: 720 * time.Hour},
		"RATE_LIMIT_AUTH_WINDOW":    {got: cfg.RateLimitAuthWindow, want: time.Minute},
		"RATE_LIMIT_PUBLIC_WINDOW":  {got: cfg.RateLimitPublicWindow, want: time.Minute},
		"RATE_LIMIT_PRIVATE_WINDOW": {got: cfg.RateLimitPrivateWindow, want: time.Minute},
	} {
		if values.got != values.want {
			t.Errorf("default %s = %s, want %s", name, values.got, values.want)
		}
	}
	if cfg.RateLimitAuthLimit != 5 ||
		cfg.RateLimitPublicLimit != 100 ||
		cfg.RateLimitPrivateLimit != 60 {
		t.Fatalf("rate-limit defaults were not preserved: %#v", cfg)
	}
}

func TestLoadRejectsMissingRequiredSettings(t *testing.T) {
	for _, name := range []string{
		"DB_HOST",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"JWT_SECRET",
		"PASSWORD_PEPPER",
	} {
		t.Run(name, func(t *testing.T) {
			clearConfigEnvironment(t)
			values := requiredEnvironment()
			delete(values, name)
			setEnvironmentValues(t, values)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want missing %s error", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("Load() error = %v, want %s context", err, name)
			}
		})
	}
}

func TestLoadRejectsMalformedTypedValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "SERVER_PORT", value: "not-a-port"},
		{name: "REDIS_DB", value: "not-an-integer"},
		{name: "SEED", value: "not-a-boolean"},
		{name: "PASSWORD_HASH_MAX_CONCURRENCY", value: "not-an-integer"},
		{name: "HTTP_READ_HEADER_TIMEOUT", value: "not-a-duration"},
		{name: "HTTP_MAX_BODY_BYTES", value: "not-an-integer"},
		{name: "JWT_EXPIRATION", value: "not-a-duration"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv(test.name, test.value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want malformed %s error", test.name)
			}
		})
	}
}

func TestLoadRejectsInvalidRanges(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		wantFragment string
	}{
		{name: "SERVER_PORT", value: "0", wantFragment: "SERVER_PORT"},
		{name: "HTTP_READ_HEADER_TIMEOUT", value: "0s", wantFragment: "HTTP_READ_HEADER_TIMEOUT"},
		{name: "HTTP_READ_HEADER_TIMEOUT", value: "31s", wantFragment: "HTTP_READ_HEADER_TIMEOUT"},
		{name: "HTTP_READ_TIMEOUT", value: "121s", wantFragment: "HTTP_READ_TIMEOUT"},
		{name: "HTTP_WRITE_TIMEOUT", value: "301s", wantFragment: "HTTP_WRITE_TIMEOUT"},
		{name: "HTTP_IDLE_TIMEOUT", value: "601s", wantFragment: "HTTP_IDLE_TIMEOUT"},
		{name: "HTTP_MAX_HEADER_BYTES", value: "0", wantFragment: "HTTP_MAX_HEADER_BYTES"},
		{name: "HTTP_MAX_HEADER_BYTES", value: "1048577", wantFragment: "HTTP_MAX_HEADER_BYTES"},
		{name: "HTTP_MAX_BODY_BYTES", value: "-1", wantFragment: "HTTP_MAX_BODY_BYTES"},
		{name: "HTTP_MAX_BODY_BYTES", value: "10485761", wantFragment: "HTTP_MAX_BODY_BYTES"},
		{name: "DB_PORT", value: "0", wantFragment: "DB_PORT"},
		{name: "REDIS_PORT", value: "0", wantFragment: "REDIS_PORT"},
		{name: "REDIS_DB", value: "-1", wantFragment: "REDIS_DB"},
		{name: "DB_CONNECTION_TIMEOUT", value: "0s", wantFragment: "DB_CONNECTION_TIMEOUT"},
		{name: "DB_CONNECTION_TIMEOUT", value: "-1s", wantFragment: "DB_CONNECTION_TIMEOUT"},
		{name: "DB_CONNECTION_TIMEOUT", value: "61s", wantFragment: "DB_CONNECTION_TIMEOUT"},
		{name: "DB_INITIALIZATION_TIMEOUT", value: "31m", wantFragment: "DB_INITIALIZATION_TIMEOUT"},
		{name: "REDIS_CONNECTION_TIMEOUT", value: "61s", wantFragment: "REDIS_CONNECTION_TIMEOUT"},
		{name: "REFRESH_TOKEN_EXPIRATION", value: "0s", wantFragment: "REFRESH_TOKEN_EXPIRATION"},
		{name: "SESSION_EXPIRATION", value: "-1s", wantFragment: "SESSION_EXPIRATION"},
		{name: "PASSWORD_HASH_MAX_CONCURRENCY", value: "0", wantFragment: "PASSWORD_HASH_MAX_CONCURRENCY"},
		{name: "PASSWORD_HASH_MAX_CONCURRENCY", value: "-1", wantFragment: "PASSWORD_HASH_MAX_CONCURRENCY"},
		{name: "PASSWORD_HASH_MAX_CONCURRENCY", value: "5", wantFragment: "PASSWORD_HASH_MAX_CONCURRENCY"},
		{name: "RATE_LIMIT_AUTH_LIMIT", value: "0", wantFragment: "RATE_LIMIT_AUTH_LIMIT"},
		{name: "RATE_LIMIT_PUBLIC_LIMIT", value: "-1", wantFragment: "RATE_LIMIT_PUBLIC_LIMIT"},
		{name: "RATE_LIMIT_PRIVATE_WINDOW", value: "0s", wantFragment: "RATE_LIMIT_PRIVATE_WINDOW"},
	}
	for _, test := range tests {
		t.Run(test.name+"="+test.value, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv(test.name, test.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want invalid %s error", test.name)
			}
			if !strings.Contains(err.Error(), test.wantFragment) {
				t.Fatalf("Load() error = %v, want %s context", err, test.wantFragment)
			}
		})
	}
}

func TestLoadValidatesTrustedHTTPProxies(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      []string
		wantError bool
	}{
		{name: "omitted trusts no proxy"},
		{name: "explicit empty trusts no proxy", value: ""},
		{
			name:  "IP and CIDR entries",
			value: " 127.0.0.1 , 10.0.0.0/8 , ::1 , 2001:db8::/32 ",
			want:  []string{"127.0.0.1", "10.0.0.0/8", "::1", "2001:db8::/32"},
		},
		{name: "hostname rejected", value: "proxy.internal", wantError: true},
		{name: "empty entry rejected", value: "127.0.0.1,,10.0.0.1", wantError: true},
		{name: "invalid CIDR rejected", value: "10.0.0.0/99", wantError: true},
		{name: "universal IPv4 range rejected", value: "0.0.0.0/0", wantError: true},
		{name: "universal IPv6 range rejected", value: "::/0", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setValidLoadEnvironment(t)
			if test.name != "omitted trusts no proxy" {
				t.Setenv("HTTP_TRUSTED_PROXIES", test.value)
			}

			cfg, err := Load()
			if (err != nil) != test.wantError {
				t.Fatalf("Load() error = %v, wantError %t", err, test.wantError)
			}
			if err == nil && !reflect.DeepEqual(cfg.HTTPTrustedProxies, test.want) {
				t.Fatalf("HTTPTrustedProxies = %#v, want %#v", cfg.HTTPTrustedProxies, test.want)
			}
		})
	}
}

func TestLoadRejectsInvalidEnvironmentAndTokenBounds(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "ENV", value: "staging"},
		{name: "GIN_MODE", value: "test"},
		{name: "JWT_EXPIRATION", value: "59s"},
		{name: "JWT_EXPIRATION", value: "16m"},
	}
	for _, test := range tests {
		t.Run(test.name+"="+test.value, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv(test.name, test.value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want invalid %s error", test.name)
			}
		})
	}
}

func TestLoadRejectsEmptyRequiredValues(t *testing.T) {
	for name, value := range map[string]string{
		"HTTP_READ_TIMEOUT": "",
		"REDIS_HOST":        "",
		"JWT_SECRET":        " ",
		"PASSWORD_PEPPER":   "\t",
	} {
		t.Run(name, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv(name, value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want empty %s error", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("Load() error = %v, want %s context", err, name)
			}
		})
	}
}

func TestLoadPreservesNumericBooleanSeedValues(t *testing.T) {
	for value, want := range map[string]bool{"0": false, "1": true} {
		t.Run(value, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv("SEED", value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Seed != want {
				t.Fatalf("Seed = %t, want %t", cfg.Seed, want)
			}
		})
	}
}

func TestLoadValidatesProductionAdministratorSeeding(t *testing.T) {
	tests := []struct {
		name          string
		environment   map[string]string
		wantErr       bool
		wantSeedValue bool
	}{
		{
			name: "seeding disabled",
			environment: map[string]string{
				"SEED": "false",
			},
		},
		{
			name:    "default credentials rejected",
			wantErr: true,
			environment: map[string]string{
				"SEED": "true",
			},
		},
		{
			name:    "default email rejected",
			wantErr: true,
			environment: map[string]string{
				"SEED":           "true",
				"ADMIN_PASSWORD": "Another-Strong1!",
			},
		},
		{
			name:    "default password rejected",
			wantErr: true,
			environment: map[string]string{
				"SEED":        "true",
				"ADMIN_EMAIL": "security@example.com",
			},
		},
		{
			name:    "weak password rejected",
			wantErr: true,
			environment: map[string]string{
				"SEED":           "true",
				"ADMIN_EMAIL":    "security@example.com",
				"ADMIN_PASSWORD": "weak",
			},
		},
		{
			name:          "explicit credentials accepted",
			wantSeedValue: true,
			environment: map[string]string{
				"SEED":           "true",
				"ADMIN_EMAIL":    "security@example.com",
				"ADMIN_PASSWORD": "Another-Strong1!",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setValidLoadEnvironment(t)
			t.Setenv("ENV", "production")
			for name, value := range test.environment {
				t.Setenv(name, value)
			}

			cfg, err := Load()
			if (err != nil) != test.wantErr {
				t.Fatalf("Load() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && cfg.Seed != test.wantSeedValue {
				t.Fatalf("Seed = %t, want %t", cfg.Seed, test.wantSeedValue)
			}
		})
	}
}

func TestLoadErrorsDoNotExposeSensitiveValues(t *testing.T) {
	setValidLoadEnvironment(t)
	secrets := []string{
		"database-sensitive-value",
		"jwt-sensitive-value",
		"pepper-sensitive-value",
		"redis-sensitive-value",
		"administrator-sensitive-value",
	}
	for name, value := range map[string]string{
		"DB_PASSWORD":     secrets[0],
		"JWT_SECRET":      secrets[1],
		"PASSWORD_PEPPER": secrets[2],
		"REDIS_PASSWORD":  secrets[3],
		"ADMIN_PASSWORD":  secrets[4],
		"REDIS_DB":        "not-an-integer",
	} {
		t.Setenv(name, value)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want malformed REDIS_DB error")
	}
	for _, secret := range secrets {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Load() error exposed a sensitive value: %v", err)
		}
	}
}

func TestLoadDoesNotReadDotEnvFiles(t *testing.T) {
	clearConfigEnvironment(t)
	directory := t.TempDir()
	dotEnv := strings.Join([]string{
		"DB_HOST=dotenv-postgres",
		"DB_USER=dotenv-user",
		"DB_PASSWORD=dotenv-password",
		"DB_NAME=dotenv-database",
		"JWT_SECRET=dotenv-jwt-secret",
		"PASSWORD_PEPPER=dotenv-pepper",
	}, "\n")
	if err := os.WriteFile(directory+"/.env", []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env fixture: %v", err)
	}
	t.Chdir(directory)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing process-environment settings")
	}
	if !strings.Contains(err.Error(), "DB_HOST") {
		t.Fatalf("Load() error = %v, want missing DB_HOST context", err)
	}
	if strings.Contains(err.Error(), "dotenv-password") ||
		strings.Contains(err.Error(), "dotenv-jwt-secret") {
		t.Fatalf("Load() error exposed .env contents: %v", err)
	}
}

func TestConfigUsesExplicitEnvironmentTags(t *testing.T) {
	configType := reflect.TypeOf(Config{})
	for index := range configType.NumField() {
		field := configType.Field(index)
		if field.Tag.Get("env") == "" {
			t.Errorf("Config.%s has no env tag", field.Name)
		}
	}
}

func requiredEnvironment() map[string]string {
	return map[string]string{
		"DB_HOST":         "127.0.0.1",
		"DB_USER":         "postgres",
		"DB_PASSWORD":     "postgres",
		"DB_NAME":         "goat",
		"JWT_SECRET":      "config-test-secret",
		"PASSWORD_PEPPER": "config-test-pepper",
	}
}

func setEnvironmentValues(t *testing.T, values map[string]string) {
	t.Helper()
	for name, value := range values {
		t.Setenv(name, value)
	}
}
