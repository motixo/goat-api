package user

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	authUseCase "github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	sessionUseCase "github.com/motixo/goat-api/internal/usecase/session"
	"github.com/redis/go-redis/v9"
)

func TestStatusStateMachineIntegrationActivatesBeforeFirstSession(t *testing.T) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := sessionUseCase.NewUsecase(sessionRepository, logger)
	jwtManager := authInfra.NewJWTManager("status-state-machine-integration-secret")
	authUsecase := authUseCase.NewUsecase(
		userRepository,
		userRepository,
		sessions,
		passwordHasher,
		jwtManager,
		logger,
		authUseCase.AccessTTL(5*time.Minute),
		authUseCase.RefreshTTL(time.Hour),
		authUseCase.SessionTTL(time.Hour),
	)

	const password = "Password1!"
	email := "inactive-" + uuid.NewString() + "@example.com"
	registered, err := authUsecase.Signup(ctx, authUseCase.RegisterInput{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	if registered.Status != valueobject.StatusInactive.String() {
		t.Fatalf("registered status = %q, want inactive", registered.Status)
	}

	accessKey := pkg.RedisKey("session", "access", registered.ID)
	userIndexKey := pkg.RedisKey("session", "user", registered.ID)
	t.Cleanup(func() {
		_ = redisClient.Del(context.Background(), accessKey, userIndexKey).Err()
	})
	assertStatusRedisKeysAbsent(t, ctx, redisClient, accessKey, userIndexKey)

	inactiveLogin, err := authUsecase.Login(ctx, authUseCase.LoginInput{
		Email:    email,
		Password: password,
		IP:       "127.0.0.1",
		Device:   "inactive-login",
	})
	if !errors.Is(err, authorization.ErrPrincipalInactive) {
		t.Fatalf("inactive Login() error = %v, want ErrPrincipalInactive", err)
	}
	if inactiveLogin.AccessToken != "" || inactiveLogin.RefreshToken != "" {
		t.Fatalf("inactive Login() returned tokens: %#v", inactiveLogin)
	}
	assertStatusRedisKeysAbsent(t, ctx, redisClient, accessKey, userIndexKey)

	statusChange := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)
	if err := statusChange.ChangeStatus(ctx, UpdateStatusInput{
		UserID:    registered.ID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	}); err != nil {
		t.Fatalf("ChangeStatus(inactive -> active) error = %v", err)
	}

	persisted, err := userRepository.FindByID(ctx, registered.ID)
	if err != nil {
		t.Fatalf("FindByID(after activation) error = %v", err)
	}
	if persisted.Status != valueobject.StatusActive {
		t.Fatalf("persisted status = %s, want active", persisted.Status)
	}
	assertStatusRedisKeysAbsent(t, ctx, redisClient, accessKey, userIndexKey)

	activeLogin, err := authUsecase.Login(ctx, authUseCase.LoginInput{
		Email:    email,
		Password: password,
		IP:       "127.0.0.1",
		Device:   "first-active-login",
	})
	if err != nil {
		t.Fatalf("first active Login() error = %v", err)
	}
	if activeLogin.AccessToken == "" || activeLogin.RefreshToken == "" {
		t.Fatalf("first active Login() returned empty tokens: %#v", activeLogin)
	}

	claims, err := jwtManager.ParseAndValidate(activeLogin.AccessToken)
	if err != nil {
		t.Fatalf("parse first access token: %v", err)
	}
	t.Cleanup(func() {
		_ = redisClient.Del(
			context.Background(),
			pkg.RedisKey("session", "id", claims.SessionID),
			pkg.RedisKey("session", "jti", claims.JTI),
			accessKey,
			userIndexKey,
		).Err()
	})

	accessState, err := redisClient.HMGet(
		ctx,
		accessKey,
		"blocked",
		"session_generation",
	).Result()
	if err != nil {
		t.Fatalf("read first-login access state: %v", err)
	}
	if len(accessState) != 2 || accessState[0] != "0" || accessState[1] != "1" {
		t.Fatalf(
			"first-login access state = %#v, want unblocked generation 1",
			accessState,
		)
	}

	valid, err := sessions.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		JTI:               claims.JTI,
		CredentialVersion: claims.CredentialVersion,
	})
	if err != nil || !valid {
		t.Fatalf("first active session validation = (%t, %v), want valid", valid, err)
	}
}

