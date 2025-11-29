package session

import "context"

func (r *Repository) ExistsJTI(ctx context.Context, jti string) (bool, error) {
	jtiKey := r.key("jti", jti)
	val, err := r.client.Exists(ctx, jtiKey).Result()
	return val == 1, err
}
