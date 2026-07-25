package cron

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/pkg"
)

var ErrSessionCleanerStarted = errors.New("session cleaner already started")

type SessionCleaner struct {
	sessionRepo repository.SessionRepository
	interval    time.Duration
	logger      pkg.Logger

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewSessionCleaner(repo repository.SessionRepository, logger pkg.Logger) *SessionCleaner {
	return &SessionCleaner{
		sessionRepo: repo,
		interval:    24 * time.Hour,
		logger:      logger,
	}
}

func (c *SessionCleaner) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session cleaner context is required")
	}

	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return ErrSessionCleanerStarted
	}
	workerCtx, cancel := context.WithCancel(ctx)
	c.started = true
	c.cancel = cancel
	c.done = make(chan struct{})
	done := c.done
	c.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		c.logger.Info("Session cleaner cron started")
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				if err := c.sessionRepo.DeleteOrphanSessions(workerCtx); err != nil && workerCtx.Err() == nil {
					c.logger.Error("Failed to clean orphan sessions", "error", err)
				}
			}
		}
	}()
	return nil
}

// Shutdown cancels the worker and waits until it can no longer use its
// repository or logger dependencies. It is safe to call repeatedly.
func (c *SessionCleaner) Shutdown() {
	c.mu.Lock()
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	<-done
}
