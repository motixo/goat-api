package jti

import (
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	client *redis.Client
}

func NewRepository(client *redis.Client) jti.Repository {
	return &Repository{client: client}
}
