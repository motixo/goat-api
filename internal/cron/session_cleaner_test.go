package cron

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/repository"
)

func TestSessionCleanerShutdownCancelsAndWaitsForActiveCleanup(t *testing.T) {
	t.Parallel()

	repo := &blockingSessionRepository{
		entered:   make(chan struct{}),
		cancelled: make(chan struct{}),
		release:   make(chan struct{}),
	}
	cleaner := NewSessionCleaner(repo, discardCleanerLogger{})
	cleaner.interval = time.Millisecond

	if err := cleaner.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("session cleanup did not start")
	}

	shutdownDone := make(chan struct{})
	go func() {
		cleaner.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-repo.cancelled:
	case <-time.After(time.Second):
		t.Fatal("Shutdown() did not cancel the active cleanup")
	}
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown() returned before the active cleanup finished")
	case <-time.After(20 * time.Millisecond):
	}

	close(repo.release)
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("Shutdown() did not return after the active cleanup finished")
	}

	cleaner.Shutdown()
	if err := cleaner.Start(context.Background()); !errors.Is(err, ErrSessionCleanerStarted) {
		t.Fatalf("second Start() error = %v, want ErrSessionCleanerStarted", err)
	}
}

type blockingSessionRepository struct {
	repository.SessionRepository
	entered       chan struct{}
	cancelled     chan struct{}
	release       chan struct{}
	enterOnce     sync.Once
	cancelledOnce sync.Once
}

func (r *blockingSessionRepository) DeleteOrphanSessions(ctx context.Context) error {
	r.enterOnce.Do(func() {
		close(r.entered)
	})
	<-ctx.Done()
	r.cancelledOnce.Do(func() {
		close(r.cancelled)
	})
	<-r.release
	return ctx.Err()
}

type discardCleanerLogger struct{}

func (discardCleanerLogger) Info(string, ...any)  {}
func (discardCleanerLogger) Error(string, ...any) {}
func (discardCleanerLogger) Warn(string, ...any)  {}
func (discardCleanerLogger) Debug(string, ...any) {}
func (discardCleanerLogger) Panic(string, ...any) {}
