package jti

import (
	"context"
	"time"
)

func (r *Repository) SaveJTI(ctx context.Context, userID string, jti string, ttlSeconds int) error {
	return r.client.Set(ctx, jti, userID, time.Duration(ttlSeconds)*time.Second).Err()
}
