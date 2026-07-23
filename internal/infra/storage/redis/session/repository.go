package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	redisClinet "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	client *redis.Client
	logger pkg.Logger
}

func NewRepository(client *redis.Client, logger pkg.Logger) repository.SessionRepository {
	return &Repository{
		client: client,
		logger: logger,
	}
}

func (r *Repository) Create(ctx context.Context, s *entity.Session) error {
	if s.SessionTTLSeconds <= 0 || s.JTITTLSeconds <= 0 {
		return fmt.Errorf("TTL values must be positive")
	}
	if s.CredentialVersion <= 0 {
		return fmt.Errorf("credential version must be positive")
	}
	sessionkey := pkg.RedisKey("session", "id", s.ID)
	jtiKey := pkg.RedisKey("session", "jti", s.CurrentJTI)
	userkey := pkg.RedisKey("session", "user", s.UserID)

	argv := []interface{}{
		"id", s.ID,
		"user_id", s.UserID,
		"device", s.Device,
		"ip", s.IP,
		"created_at", s.CreatedAt.Unix(),
		"updated_at", s.UpdatedAt.Unix(),
		"expires_at", s.ExpiresAt.Unix(),
		"current_jti", s.CurrentJTI,
		"credential_version", s.CredentialVersion,
		s.SessionTTLSeconds,
		s.JTITTLSeconds,
	}

	script := redisClinet.GetScript("create_session")
	_, err := script.Run(ctx, r.client, []string{sessionkey, jtiKey, userkey}, argv...).Result()
	return err
}

func (r *Repository) FindByJTI(ctx context.Context, jti string) (*entity.Session, error) {
	if jti == "" {
		return nil, nil
	}

	jtiKey := pkg.RedisKey("session", "jti", jti)
	script := redisClinet.GetScript("get_session_by_jti")
	result, err := script.Run(ctx, r.client, []string{jtiKey}, jti).Slice()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(result) != 4 {
		return nil, fmt.Errorf("unexpected Redis session field count: %d", len(result))
	}

	fields := make([]string, len(result))
	for index := range result {
		value, ok := result[index].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected Redis session field type at index %d: %T", index, result[index])
		}
		fields[index] = value
	}
	credentialVersion, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || credentialVersion <= 0 {
		return nil, fmt.Errorf("parse session credential version")
	}

	return &entity.Session{
		ID:                fields[0],
		UserID:            fields[1],
		CurrentJTI:        fields[2],
		CredentialVersion: credentialVersion,
	}, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string, offset, limit int) ([]*entity.Session, int64, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("user ID is required")
	}
	if offset < 0 || limit < 0 {
		return nil, 0, fmt.Errorf("session pagination values must not be negative")
	}

	userKey := pkg.RedisKey("session", "user", userID)
	script := redisClinet.GetScript("list_sessions")
	result, err := script.Run(ctx, r.client, []string{userKey}, userID, offset, limit).Slice()
	if err != nil {
		return nil, 0, err
	}
	return decodeSessionList(result)
}

const sessionListFieldCount = 9

func decodeSessionList(result []any) ([]*entity.Session, int64, error) {
	if len(result) == 0 {
		return nil, 0, fmt.Errorf("redis session list returned no total")
	}
	total, ok := result[0].(int64)
	if !ok || total < 0 {
		return nil, 0, fmt.Errorf("unexpected redis session total: %T", result[0])
	}
	if (len(result)-1)%sessionListFieldCount != 0 {
		return nil, 0, fmt.Errorf("unexpected redis session list field count: %d", len(result)-1)
	}

	sessions := make([]*entity.Session, 0, (len(result)-1)/sessionListFieldCount)
	for i := 1; i < len(result); i += sessionListFieldCount {
		fields := make([]string, sessionListFieldCount)
		for fieldIndex := range fields {
			value, ok := result[i+fieldIndex].(string)
			if !ok {
				return nil, 0, fmt.Errorf("unexpected redis session field type at index %d: %T", i+fieldIndex, result[i+fieldIndex])
			}
			fields[fieldIndex] = value
		}

		credentialVersion, err := strconv.ParseInt(fields[5], 10, 64)
		if err != nil || credentialVersion <= 0 {
			return nil, 0, fmt.Errorf("parse session credential_version")
		}
		createdAt, err := strconv.ParseInt(fields[6], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse session created_at: %w", err)
		}
		updatedAt, err := strconv.ParseInt(fields[7], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse session updated_at: %w", err)
		}
		expiresAt, err := strconv.ParseInt(fields[8], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse session expires_at: %w", err)
		}

		s := &entity.Session{
			ID:                fields[0],
			UserID:            fields[1],
			Device:            fields[2],
			IP:                fields[3],
			CurrentJTI:        fields[4],
			CredentialVersion: credentialVersion,
			CreatedAt:         time.Unix(createdAt, 0).UTC(),
			UpdatedAt:         time.Unix(updatedAt, 0).UTC(),
			ExpiresAt:         time.Unix(expiresAt, 0).UTC(),
		}
		sessions = append(sessions, s)
	}

	return sessions, total, nil
}

