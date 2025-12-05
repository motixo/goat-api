package session

import (
	"fmt"

	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	client *redis.Client
}

func NewRepository(client *redis.Client) repository.SessionRepository {
	return &Repository{client: client}
}

func (r *Repository) key(perfix string, sessionID string) string {
	return fmt.Sprintf("%s:%s", perfix, sessionID)
}
