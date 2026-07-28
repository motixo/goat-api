package redis

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
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
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		t.Fatalf("parse GOAT_REDIS_ADDR port: %q is not a valid port", port)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := NewClient(ctx, ClientConfig{
		Host:              host,
		Port:              uint16(parsedPort),
		ConnectionTimeout: 2 * time.Second,
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
