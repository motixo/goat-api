package jti

import "context"

func (r *Repository) DeleteJTI(ctx context.Context, jti string) error {
	return r.client.Del(ctx, jti).Err()
}
