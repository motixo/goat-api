package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPGXStdlibDriverIsRegistered(t *testing.T) {
	for _, registered := range sql.Drivers() {
		if registered == driverName {
			return
		}
	}
	t.Fatalf("registered database drivers = %v, want %q", sql.Drivers(), driverName)
}

func TestNewDatabaseReturnsConnectionFailure(t *testing.T) {
	cfg := validClientConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 1
	cfg.Password = "connection-secret-that-must-not-appear"
	cfg.ConnectionTimeout = 100 * time.Millisecond
	cfg.InitializationTimeout = time.Second

	database, err := NewDatabase(context.Background(), cfg, postgresTestLogger{}, nil)
	if database != nil {
		t.Fatal("NewDatabase() database is non-nil after connection failure")
	}
	if err == nil {
		t.Fatal("NewDatabase() error = nil, want PostgreSQL connection failure")
	}
	for _, forbidden := range []string{cfg.Password, "postgres://", "password="} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("NewDatabase() error exposed PostgreSQL credentials or a full connection string: %v", err)
		}
	}
}

func TestNewDatabasePreservesCallerCancellationCause(t *testing.T) {
	cancellationCause := errors.New("startup canceled by process signal")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)

	cfg := validClientConfig()
	cfg.Host = "127.0.0.1"
	cfg.ConnectionTimeout = time.Second
	cfg.InitializationTimeout = time.Second

	database, err := NewDatabase(ctx, cfg, postgresTestLogger{}, nil)
	if database != nil {
		t.Fatal("NewDatabase() database is non-nil after caller cancellation")
	}
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("NewDatabase() error = %v, want caller cancellation cause", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NewDatabase() error = %v, want context.Canceled", err)
	}
}

func TestPingDatabaseAppliesConnectionDeadline(t *testing.T) {
	pinger := pingContextFunc(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("ping context has no deadline")
		}
		if remaining := time.Until(deadline); remaining > 100*time.Millisecond {
			return errors.New("ping context deadline is not the configured short timeout")
		}
		<-ctx.Done()
		return ctx.Err()
	})

	err := pingDatabase(context.Background(), 10*time.Millisecond, pinger)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pingDatabase() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestInitializeSchemaStopsOnContextDeadline(t *testing.T) {
	executions := 0
	executor := execContextFunc(func(ctx context.Context, _ string, _ ...any) (sql.Result, error) {
		executions++
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := initializeSchema(ctx, executor)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("initializeSchema() error = %v, want context.DeadlineExceeded", err)
	}
	if executions != 1 {
		t.Fatalf("schema executions = %d, want 1 before cancellation", executions)
	}
}

func TestStartupOperationErrorPreservesPrimaryAndCancellationCauses(t *testing.T) {
	primaryErr := errors.New("PostgreSQL operation failed")
	cancellationCause := errors.New("startup canceled by operator")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)

	err := startupOperationError("initialize schema", ctx, primaryErr)
	if !errors.Is(err, primaryErr) {
		t.Fatalf("startupOperationError() = %v, want primary error", err)
	}
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("startupOperationError() = %v, want cancellation cause", err)
	}
}

func TestCloseFailedDatabaseInitializationPreservesPrimaryAndCleanupErrors(t *testing.T) {
	primaryErr := errors.New("schema initialization failed")
	closeErr := errors.New("close failed")

	err := closeFailedDatabaseInitialization(closeFunc(func() error {
		return closeErr
	}), primaryErr)
	if !errors.Is(err, primaryErr) {
		t.Fatalf("closeFailedDatabaseInitialization() = %v, want primary error", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("closeFailedDatabaseInitialization() = %v, want close error", err)
	}
}

type pingContextFunc func(context.Context) error

func (f pingContextFunc) PingContext(ctx context.Context) error {
	return f(ctx)
}

type execContextFunc func(context.Context, string, ...any) (sql.Result, error)

func (f execContextFunc) ExecContext(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	return f(ctx, query, args...)
}

type closeFunc func() error

func (f closeFunc) Close() error {
	return f()
}

type postgresTestLogger struct{}

func (postgresTestLogger) Info(string, ...any)  {}
func (postgresTestLogger) Error(string, ...any) {}
func (postgresTestLogger) Warn(string, ...any)  {}
func (postgresTestLogger) Debug(string, ...any) {}
func (postgresTestLogger) Panic(string, ...any) {}
