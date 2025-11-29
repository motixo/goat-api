package session

import (
	"fmt"

	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	client *redis.Client
}

func NewRepository(client *redis.Client) session.Repository {
	return &Repository{client: client}
}

func (r *Repository) key(perfix string, sessionID string) string {
	return fmt.Sprintf("%s:%s", perfix, sessionID)
}