func TestStatusStateMachineIntegrationSuspensionCASFailureRemainsBlocked(
	t *testing.T,
) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	sessionRepository := redisSession.NewRepository(redisClient, logger)
	userID := createCredentialVersionIntegrationUser(
		t,
		ctx,
		userRepository,
		passwordHasher,
	)
	current := newPasswordChangeIntegrationSession(userID)
	if err := sessionRepository.Create(ctx, current); err != nil {
		t.Fatalf("create current session: %v", err)
	}
	registerCredentialVersionSessionCleanup(t, redisClient, current)
	before := readStatusIntegrationAccessState(t, ctx, redisClient, userID)

	postgresErr := errors.New("injected PostgreSQL suspension failure")
	failingRepository := &failFirstStatusUpdateUserRepository{
		UserRepository: userRepository,
		requested:      valueobject.StatusSuspended,
		firstFailure:   postgresErr,
	}
	statusChange := NewUsecase(
		failingRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)
	input := UpdateStatusInput{
		UserID:    userID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusSuspended,
	}

	err := statusChange.ChangeStatus(ctx, input)
	if !errors.Is(err, postgresErr) {
		t.Fatalf("ChangeStatus(first suspension) error = %v, want PostgreSQL failure", err)
	}
	persisted, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after failed suspension) error = %v", err)
	}
	if persisted.Status != valueobject.StatusActive {
		t.Fatalf(
			"status after failed suspension = %s, want active",
			persisted.Status,
		)
	}
	blocked := readStatusIntegrationAccessState(t, ctx, redisClient, userID)
	if !blocked.blocked || blocked.generation != before.generation+1 {
		t.Fatalf(
			"access state after failed suspension = %#v, want blocked after %#v",
			blocked,
			before,
		)
	}
	assertStatusRedisKeysAbsent(
		t,
		ctx,
		redisClient,
		pkg.RedisKey("session", "id", current.ID),
		pkg.RedisKey("session", "jti", current.CurrentJTI),
		pkg.RedisKey("session", "user", userID),
	)
	if err := statusChange.ChangeStatus(ctx, input); err != nil {
		t.Fatalf("ChangeStatus(retry suspension) error = %v", err)
	}
	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after suspension retry) error = %v", err)
	}
	if persisted.Status != valueobject.StatusSuspended {
		t.Fatalf(
			"status after suspension retry = %s, want suspended",
			persisted.Status,
		)
	}
	if repeated := readStatusIntegrationAccessState(t, ctx, redisClient, userID); repeated != blocked {
		t.Fatalf(
			"access state after suspension retry = %#v, want idempotent %#v",
			repeated,
			blocked,
		)
	}
}

