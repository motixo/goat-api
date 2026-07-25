package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
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

type discardLogger struct{}

func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Panic(string, ...any) {}
