package main

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestApplicationRunCleansUpInReverseDependencyOrderAndOnlyOnce(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	server := newFakeLifecycleServer(recorder)
	runCtx, cancel := context.WithCancel(context.Background())
	server.onStart = cancel
	app := newApplication(applicationResources{
		address:         "127.0.0.1:0",
		shutdownTimeout: time.Second,
		server:          server,
		cleaner:         &fakeLifecycleWorker{recorder: recorder},
		closeRedis:      recorder.action("redis.close", nil),
		closePostgres:   recorder.action("postgres.close", nil),
		syncLogger:      recorder.action("logger.sync", nil),
	})

	defer cancel()
	if err := app.Run(runCtx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
	if err := app.Run(context.Background()); !errors.Is(err, ErrApplicationClosed) {
		t.Fatalf("Run() after Shutdown error = %v, want ErrApplicationClosed", err)
	}

	want := []string{
		"cleaner.start",
		"http.start",
		"http.shutdown",
		"cleaner.shutdown",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle order = %v, want %v", got, want)
	}
}

func TestApplicationServerStartFailureStopsStartedWorkerBeforeDependencies(t *testing.T) {
	t.Parallel()

	startErr := errors.New("bind failed")
	recorder := &lifecycleRecorder{}
	server := newFakeLifecycleServer(recorder)
	server.startErr = startErr
	app := newApplication(applicationResources{
		address:         "invalid",
		shutdownTimeout: time.Second,
		server:          server,
		cleaner:         &fakeLifecycleWorker{recorder: recorder},
		closeRedis:      recorder.action("redis.close", nil),
		closePostgres:   recorder.action("postgres.close", nil),
		syncLogger:      recorder.action("logger.sync", nil),
	})

	err := app.Run(context.Background())
	if !errors.Is(err, startErr) {
		t.Fatalf("Run() error = %v, want wrapped start error", err)
	}

	want := []string{
		"cleaner.start",
		"http.start",
		"cleaner.shutdown",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle order = %v, want %v", got, want)
	}
}

func TestApplicationBoundsHTTPShutdownAndForceClosesBeforeDependencies(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	server := newFakeLifecycleServer(recorder)
	runCtx, cancel := context.WithCancel(context.Background())
	server.onStart = cancel
	server.shutdown = func(ctx context.Context) error {
		recorder.append("http.shutdown")
		<-ctx.Done()
		return ctx.Err()
	}
	app := newApplication(applicationResources{
		address:         "127.0.0.1:0",
		shutdownTimeout: 20 * time.Millisecond,
		server:          server,
		cleaner:         &fakeLifecycleWorker{recorder: recorder},
		closeRedis:      recorder.action("redis.close", nil),
		closePostgres:   recorder.action("postgres.close", nil),
		syncLogger:      recorder.action("logger.sync", nil),
	})

	defer cancel()
	startedAt := time.Now()
	err := app.Run(runCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want wrapped context deadline", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("bounded shutdown took %s", elapsed)
	}

	want := []string{
		"cleaner.start",
		"http.start",
		"http.shutdown",
		"http.close",
		"cleaner.shutdown",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle order = %v, want %v", got, want)
	}
}

func TestApplicationSurfacesServeAndCleanupErrors(t *testing.T) {
	t.Parallel()

	serveErr := errors.New("serve failed")
	shutdownErr := errors.New("HTTP shutdown failed")
	forceCloseErr := errors.New("HTTP force close failed")
	redisErr := errors.New("redis close failed")
	postgresErr := errors.New("postgres close failed")
	loggerErr := errors.New("logger sync failed")
	recorder := &lifecycleRecorder{}
	server := newFakeLifecycleServer(recorder)
	server.serveErrors <- serveErr
	server.shutdown = func(context.Context) error {
		recorder.append("http.shutdown")
		return shutdownErr
	}
	server.closeErr = forceCloseErr

	app := newApplication(applicationResources{
		address:         "127.0.0.1:0",
		shutdownTimeout: time.Second,
		server:          server,
		cleaner:         &fakeLifecycleWorker{recorder: recorder},
		closeRedis:      recorder.action("redis.close", redisErr),
		closePostgres:   recorder.action("postgres.close", postgresErr),
		syncLogger:      recorder.action("logger.sync", loggerErr),
	})

	err := app.Run(context.Background())
	for name, target := range map[string]error{
		"serve":          serveErr,
		"HTTP shutdown":  shutdownErr,
		"HTTP close":     forceCloseErr,
		"redis close":    redisErr,
		"postgres close": postgresErr,
		"logger sync":    loggerErr,
	} {
		if !errors.Is(err, target) {
			t.Errorf("Run() error = %v, want wrapped %s error", err, name)
		}
	}
}

type lifecycleRecorder struct {
	mu            sync.Mutex
	events        []string
	depsAreClosed bool
}

func (r *lifecycleRecorder) append(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.depsAreClosed && event == "cleaner.shutdown" {
		event = "USE_AFTER_CLOSE:" + event
	}
	if event == "redis.close" {
		r.depsAreClosed = true
	}
	r.events = append(r.events, event)
}

func (r *lifecycleRecorder) action(event string, result error) func() error {
	return func() error {
		r.append(event)
		return result
	}
}

func (r *lifecycleRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type fakeLifecycleServer struct {
	recorder    *lifecycleRecorder
	serveErrors chan error
	startErr    error
	closeErr    error
	shutdown    func(context.Context) error
	onStart     func()
}

func newFakeLifecycleServer(recorder *lifecycleRecorder) *fakeLifecycleServer {
	server := &fakeLifecycleServer{
		recorder:    recorder,
		serveErrors: make(chan error, 1),
	}
	server.shutdown = func(context.Context) error {
		recorder.append("http.shutdown")
		return nil
	}
	return server
}

func (s *fakeLifecycleServer) Start(string) (<-chan error, error) {
	s.recorder.append("http.start")
	if s.startErr != nil {
		return nil, s.startErr
	}
	if s.onStart != nil {
		s.onStart()
	}
	return s.serveErrors, nil
}

func (s *fakeLifecycleServer) Shutdown(ctx context.Context) error {
	return s.shutdown(ctx)
}

func (s *fakeLifecycleServer) Close() error {
	s.recorder.append("http.close")
	return s.closeErr
}

type fakeLifecycleWorker struct {
	recorder *lifecycleRecorder
}

func (w *fakeLifecycleWorker) Start(context.Context) error {
	w.recorder.append("cleaner.start")
	return nil
}

func (w *fakeLifecycleWorker) Shutdown() {
	w.recorder.append("cleaner.shutdown")
}
