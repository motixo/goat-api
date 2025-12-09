package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/infra/helper"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	client *redis.Client
}

func NewRepository(client *redis.Client) repository.SessionRepository {
	return &Repository{client: client}
}

func (r *Repository) Create(ctx context.Context, s *entity.Session) error {
	if s.SessionTTLSeconds <= 0 || s.JTITTLSeconds <= 0 {
		return fmt.Errorf("TTL values must be positive")
	}
	sessionkey := helper.Key("session", "id", s.ID)
	jtiKey := helper.Key("session", "jti", s.CurrentJTI)
	userkey := helper.Key("session", "user", s.UserID)

	argv := []interface{}{
		"id", s.ID,
		"user_id", s.UserID,
		"device", s.Device,
		"ip", s.IP,
		"created_at", s.CreatedAt.Unix(),
		"updated_at", s.UpdatedAt.Unix(),
		"expires_at", s.ExpiresAt.Unix(),
		"current_jti", s.CurrentJTI,
		s.SessionTTLSeconds,
		s.JTITTLSeconds,
	}

	script := getScript("create_session")
	_, err := script.Run(ctx, r.client, []string{sessionkey, jtiKey, userkey}, argv...).Result()
	return err
}

func (r *Repository) ListByUser(ctx context.Context, userID string, offset, limit int) ([]*entity.Session, int64, error) {
	userKey := helper.Key("session", "user", userID)
	end := int64(offset) + int64(limit) - 1

	sessionKeys, err := r.client.ZRevRange(ctx, userKey, int64(offset), end).Result()
	if err != nil {
		return nil, 0, err
	}

	total, err := r.client.ZCard(ctx, userKey).Result()
	if err != nil {
		return nil, 0, err
	}

	sessions := make([]*entity.Session, 0, len(sessionKeys))

	for _, sessionKey := range sessionKeys {
		fields, err := r.client.HGetAll(ctx, sessionKey).Result()
		if err != nil {
			return nil, 0, err
		}

		if len(fields) == 0 {
			continue
		}

		s := &entity.Session{
			ID:         fields["id"],
			UserID:     fields["user_id"],
			Device:     fields["device"],
			IP:         fields["ip"],
			CurrentJTI: fields["current_jti"],
		}

		if createdAt, err := strconv.ParseInt(fields["created_at"], 10, 64); err == nil {
			s.CreatedAt = time.Unix(createdAt, 0).UTC()
		}
		if updatedAt, err := strconv.ParseInt(fields["updated_at"], 10, 64); err == nil {
			s.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		}
		if expiresAt, err := strconv.ParseInt(fields["expires_at"], 10, 64); err == nil {
			s.ExpiresAt = time.Unix(expiresAt, 0).UTC()
		}

		sessions = append(sessions, s)
	}

	return sessions, total, nil
}

func (r *Repository) ExistsJTI(ctx context.Context, jti string) (bool, error) {
	jtiKey := helper.Key("session", "jti", jti)
	val, err := r.client.Exists(ctx, jtiKey).Result()
	return val == 1, err
}

func (r *Repository) RotateJTI(
	ctx context.Context,
	oldJTI, newJTI, ip, device string,
	expiresAt time.Time,
	jtiTTL, sessionTTL int64,
) (string, error) {

	oldJTIKey := helper.Key("session", "jti", oldJTI)
	newJTIKey := helper.Key("session", "jti", newJTI)

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

	rawID := extractSessionIDFromKey(sessionID)
	return rawID, nil
}

func (r *Repository) Delete(ctx context.Context, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	sessionKeys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionKeys = append(sessionKeys, helper.Key("session", "id", sessionID))
	}

	script := getScript("delete_session")
	_, err := script.Run(ctx, r.client, sessionKeys).Result()
	return err
}

func extractSessionIDFromKey(key string) string {
	const prefix = "session:id:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return key
}
