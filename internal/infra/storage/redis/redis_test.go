package redis

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestClientConfigMapsRedisOptionsExactly(t *testing.T) {
	t.Parallel()

	cfg := ClientConfig{
		Host:              "redis.internal",
		Port:              6380,
		Password:          "redis-secret",
		Database:          2,
		ConnectionTimeout: 8 * time.Second,
	}

	options := newClientOptions(cfg)
	if options.Addr != "redis.internal:6380" {
		t.Fatalf("Addr = %q, want %q", options.Addr, "redis.internal:6380")
	}
	if options.Password != cfg.Password {
		t.Fatal("Password was not preserved in adapter-owned Redis options")
	}
	if options.DB != cfg.Database {
		t.Fatalf("DB = %d, want %d", options.DB, cfg.Database)
	}
	if !options.ContextTimeoutEnabled {
		t.Fatal("ContextTimeoutEnabled = false, want true")
	}
	if options.Username != "" ||
		options.Protocol != 0 ||
		options.PoolSize != 0 ||
		options.TLSConfig != nil ||
		options.DialTimeout != 0 ||
		options.ReadTimeout != 0 ||
		options.WriteTimeout != 0 {
		t.Fatalf("unconfigured Redis options changed from library defaults: %#v", options)
	}
}

func TestClientConfigFormattingRedactsPassword(t *testing.T) {
	t.Parallel()

	const password = "redis-client-config-secret"
	cfg := ClientConfig{
		Host:              "redis.internal",
		Port:              6380,
		Password:          password,
		Database:          2,
		ConnectionTimeout: time.Second,
	}

	formatted := fmt.Sprintf("%v %+v %#v", cfg, cfg, cfg)
	if strings.Contains(formatted, password) {
		t.Fatalf("formatted ClientConfig exposed Redis credentials: %s", formatted)
	}
	if !strings.Contains(formatted, "<redacted>") {
		t.Fatalf("formatted ClientConfig = %q, want explicit redaction", formatted)
	}
}

func TestRedisAdapterDoesNotLoadProcessEnvironmentOrApplicationConfig(t *testing.T) {
	t.Parallel()

	pkg, err := build.Default.ImportDir(".", build.IgnoreVendor)
	if err != nil {
		t.Fatalf("inspect Redis adapter imports: %v", err)
	}
	forbidden := map[string]struct{}{
		"github.com/motixo/goat-api/internal/config": {},
		"os": {},
	}
	for _, importPath := range pkg.Imports {
		if _, found := forbidden[importPath]; found {
			t.Errorf("Redis adapter imports forbidden package %q", importPath)
		}
	}
}

func TestInitializeRedisClientValidatesConnectionWithoutClosing(t *testing.T) {
	t.Parallel()

	const connectionTimeout = 5 * time.Second
	var validationDeadline time.Time
	client := &fakeStartupRedisClient{
		ping: func(ctx context.Context) error {
			var ok bool
			validationDeadline, ok = ctx.Deadline()
			if !ok {
				t.Fatal("Ping() context has no startup deadline")
			}
			return nil
		},
	}

	if err := initializeRedisClient(context.Background(), connectionTimeout, client); err != nil {
		t.Fatalf("initializeRedisClient() error = %v", err)
	}
	if client.closeCalls != 0 {
		t.Fatalf("Close() calls = %d, want 0 after successful validation", client.closeCalls)
	}
	remaining := time.Until(validationDeadline)
	if remaining <= 0 || remaining > connectionTimeout {
		t.Fatalf("Ping() deadline remaining = %s, want within (0, %s]", remaining, connectionTimeout)
	}
}

func TestInitializeRedisClientConnectionDeadlineExceeded(t *testing.T) {
	t.Parallel()

	client := &fakeStartupRedisClient{
		ping: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	err := initializeRedisClient(context.Background(), time.Millisecond, client)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("initializeRedisClient() error = %v, want deadline exceeded", err)
	}
	if client.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1 after timed-out validation", client.closeCalls)
	}
}

func TestInitializeRedisClientPreservesCallerCancellationCause(t *testing.T) {
	t.Parallel()

	cancellationCause := errors.New("startup canceled by process signal")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)
	client := &fakeStartupRedisClient{
		ping: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	err := initializeRedisClient(ctx, time.Minute, client)
	for name, target := range map[string]error{
		"context cancellation": context.Canceled,
		"caller cause":         cancellationCause,
	} {
		if !errors.Is(err, target) {
			t.Errorf("initializeRedisClient() error = %v, want %s", err, name)
		}
	}
	if client.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1 after canceled validation", client.closeCalls)
	}
}