func TestStatusStateMachineIntegrationReactivationIsSafelyRetryable(
	t *testing.T,
) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := sessionUseCase.NewUsecase(sessionRepository, logger)
	jwtManager := authInfra.NewJWTManager("reactivation-retry-integration-secret")
	authUsecase := authUseCase.NewUsecase(
		userRepository,
		userRepository,
		sessions,
		passwordHasher,
		jwtManager,
		logger,
		authUseCase.AccessTTL(5*time.Minute),
		authUseCase.RefreshTTL(time.Hour),
		authUseCase.SessionTTL(time.Hour),
	)

	userID := createCredentialVersionIntegrationUser(
		t,
		ctx,
		userRepository,
		passwordHasher,
	)
	persisted, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(before login) error = %v", err)
	}

	oldLogin, err := authUsecase.Login(ctx, authUseCase.LoginInput{
		Email:    persisted.Email,
		Password: passwordChangeOldPassword,
		IP:       "127.0.0.1",
		Device:   "pre-suspension",
	})
	if err != nil {
		t.Fatalf("Login(before suspension) error = %v", err)
	}
	oldClaims, err := jwtManager.ParseAndValidate(oldLogin.AccessToken)
	if err != nil {
		t.Fatalf("parse old access token: %v", err)
	}

	accessKey := pkg.RedisKey("session", "access", userID)
	userIndexKey := pkg.RedisKey("session", "user", userID)
	t.Cleanup(func() {
		_ = redisClient.Del(
			context.Background(),
			pkg.RedisKey("session", "id", oldClaims.SessionID),
			pkg.RedisKey("session", "jti", oldClaims.JTI),
			accessKey,
			userIndexKey,
		).Err()
	})
	initialState := readStatusIntegrationAccessState(t, ctx, redisClient, userID)

	statusChange := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)
	if err := statusChange.ChangeStatus(ctx, UpdateStatusInput{
		UserID:    userID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusSuspended,
	}); err != nil {
		t.Fatalf("ChangeStatus(active -> suspended) error = %v", err)
	}
	blockedState := readStatusIntegrationAccessState(t, ctx, redisClient, userID)
	if !blockedState.blocked ||
		blockedState.generation <= initialState.generation {
		t.Fatalf(
			"blocked access state = %#v, want blocked generation after %#v",
			blockedState,
			initialState,
		)
	}
	assertStatusRedisKeysAbsent(
		t,
		ctx,
		redisClient,
		pkg.RedisKey("session", "id", oldClaims.SessionID),
		pkg.RedisKey("session", "jti", oldClaims.JTI),
		userIndexKey,
	)

	postgresErr := errors.New("injected PostgreSQL activation failure")
	activationStarted := make(chan struct{})
	continueActivation := make(chan struct{})
	var releaseActivation sync.Once
	release := func() {
		releaseActivation.Do(func() { close(continueActivation) })
	}
	defer release()
	retryRepository := &failFirstStatusUpdateUserRepository{
		UserRepository: userRepository,
		requested:      valueobject.StatusActive,
		updateStarted:  activationStarted,
		continueUpdate: continueActivation,
		firstFailure:   postgresErr,
	}
	retryStatusChange := NewUsecase(
		retryRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)

	activationResult := make(chan error, 1)
	go func() {
		activationResult <- retryStatusChange.ChangeStatus(ctx, UpdateStatusInput{
			UserID:    userID,
			ActorID:   uuid.NewString(),
			ActorRole: valueobject.RoleAdmin,
			Status:    valueobject.StatusActive,
		})
	}()

	select {
	case <-activationStarted:
	case <-ctx.Done():
		t.Fatalf("wait for PostgreSQL activation boundary: %v", ctx.Err())
	}

	midpointState := readStatusIntegrationAccessState(t, ctx, redisClient, userID)
	if midpointState.blocked ||
		midpointState.generation != blockedState.generation {
		t.Fatalf(
			"midpoint access state = %#v, want unblocked preserved generation %#v",
			midpointState,
			blockedState,
		)
	}
	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(during reactivation) error = %v", err)
	}
	if persisted.Status != valueobject.StatusSuspended {
		t.Fatalf(
			"status during reactivation = %s, want authoritatively suspended",
			persisted.Status,
		)
	}

	midpointLogin, loginErr := authUsecase.Login(ctx, authUseCase.LoginInput{
		Email:    persisted.Email,
		Password: passwordChangeOldPassword,
		IP:       "127.0.0.1",
		Device:   "reactivation-midpoint",
	})
	if !errors.Is(loginErr, domainErrors.ErrAccountSuspended) {
		t.Fatalf(
			"Login(during reactivation) error = %v, want ErrAccountSuspended",
			loginErr,
		)
	}
	if midpointLogin.AccessToken != "" || midpointLogin.RefreshToken != "" {
		t.Fatalf("Login(during reactivation) returned tokens: %#v", midpointLogin)
	}
	assertStatusRedisKeysAbsent(t, ctx, redisClient, userIndexKey)

	release()
	select {
	case err = <-activationResult:
	case <-ctx.Done():
		t.Fatalf("wait for failed PostgreSQL activation: %v", ctx.Err())
	}
	if !errors.Is(err, postgresErr) {
		t.Fatalf("first reactivation error = %v, want PostgreSQL failure", err)
	}
	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after failed reactivation) error = %v", err)
	}
	if persisted.Status != valueobject.StatusSuspended {
		t.Fatalf(
			"status after failed reactivation = %s, want suspended",
			persisted.Status,
		)
	}
	if state := readStatusIntegrationAccessState(t, ctx, redisClient, userID); state != midpointState {
		t.Fatalf(
			"access state after failed PostgreSQL activation = %#v, want %#v",
			state,
			midpointState,
		)
	}

	if err := retryStatusChange.ChangeStatus(ctx, UpdateStatusInput{
		UserID:    userID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	}); err != nil {
		t.Fatalf("retry ChangeStatus(suspended -> active) error = %v", err)
	}
	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after successful retry) error = %v", err)
	}
	if persisted.Status != valueobject.StatusActive {
		t.Fatalf("status after successful retry = %s, want active", persisted.Status)
	}
	if state := readStatusIntegrationAccessState(t, ctx, redisClient, userID); state != midpointState {
		t.Fatalf(
			"access state after successful retry = %#v, want preserved %#v",
			state,
			midpointState,
		)
	}
	assertStatusRedisKeysAbsent(t, ctx, redisClient, userIndexKey)

	oldValid, err := sessions.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            oldClaims.UserID,
		SessionID:         oldClaims.SessionID,
		JTI:               oldClaims.JTI,
		CredentialVersion: oldClaims.CredentialVersion,
	})
	if err != nil || oldValid {
		t.Fatalf(
			"old session after reactivation = (%t, %v), want invalid without error",
			oldValid,
			err,
		)
	}

	newLogin, err := authUsecase.Login(ctx, authUseCase.LoginInput{
		Email:    persisted.Email,
		Password: passwordChangeOldPassword,
		IP:       "127.0.0.1",
		Device:   "post-reactivation",
	})
	if err != nil {
		t.Fatalf("Login(after successful reactivation) error = %v", err)
	}
	if newLogin.AccessToken == "" || newLogin.RefreshToken == "" {
		t.Fatalf("Login(after successful reactivation) returned empty tokens: %#v", newLogin)
	}
	newClaims, err := jwtManager.ParseAndValidate(newLogin.AccessToken)
	if err != nil {
		t.Fatalf("parse new access token: %v", err)
	}
	t.Cleanup(func() {
		_ = redisClient.Del(
			context.Background(),
			pkg.RedisKey("session", "id", newClaims.SessionID),
			pkg.RedisKey("session", "jti", newClaims.JTI),
		).Err()
	})
	newSession, err := sessionRepository.FindByJTI(
		ctx,
		newClaims.JTI,
		newClaims.UserID,
		newClaims.SessionID,
		newClaims.CredentialVersion,
	)
	if err != nil {
		t.Fatalf("FindByJTI(new session) error = %v", err)
	}
	if newSession == nil ||
		newSession.SessionGeneration != midpointState.generation {
		t.Fatalf(
			"new session = %#v, want preserved generation %d",
			newSession,
			midpointState.generation,
		)
	}
}

