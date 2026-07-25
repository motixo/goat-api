package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
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
	accessKey := pkg.RedisKey("session", "access", s.UserID)

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

	script, err := redisClinet.GetScript(redisClinet.ScriptCreateSession)
	if err != nil {
		return fmt.Errorf("resolve create-session script: %w", err)
	}
	generation, err := script.Run(
		ctx,
		r.client,
		[]string{sessionkey, jtiKey, userkey, accessKey},
		argv...,
	).Int64()
	if err != nil {
		return err
	}
	if generation == -1 {
		return domainErrors.ErrUserAccessBlocked
	}
	if generation <= 0 {
		return fmt.Errorf("create session returned invalid generation")
	}
	s.SessionGeneration = generation
	return nil
}

func (r *Repository) FindByJTI(
	ctx context.Context,
	jti, expectedUserID, expectedSessionID string,
	expectedCredentialVersion int64,
) (*entity.Session, error) {
	if jti == "" ||
		expectedUserID == "" ||
		expectedSessionID == "" ||
		expectedCredentialVersion <= 0 {
		return nil, nil
	}

	jtiKey := pkg.RedisKey("session", "jti", jti)
	accessKey := pkg.RedisKey("session", "access", expectedUserID)
	script, err := redisClinet.GetScript(redisClinet.ScriptGetSessionByJTI)
	if err != nil {
		return nil, fmt.Errorf("resolve session-by-JTI script: %w", err)
	}
	result, err := script.Run(
		ctx,
		r.client,
		[]string{jtiKey, accessKey},
		jti,
		expectedUserID,
		expectedSessionID,
		expectedCredentialVersion,
	).Slice()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(result) == 1 {
		status, ok := result[0].(int64)
		if ok && status == -1 {
			return nil, domainErrors.ErrUserAccessBlocked
		}
	}
	if len(result) != 5 {
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
	sessionGeneration, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil || sessionGeneration <= 0 {
		return nil, fmt.Errorf("parse session generation")
	}

	return &entity.Session{
		ID:                fields[0],
		UserID:            fields[1],
		CurrentJTI:        fields[2],
		CredentialVersion: credentialVersion,
		SessionGeneration: sessionGeneration,
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
	script, err := redisClinet.GetScript(redisClinet.ScriptListSessions)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve list-sessions script: %w", err)
	}
	result, err := script.Run(ctx, r.client, []string{userKey}, userID, offset, limit).Slice()
	if err != nil {
		return nil, 0, err
	}
	return decodeSessionList(result)
}

const sessionListFieldCount = 10

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
		sessionGeneration, err := strconv.ParseInt(fields[6], 10, 64)
		if err != nil || sessionGeneration <= 0 {
			return nil, 0, fmt.Errorf("parse session session_generation")
		}
		createdAt, err := strconv.ParseInt(fields[7], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse session created_at: %w", err)
		}
		updatedAt, err := strconv.ParseInt(fields[8], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("parse session updated_at: %w", err)
		}
		expiresAt, err := strconv.ParseInt(fields[9], 10, 64)
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
			SessionGeneration: sessionGeneration,
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
	oldJTI, newJTI, expectedUserID, expectedSessionID string,
	expectedCredentialVersion int64,
	ip, device string,
	expiresAt time.Time,
	jtiTTL, sessionTTL int64,
) (string, error) {

	oldJTIKey := pkg.RedisKey("session", "jti", oldJTI)
	newJTIKey := pkg.RedisKey("session", "jti", newJTI)
	accessKey := pkg.RedisKey("session", "access", expectedUserID)

	updatedAt := time.Now().UTC().Unix()

	argv := []interface{}{
		newJTI,
		expectedUserID,
		expectedSessionID,
		expectedCredentialVersion,
		ip,
		device,
		updatedAt,
		expiresAt.Unix(),
		jtiTTL,
		sessionTTL,
	}

	script, err := redisClinet.GetScript(redisClinet.ScriptRotateJTI)
	if err != nil {
		return "", fmt.Errorf("resolve rotate-JTI script: %w", err)
	}
	res, err := script.Run(
		ctx,
		r.client,
		[]string{oldJTIKey, newJTIKey, accessKey},
		argv...,
	).Slice()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to rotate JTI: %w", err)
	}
	if len(res) == 1 {
		status, ok := res[0].(int64)
		if !ok {
			return "", fmt.Errorf("unexpected Redis rotation status type: %T", res[0])
		}
		switch status {
		case -1:
			return "", domainErrors.ErrUserAccessBlocked
		case 0:
			return "", domainErrors.ErrUnauthorized
		}
	}
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected Redis rotation result length: %d", len(res))
	}
	status, ok := res[0].(int64)
	if !ok || status != 1 {
		return "", fmt.Errorf("unexpected Redis rotation status: %v", res[0])
	}
	sessionID, ok := res[1].(string)
	if !ok || sessionID == "" {
		return "", fmt.Errorf("unexpected Redis session ID type: %T", res[1])
	}
	return extractSessionIDFromKey(sessionID), nil
}

