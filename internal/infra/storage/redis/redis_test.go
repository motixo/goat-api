package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

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

	validConfig := &config.Config{
		RedisHost:              "127.0.0.1",
		RedisPort:              "6379",
		RedisConnectionTimeout: time.Second,
	}
	tests := []struct {
		name string
		ctx  context.Context
		cfg  *config.Config
	}{
		{name: "missing context", cfg: validConfig},
		{name: "missing configuration", ctx: context.Background()},
		{
			name: "zero timeout",
			ctx:  context.Background(),
			cfg: &config.Config{
				RedisHost: "127.0.0.1",
				RedisPort: "6379",
			},
		},
		{
			name: "negative timeout",
			ctx:  context.Background(),
			cfg: &config.Config{
				RedisHost:              "127.0.0.1",
				RedisPort:              "6379",
				RedisConnectionTimeout: -time.Second,
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
	client, err := NewClient(context.Background(), &config.Config{
		RedisHost:              "127.0.0.1",
		RedisPort:              "1",
		RedisPassword:          password,
		RedisConnectionTimeout: 100 * time.Millisecond,
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