func assertStatusRedisKeysAbsent(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	keys ...string,
) {
	t.Helper()
	count, err := client.Exists(ctx, keys...).Result()
	if err != nil {
		t.Fatalf("check Redis status keys: %v", err)
	}
	if count != 0 {
		t.Fatalf("Redis status keys present = %d, want 0", count)
	}
}

type statusIntegrationAccessState struct {
	blocked    bool
	generation int64
}

func readStatusIntegrationAccessState(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	userID string,
) statusIntegrationAccessState {
	t.Helper()
	values, err := client.HMGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"blocked",
		"session_generation",
	).Result()
	if err != nil {
		t.Fatalf("read Redis access state: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("Redis access state field count = %d, want 2", len(values))
	}
	blocked, blockedOK := values[0].(string)
	generation, generationOK := values[1].(string)
	if !blockedOK || (blocked != "0" && blocked != "1") ||
		!generationOK {
		t.Fatalf("invalid Redis access state = %#v", values)
	}
	generationValue, err := strconv.ParseInt(generation, 10, 64)
	if err != nil || generationValue <= 0 {
		t.Fatalf("parse Redis access-state generation %q: %v", generation, err)
	}
	return statusIntegrationAccessState{
		blocked:    blocked == "1",
		generation: generationValue,
	}
}

type failFirstStatusUpdateUserRepository struct {
	repository.UserRepository
	requested      valueobject.UserStatus
	updateStarted  chan struct{}
	continueUpdate chan struct{}
	firstFailure   error
	firstUpdate    sync.Once
}

func (r *failFirstStatusUpdateUserRepository) UpdateStatus(
	ctx context.Context,
	userID string,
	expected valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	if requested != r.requested {
		return r.UserRepository.UpdateStatus(ctx, userID, expected, requested)
	}

	first := false
	r.firstUpdate.Do(func() {
		first = true
		if r.updateStarted != nil {
			close(r.updateStarted)
		}
	})
	if first {
		if r.continueUpdate != nil {
			select {
			case <-r.continueUpdate:
			case <-ctx.Done():
				return repository.UserStatusUpdateResult{}, ctx.Err()
			}
		}
		return repository.UserStatusUpdateResult{}, r.firstFailure
	}
	return r.UserRepository.UpdateStatus(ctx, userID, expected, requested)
}