func (r *Repository) Delete(ctx context.Context, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	sessionKeys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionKeys = append(sessionKeys, pkg.RedisKey("session", "id", sessionID))
	}

	script, err := redisClinet.GetScript(redisClinet.ScriptDeleteSession)
	if err != nil {
		return fmt.Errorf("resolve delete-session script: %w", err)
	}
	_, err = script.Run(ctx, r.client, sessionKeys).Result()
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

	script, err := redisClinet.GetScript(redisClinet.ScriptDeleteOwnedSessions)
	if err != nil {
		return false, fmt.Errorf("resolve delete-owned-sessions script: %w", err)
	}
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
	script, err := redisClinet.GetScript(redisClinet.ScriptDeleteOtherSessions)
	if err != nil {
		return fmt.Errorf("resolve delete-all-sessions script: %w", err)
	}
	_, err = script.Run(
		ctx,
		r.client,
		[]string{userKey},
		userID,
		"all",
	).Result()
	return err
}

func (r *Repository) BlockAndDeleteAllByUser(
	ctx context.Context,
	userID string,
) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	userKey := pkg.RedisKey("session", "user", userID)
	accessKey := pkg.RedisKey("session", "access", userID)
	script, err := redisClinet.GetScript(redisClinet.ScriptDeleteOtherSessions)
	if err != nil {
		return fmt.Errorf("resolve block-and-delete-all-sessions script: %w", err)
	}
	_, err = script.Run(
		ctx,
		r.client,
		[]string{userKey, accessKey},
		userID,
		"block_all",
	).Result()
	return err
}

func (r *Repository) UnblockUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	accessKey := pkg.RedisKey("session", "access", userID)
	script, err := redisClinet.GetScript(redisClinet.ScriptUnblockUser)
	if err != nil {
		return fmt.Errorf("resolve unblock-user script: %w", err)
	}
	_, err = script.Run(ctx, r.client, []string{accessKey}).Result()
	return err
}

func (r *Repository) DeleteOthersByUser(ctx context.Context, userID, currentSessionID string) (bool, error) {
	userKey := pkg.RedisKey("session", "user", userID)
	currentSessionKey := pkg.RedisKey("session", "id", currentSessionID)

	script, err := redisClinet.GetScript(redisClinet.ScriptDeleteOtherSessions)
	if err != nil {
		return false, fmt.Errorf("resolve delete-other-sessions script: %w", err)
	}
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
	script, err := redisClinet.GetScript(redisClinet.ScriptCleanOrphans)
	if err != nil {
		return fmt.Errorf("resolve clean-orphans script: %w", err)
	}

	iter := r.client.Scan(ctx, 0, "session:user:*", 0).Iterator()

	for iter.Next(ctx) {
		userKey := iter.Val()
		res, runErr := script.Run(ctx, r.client, []string{userKey}).Result()
		if runErr != nil {
			r.logger.Error("remove orphan sessionkey feild", "error", runErr)
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
