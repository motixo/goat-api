package jti

import "context"

func (r *Repository) Exists(ctx context.Context, jti string) (bool, error) {
	val, err := r.client.Exists(ctx, jti).Result()
	return val == 1, err
}