func TestInitializeRedisClientPreservesEarlierCallerDeadline(t *testing.T) {
	t.Parallel()

	callerDeadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()
	var validationDeadline time.Time
	client := &fakeStartupRedisClient{
		ping: func(ctx context.Context) error {
			var ok bool
			validationDeadline, ok = ctx.Deadline()
			if !ok {
				t.Fatal("Ping() context has no deadline")
			}
			return nil
		},
	}

	if err := initializeRedisClient(ctx, time.Minute, client); err != nil {
		t.Fatalf("initializeRedisClient() error = %v", err)
	}
	if !validationDeadline.Equal(callerDeadline) {
		t.Fatalf(
			"Ping() deadline = %s, want caller deadline %s",
			validationDeadline,
			callerDeadline,
		)
	}
}

func TestInitializeRedisClientPreservesValidationAndCleanupErrors(t *testing.T) {
	t.Parallel()

	validationErr := errors.New("connection refused")
	closeErr := errors.New("close failed")
	client := &fakeStartupRedisClient{
		ping: func(context.Context) error {
			return validationErr
		},
		closeErr: closeErr,
	}

	err := initializeRedisClient(context.Background(), time.Second, client)
	for name, target := range map[string]error{
		"validation": validationErr,
		"cleanup":    closeErr,
	} {
		if !errors.Is(err, target) {
			t.Errorf("initializeRedisClient() error = %v, want wrapped %s error", err, name)
		}
	}
	if client.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1 after failed validation", client.closeCalls)
	}
}

func TestNewClientRejectsInvalidStartupInputs(t *testing.T) {
	t.Parallel()

	validConfig := ClientConfig{
		Host:              "127.0.0.1",
		Port:              6379,
		ConnectionTimeout: time.Second,
	}
	tests := []struct {
		name string
		ctx  context.Context
		cfg  ClientConfig
	}{
		{name: "missing context", cfg: validConfig},
		{
			name: "missing host",
			ctx:  context.Background(),
			cfg: ClientConfig{
				Port:              6379,
				ConnectionTimeout: time.Second,
			},
		},
		{
			name: "missing port",
			ctx:  context.Background(),
			cfg: ClientConfig{
				Host:              "127.0.0.1",
				ConnectionTimeout: time.Second,
			},
		},
		{
			name: "negative database",
			ctx:  context.Background(),
			cfg: ClientConfig{
				Host:              "127.0.0.1",
				Port:              6379,
				Database:          -1,
				ConnectionTimeout: time.Second,
			},
		},
		{
			name: "zero timeout",
			ctx:  context.Background(),
			cfg: ClientConfig{
				Host: "127.0.0.1",
				Port: 6379,
			},
		},
		{
			name: "negative timeout",
			ctx:  context.Background(),
			cfg: ClientConfig{
				Host:              "127.0.0.1",
				Port:              6379,
				ConnectionTimeout: -time.Second,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(test.ctx, test.cfg, &startupTestLogger{})
			if client != nil {
				_ = client.Close()
				t.Fatal("NewClient() returned a client for invalid startup input")
			}
			if err == nil {
				t.Fatal("NewClient() error = nil, want validation error")
			}
		})
	}
}

func TestNewClientConnectionRefusedDoesNotExposeCredentials(t *testing.T) {
	t.Parallel()

	const password = "redis-startup-test-secret"
	logger := &startupTestLogger{}
	client, err := NewClient(context.Background(), ClientConfig{
		Host:              "127.0.0.1",
		Port:              1,
		Password:          password,
		ConnectionTimeout: 100 * time.Millisecond,
	}, logger)
	if client != nil {
		_ = client.Close()
		t.Fatal("NewClient() returned a client after connection refusal")
	}
	if err == nil {
		t.Fatal("NewClient() error = nil, want connection failure")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("NewClient() error exposed Redis credentials: %v", err)
	}
	if logger.infoCalls != 0 {
		t.Fatalf("successful connection logs = %d, want 0", logger.infoCalls)
	}
	if logger.errorCalls != 1 {
		t.Fatalf("failed connection logs = %d, want 1", logger.errorCalls)
	}
}

type fakeStartupRedisClient struct {
	ping       func(context.Context) error
	closeErr   error
	closeCalls int
}

func (f *fakeStartupRedisClient) Ping(ctx context.Context) *goredis.StatusCmd {
	command := goredis.NewStatusCmd(ctx)
	if f.ping != nil {
		if err := f.ping(ctx); err != nil {
			command.SetErr(err)
			return command
		}
	}
	command.SetVal("PONG")
	return command
}

func (f *fakeStartupRedisClient) Close() error {
	f.closeCalls++
	return f.closeErr
}

type startupTestLogger struct {
	infoCalls  int
	errorCalls int
}

func (l *startupTestLogger) Info(string, ...any) {
	l.infoCalls++
}

func (l *startupTestLogger) Error(string, ...any) {
	l.errorCalls++
}

func (*startupTestLogger) Warn(string, ...any)  {}
func (*startupTestLogger) Debug(string, ...any) {}
func (*startupTestLogger) Panic(string, ...any) {}
