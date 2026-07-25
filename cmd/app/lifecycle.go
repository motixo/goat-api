package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrApplicationAlreadyRun = errors.New("application already run")
	ErrApplicationClosed     = errors.New("application is closed")
	ErrHTTPServerStopped     = errors.New("HTTP server stopped unexpectedly")
)

type lifecycleServer interface {
	Start(addr string) (<-chan error, error)
	Shutdown(ctx context.Context) error
	Close() error
}

type lifecycleWorker interface {
	Start(ctx context.Context) error
	Shutdown()
}

type applicationResources struct {
	address         string
	shutdownTimeout time.Duration
	server          lifecycleServer
	cleaner         lifecycleWorker
	closeRedis      func() error
	closePostgres   func() error
	syncLogger      func() error
}

// Application is the composition root's lifecycle boundary. It is the sole
// owner of successfully constructed process resources.
type Application struct {
	applicationResources

	lifecycleMu           sync.Mutex
	runStarted            bool
	closed                bool
	cleanerStartAttempted bool
	serverStarted         bool
	cancelWorkers         context.CancelFunc

	cleanupOnce sync.Once
	cleanupErr  error
}

func newApplication(resources applicationResources) *Application {
	if resources.shutdownTimeout <= 0 {
		resources.shutdownTimeout = defaultShutdownTimeout
	}
	return &Application{
		applicationResources: resources,
	}
}

// Run starts background work and HTTP serving, then blocks until cancellation
// or an HTTP serve failure. Every return path performs complete cleanup.
func (a *Application) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.Join(
			errors.New("application context is required"),
			a.shutdownWithTimeout(),
		)
	}
	if ctx.Err() != nil {
		return a.shutdownWithTimeout()
	}

	serveErrors, err := a.start(ctx)
	if err != nil {
		return errors.Join(err, a.shutdownWithTimeout())
	}

	select {
	case <-ctx.Done():
		return a.shutdownWithTimeout()
	case serveErr, ok := <-serveErrors:
		if !ok {
			serveErr = nil
		}
		cleanupErr := a.shutdownWithTimeout()
		if serveErr == nil {
			return errors.Join(ErrHTTPServerStopped, cleanupErr)
		}
		return errors.Join(fmt.Errorf("serve HTTP: %w", serveErr), cleanupErr)
	}
}

func (a *Application) start(ctx context.Context) (<-chan error, error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()

	if a.closed {
		return nil, ErrApplicationClosed
	}
	if a.runStarted {
		return nil, ErrApplicationAlreadyRun
	}
	a.runStarted = true

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	a.cancelWorkers = cancelWorkers
	a.cleanerStartAttempted = true
	if err := a.cleaner.Start(workerCtx); err != nil {
		return nil, fmt.Errorf("start session cleaner: %w", err)
	}
	serveErrors, err := a.server.Start(a.address)
	if err != nil {
		return nil, fmt.Errorf("start HTTP server: %w", err)
	}
	if serveErrors == nil {
		return nil, errors.New("start HTTP server: missing serve result channel")
	}
	a.serverStarted = true
	return serveErrors, nil
}

func (a *Application) shutdownWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	return a.Shutdown(ctx)
}

// Shutdown is idempotent. The supplied context is additionally capped by the
// application's shutdown timeout so graceful HTTP shutdown is always bounded.
func (a *Application) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, a.shutdownTimeout)
	defer cancel()

	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	a.closed = true
	a.cleanupOnce.Do(func() {
		a.cleanupErr = a.cleanup(shutdownCtx)
	})
	return a.cleanupErr
}

func (a *Application) cleanup(httpShutdownCtx context.Context) error {
	if a.cancelWorkers != nil {
		a.cancelWorkers()
	}

	var cleanupErrors []error
	if a.serverStarted {
		if err := a.server.Shutdown(httpShutdownCtx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("shutdown HTTP server: %w", err))
			if closeErr := a.server.Close(); closeErr != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("force close HTTP server: %w", closeErr))
			}
		}
	}

	if a.cleanerStartAttempted {
		a.cleaner.Shutdown()
	}

	cleanupErrors = append(
		cleanupErrors,
		runCleanup("close Redis", a.closeRedis),
		runCleanup("close PostgreSQL", a.closePostgres),
		runCleanup("sync logger", a.syncLogger),
	)
	return errors.Join(cleanupErrors...)
}

func runCleanup(operation string, cleanup func() error) error {
	if cleanup == nil {
		return nil
	}
	if err := cleanup(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}
