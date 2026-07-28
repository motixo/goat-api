package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
	deliveryHTTP "github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
)

func TestNewRateLimitConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		RateLimitAuthLimit:     5,
		RateLimitAuthWindow:    time.Minute,
		RateLimitPublicLimit:   100,
		RateLimitPublicWindow:  2 * time.Minute,
		RateLimitPrivateLimit:  60,
		RateLimitPrivateWindow: 3 * time.Minute,
	}

	want := middleware.RateLimitConfig{
		Auth: middleware.RateLimit{
			Limit:  5,
			Window: time.Minute,
		},
		Public: middleware.RateLimit{
			Limit:  100,
			Window: 2 * time.Minute,
		},
		Private: middleware.RateLimit{
			Limit:  60,
			Window: 3 * time.Minute,
		},
	}

	if got := newRateLimitConfig(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("newRateLimitConfig() = %#v, want %#v", got, want)
	}
}

func TestNewHTTPServerConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		GinMode:               "release",
		HTTPReadHeaderTimeout: 6 * time.Second,
		HTTPReadTimeout:       20 * time.Second,
		HTTPWriteTimeout:      40 * time.Second,
		HTTPIdleTimeout:       90 * time.Second,
		HTTPMaxHeaderBytes:    32 << 10,
		HTTPMaxBodyBytes:      2 << 20,
		HTTPTrustedProxies:    []string{"127.0.0.1", "10.0.0.0/8"},
	}
	want := deliveryHTTP.ServerConfig{
		GinMode:             deliveryHTTP.GinModeRelease,
		ReadHeaderTimeout:   cfg.HTTPReadHeaderTimeout,
		ReadTimeout:         cfg.HTTPReadTimeout,
		WriteTimeout:        cfg.HTTPWriteTimeout,
		IdleTimeout:         cfg.HTTPIdleTimeout,
		MaxHeaderBytes:      cfg.HTTPMaxHeaderBytes,
		MaxRequestBodyBytes: cfg.HTTPMaxBodyBytes,
		TrustedProxies:      []string{"127.0.0.1", "10.0.0.0/8"},
	}

	got := newHTTPServerConfig(cfg)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("newHTTPServerConfig() = %#v, want %#v", got, want)
	}
	cfg.HTTPTrustedProxies[0] = "203.0.113.1"
	if got.TrustedProxies[0] != "127.0.0.1" {
		t.Fatal("newHTTPServerConfig() retained the mutable application-config slice")
	}
}

func TestNewRedisClientConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		RedisHost:              "redis.internal",
		RedisPort:              6380,
		RedisPassword:          "redis-secret",
		RedisDB:                2,
		RedisConnectionTimeout: 8 * time.Second,
	}
	want := redisStorage.ClientConfig{
		Host:              cfg.RedisHost,
		Port:              cfg.RedisPort,
		Password:          cfg.RedisPassword,
		Database:          cfg.RedisDB,
		ConnectionTimeout: cfg.RedisConnectionTimeout,
	}

	if got := newRedisClientConfig(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("newRedisClientConfig() = %#v, want %#v", got, want)
	}
}

func TestNewPasswordHasherConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		PasswordPepper:             "composition-pepper",
		PasswordHashMaxConcurrency: 3,
	}
	want := authInfra.PasswordHasherConfig{
		Pepper:         cfg.PasswordPepper,
		MaxConcurrency: cfg.PasswordHashMaxConcurrency,
	}

	if got := newPasswordHasherConfig(cfg); !reflect.DeepEqual(got, want) {
		t.Fatal("newPasswordHasherConfig() did not map only the password-hashing settings")
	}
}

func TestNewPostgresClientConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		DBHost:                  "postgres.internal",
		DBPort:                  5544,
		DBUser:                  "goat_user",
		DBPassword:              "database-secret",
		DBName:                  "goat_service",
		DBConnectionTimeout:     7 * time.Second,
		DBInitializationTimeout: 3 * time.Minute,
		Seed:                    true,
		AdminEmail:              "administrator@goat.api",
		AdminPassword:           "administrator-secret",
	}
	want := postgres.ClientConfig{
		Host:                  cfg.DBHost,
		Port:                  cfg.DBPort,
		User:                  cfg.DBUser,
		Password:              cfg.DBPassword,
		Database:              cfg.DBName,
		SSLMode:               postgres.SSLModeDisable,
		ConnectionTimeout:     cfg.DBConnectionTimeout,
		InitializationTimeout: cfg.DBInitializationTimeout,
		Seed:                  cfg.Seed,
		AdminEmail:            cfg.AdminEmail,
		AdminPassword:         cfg.AdminPassword,
	}

	if got := newPostgresClientConfig(cfg); !reflect.DeepEqual(got, want) {
		t.Fatal("newPostgresClientConfig() did not map PostgreSQL startup settings exactly")
	}
}

type discardLogger struct{}

func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Panic(string, ...any) {}
