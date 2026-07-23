package cron

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/pkg"
)

type SessionCleaner struct {
	sessionRepo repository.SessionRepository
	interval    time.Duration
	logger      pkg.Logger
}

func NewSessionCleaner(repo repository.SessionRepository, logger pkg.Logger) *SessionCleaner {
	return &SessionCleaner{
		sessionRepo: repo,
		interval:    24 * time.Hour,
		logger:      logger,
	}
}

func (c *SessionCleaner) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		c.logger.Info("Session cleaner cron started")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.sessionRepo.DeleteOrphanSessions(ctx); err != nil {
					c.logger.Error("Failed to clean orphan sessions", "error", err)
				}
			}
		}
	}()
}
