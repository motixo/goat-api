package repository

import "context"

type JTIRepository interface {
	SaveJTI(ctx context.Context, userID string, tokenID string, ttlSeconds int) error
	Exists(ctx context.Context, tokenID string) (bool, error)
	DeleteJTI(ctx context.Context, tokenID string) error
}