func (r *Repository) RotateJTI(
	ctx context.Context,
	oldJTI, newJTI, expectedUserID string,
	expectedCredentialVersion int64,
	ip, device string,
	expiresAt time.Time,
	jtiTTL, sessionTTL int64,
) (string, error) {

	oldJTIKey := pkg.RedisKey("session", "jti", oldJTI)
	newJTIKey := pkg.RedisKey("session", "jti", newJTI)

	updatedAt := time.Now().UTC().Unix()

	argv := []interface{}{
		newJTI,
		expectedUserID,
		expectedCredentialVersion,
		ip,
		device,
		updatedAt,
		expiresAt.Unix(),
		jtiTTL,
		sessionTTL,
	}

	script := redisClinet.GetScript("rotate_jti")
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
		sessionKeys = append(sessionKeys, pkg.RedisKey("session", "id", sessionID))
	}

	script := redisClinet.GetScript("delete_session")
	_, err := script.Run(ctx, r.client, sessionKeys).Result()
	return err
}

func (r *Repository) DeleteByUser(ctx context.Context, userID string, sessionIDs []string) (bool, error) {
	if len(sessionIDs) == 0 {
		return true, nil
	}

	sessionKeys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionKeys = append(sessionKeys, pkg.RedisKey("session", "id", sessionID))
	}

	script := redisClinet.GetScript("delete_owned_sessions")
	deleted, err := script.Run(ctx, r.client, sessionKeys, userID).Int64()
	if err != nil {
		return false, err
	}
	return deleted == 1, nil
}

func (r *Repository) DeleteAllByUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	userKey := pkg.RedisKey("session", "user", userID)
	script := redisClinet.GetScript("delete_other_sessions")
	_, err := script.Run(
		ctx,
		r.client,
		[]string{userKey},
		userID,
		"all",
	).Result()
	return err
}

func (r *Repository) DeleteOthersByUser(ctx context.Context, userID, currentSessionID string) (bool, error) {
	userKey := pkg.RedisKey("session", "user", userID)
	currentSessionKey := pkg.RedisKey("session", "id", currentSessionID)

	script := redisClinet.GetScript("delete_other_sessions")
	deleted, err := script.Run(
		ctx,
		r.client,
		[]string{userKey, currentSessionKey},
		userID,
		"others",
	).Int64()
	if err != nil {
		return false, err
	}
	return deleted >= 0, nil
}

func (r *Repository) DeleteOrphanSessions(ctx context.Context) error {
	script := redisClinet.GetScript("clean_orphans")

	iter := r.client.Scan(ctx, 0, "session:user:*", 0).Iterator()

	for iter.Next(ctx) {
		userKey := iter.Val()
		res, err := script.Run(ctx, r.client, []string{userKey}).Result()
		if err != nil {
			r.logger.Error("remove orphan sessionkey feild", "error", err)
		}
		if removed, ok := res.(int64); ok && removed > 0 {
			r.logger.Info("orphan sessionkeys are removed", "count", removed)
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	return nil
}

func extractSessionIDFromKey(key string) string {
	const prefix = "session:id:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return key
}
