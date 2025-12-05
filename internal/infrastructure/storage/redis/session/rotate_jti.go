package session

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) RotateJTI(
	ctx context.Context,
	oldJTI, newJTI, ip, device string,
	expiresAt time.Time,
	jtiTTL, sessionTTL int64,
) (string, error) {

	oldJTIKey := r.key("jti", oldJTI)
	newJTIKey := r.key("jti", newJTI)

	updatedAt := time.Now().UTC().Unix()

	argv := []interface{}{
		newJTI,
		ip,
		device,
		updatedAt,
		expiresAt.Unix(),
		jtiTTL,
		sessionTTL,
	}

	script := getScript("rotate_jti")
	res, err := script.Run(ctx, r.client, []string{oldJTIKey, newJTIKey}, argv...).Result()
	if err != nil {
		return "", fmt.Errorf("failed to rotate JTI: %w", err)
	}

	sessionID, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type returned from Redis: %T", res)
	}

	parts := strings.Split(sessionID, ":")
	if len(parts) == 2 {
		sessionID = parts[1]
	}
	return sessionID, nil
}
