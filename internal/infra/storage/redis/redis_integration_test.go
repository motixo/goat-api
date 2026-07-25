package redis

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
)

func TestNewClientIntegrationValidatesScriptsAndCloses(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis startup integration tests")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("parse GOAT_REDIS_ADDR: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, &config.Config{
		RedisHost:              host,
		RedisPort:              port,
		RedisConnectionTimeout: 2 * time.Second,
	}, &startupTestLogger{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if err := ValidateRuntimeScripts(ctx, client); err != nil {
		_ = client.Close()
		t.Fatalf("ValidateRuntimeScripts() after connection validation error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := client.Ping(ctx).Err(); err == nil {
		t.Fatal("Ping() after Close() error = nil, want closed-client failure")
	}
}
