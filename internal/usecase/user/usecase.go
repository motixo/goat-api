package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
)

const (
	passwordChangeCleanupStageSessionRevocation = "session_revocation"
	passwordChangeSessionCleanupTimeout         = 2 * time.Second
	userDeletionSessionCleanupTimeout           = 2 * time.Second
	userStatusAccessStateTimeout                = 2 * time.Second
)

type UserUseCase struct {
	userRepo       repository.UserRepository
	passwordHasher service.PasswordHasher
	sessionRepo    repository.SessionRepository
	logger         pkg.Logger
	metrics        PasswordChangeCleanupMetrics
}

func NewUsecase(
	r repository.UserRepository,
	passwordHasher service.PasswordHasher,
	logger pkg.Logger,
	sessionRepo repository.SessionRepository,
	metrics PasswordChangeCleanupMetrics,
) UseCase {
	return &UserUseCase{
		userRepo:       r,
		passwordHasher: passwordHasher,
		sessionRepo:    sessionRepo,
		logger:         logger,
		metrics:        metrics,
	}
}

func (us *UserUseCase) CreateUser(ctx context.Context, input CreateInput) (UserOutput, error) {

	us.logger.Info("create user attempt", "email", input.Email)
	if input.Status != valueobject.StatusInactive {
		return UserOutput{}, invalidUserStatusTransition(
			valueobject.StatusUnknown,
			input.Status,
		)
	}

	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return UserOutput{}, err
	}

	usr := &entity.User{
		ID:                uuid.New().String(),
		Email:             input.Email,
		Password:          hashedPassword,
		Status:            input.Status,
		Role:              input.Role,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         time.Now().UTC(),
	}

	err = us.userRepo.Create(ctx, usr)
	if err != nil {
		us.logger.Error("failed to create user", "email", input.Email, "error", err)
		return UserOutput{}, err
	}

	us.logger.Info("user created successfully", "userID", usr.ID, "email", usr.Email)
	return UserOutput{
		ID:        usr.ID,
		Email:     usr.Email,
		Role:      usr.Role.String(),
		Status:    usr.Status.String(),
		CreatedAt: usr.CreatedAt,
	}, nil
}

func (us *UserUseCase) GetUser(ctx context.Context, userID string) (UserOutput, error) {
	us.logger.Info("Fetching user by ID", "userID:", userID)
	user, err := us.userRepo.FindByID(ctx, userID)
	if err != nil {
		us.logger.Error("Failed to fetch user", "userID", userID, "error", err)
		return UserOutput{}, err
	}
	response := UserOutput{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role.String(),
		Status:    user.Status.String(),
		CreatedAt: user.CreatedAt,
	}
	us.logger.Info("User fetched successfully", "userID:", userID)
	return response, nil
}

func (us *UserUseCase) GetUserslist(ctx context.Context, input GetListInput) ([]UserOutput, int64, error) {
	us.logger.Info("Fetching users List")

	allowedRoles := valueobject.VisibleRoles(input.ActorRole)
	if len(allowedRoles) == 0 {
		return []UserOutput{}, 0, nil
	}
	if input.Filter.MatchNone {
		return []UserOutput{}, 0, nil
	}

	//INTERSECT allowed and requested roles

	if len(input.Filter.Roles) != 0 {
		var effectiveRoles []valueobject.UserRole
		allowedMap := make(map[valueobject.UserRole]bool)
		for _, role := range allowedRoles {
			allowedMap[role] = true
		}

		for _, requestedRole := range input.Filter.Roles {
			if allowedMap[requestedRole] {
				effectiveRoles = append(effectiveRoles, requestedRole)
			}
		}

		if len(effectiveRoles) == 0 {
			return []UserOutput{}, 0, nil
		}
		input.Filter.Roles = effectiveRoles
	} else {
		input.Filter.Roles = allowedRoles
	}

	users, total, err := us.userRepo.List(ctx, input.Offset, input.Limit, repository.UserListFilter{
		Statuses: input.Filter.Statuses,
		Roles:    input.Filter.Roles,
		Search:   input.Filter.Search,
	})
	if err != nil {
		us.logger.Error("Failed to fetch users List", "error", err)
		return []UserOutput{}, 0, err
	}

	response := make([]UserOutput, 0, len(users))
	for _, usr := range users {
		r := UserOutput{
			ID:        usr.ID,
			Email:     usr.Email,
			Role:      usr.Role.String(),
			Status:    usr.Status.String(),
			CreatedAt: usr.CreatedAt,
		}
		response = append(response, r)
	}
	us.logger.Info("Users list fetched successfully")
	return response, total, nil
}

func (us *UserUseCase) DeleteUser(ctx context.Context, userID string) error {
	us.logger.Info("attempting to delete user", "user_id", userID)

	exists, err := us.userRepo.ExistsByID(ctx, userID)
	if err != nil {
		us.logger.Error("failed to verify user before deletion", "user_id", userID, "error", err)
		return fmt.Errorf("verify user before deletion: %w", err)
	}
	if !exists {
		return errors.ErrUserNotFound
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(ctx, userDeletionSessionCleanupTimeout)
	cleanupErr := us.sessionRepo.BlockAndDeleteAllByUser(cleanupCtx, userID)
	cancelCleanup()
	if cleanupErr != nil {
		us.logger.Error("failed to revoke user sessions", "user_id", userID, "error", cleanupErr)
		return fmt.Errorf("revoke sessions before user deletion: %w", cleanupErr)
	}

	// PostgreSQL and Redis do not share a transaction. The Redis access-state
	// tombstone is installed before deleting durable user state, so a late login
	// cannot create a usable session and snapshot routes fail closed. A database
	// failure can therefore leave an existing user blocked, but a known Redis
	// enforcement failure is never reported as a successful deletion.
	if err := us.userRepo.Delete(ctx, userID); err != nil {
		us.logger.Error("failed to delete user", "user_id", userID, "error", err)
		return err
	}

	us.logger.Info("user deleted successfully", "user_id", userID)
	return nil
}

func (us *UserUseCase) UpdateUser(ctx context.Context, input UpdateInput) error {
	us.logger.Info("update user attempt", "target_id", input.UserID)
	if input.Status != valueobject.StatusUnknown {
		current, err := us.userRepo.FindByID(ctx, input.UserID)
		if err != nil {
			us.logger.Error("failed to verify user status before update", "user_id", input.UserID, "error", err)
			return err
		}
		if current == nil {
			return errors.ErrUserNotFound
		}
		if !current.Status.IsKnown() {
			return fmt.Errorf("current user status is invalid")
		}
		if current.Status != input.Status {
			return invalidUserStatusTransition(current.Status, input.Status)
		}
	}

	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "user_id", input.UserID, "error", err)
		return err
	}

	usr := entity.User{
		ID:       input.UserID,
		Email:    input.Email,
		Password: hashedPassword,
		// Status changes are owned exclusively by ChangeStatus and its
		// compare-and-set repository operation. The generic update may verify a
		// supplied same-status value for API compatibility, but must never
		// persist it.
		Status: valueobject.StatusUnknown,
		Role:   input.Role,
	}

	if err := us.userRepo.Update(ctx, &usr); err != nil {
		us.logger.Error("failed to update user", "user_id", input.UserID, "error", err)
		return err
	}
	us.logger.Info("user updated successfully", "target_id", input.UserID)
	return nil
}

func (us *UserUseCase) ChangeEmail(ctx context.Context, input UpdateEmailInput) error {
	us.logger.Info("update user attempt", "user_id", input.UserID)

	usr := &entity.User{
		ID:    input.UserID,
		Email: input.Email,
	}

	if err := us.userRepo.Update(ctx, usr); err != nil {
		us.logger.Error("user update failed", "user_id", input.UserID, "error", err)
		return err
	}

	us.logger.Info("user successfully updated", "user_id", input.UserID)
	return nil
}

func (us *UserUseCase) ChangePassword(ctx context.Context, input UpdatePassInput) error {
	us.logger.Info("change password attempt", "user_id", input.UserID)
	if input.OldPassword == input.NewPassword {
		us.logger.Error("passwords are same", "user_id", input.UserID)
		return errors.ErrPasswordSameAsCurrent
	}

	user, err := us.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		us.logger.Error("user lookup failed", "user_id", input.UserID, "error", err)
		return err
	}
	if user == nil {
		return errors.ErrUserNotFound
	}

	if !us.passwordHasher.Verify(ctx, input.OldPassword, user.Password) {
		return errors.ErrInvalidPassword
	}

	hashedPassword, err := us.passwordHasher.Hash(ctx, input.NewPassword)
	if err != nil {
		us.logger.Error("password hashing failed", "user_id", input.UserID, "error", err)
		return err
	}

	// PostgreSQL and Redis do not share a transaction. The password hash and
	// credential version therefore change first, atomically in one PostgreSQL
	// statement. That durable commit is both the security and success boundary:
	// every session holding the previous version becomes invalid before
	// best-effort Redis cleanup starts.
	version, err := us.userRepo.UpdatePassword(ctx, user.ID, hashedPassword)
	if err != nil {
		us.logger.Error("user update failed", "user_id", input.UserID, "error", err)
		return err
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(ctx, passwordChangeSessionCleanupTimeout)
	cleanupErr := us.sessionRepo.DeleteAllByUser(cleanupCtx, user.ID)
	cancelCleanup()
	if cleanupErr != nil {
		us.observePasswordChangeCleanupFailure(
			input.UserID,
			version,
			passwordChangeCleanupStageSessionRevocation,
			cleanupErr,
		)
		return nil
	}

	us.logger.Info(
		"password and credential version updated; session cleanup completed",
		"user_id", input.UserID,
		"credential_version", version,
	)
	return nil

}

func (us *UserUseCase) observePasswordChangeCleanupFailure(
	userID string,
	credentialVersion int64,
	stage string,
	err error,
) {
	us.logger.Error(
		"post-commit password-change session cleanup failed",
		"user_id", userID,
		"credential_version", credentialVersion,
		"cleanup_stage", stage,
		"credential_change_committed", true,
		"error", err,
	)
	if us.metrics != nil {
		us.metrics.RecordPasswordChangeCleanupFailure(stage)
	}
}

func (us *UserUseCase) ChangeRole(ctx context.Context, input UpdateRoleInput) error {
	us.logger.Info("change role attempt", "UserID:", input.UserID)
	usr := &entity.User{
		ID:   input.UserID,
		Role: input.Role,
	}
	if err := us.userRepo.Update(ctx, usr); err != nil {
		us.logger.Error("change user role faild", "user_id", input.UserID, "error", err)
		return err
	}

	us.logger.Info("user role changed successfully", "user_id:", input.UserID)
	return nil
}

func (us *UserUseCase) ChangeStatus(ctx context.Context, input UpdateStatusInput) error {
	us.logger.Info("change status attempt", "target_id", input.UserID, "actor_id", input.ActorID)

	target, err := us.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		us.logger.Error("change user status faild", "target_id", input.UserID, "actor_id", input.ActorID, "error", err)
		return err
	}
	if target == nil {
		return errors.ErrUserNotFound
	}

	if !input.ActorRole.CanModifyTargetRole(target.Role) {
		us.logger.Error("user not permission to perform this action", "target_id", input.UserID, "actor_id", input.ActorID)
		return errors.ErrForbidden
	}

	currentStatus := target.Status
	if !currentStatus.IsKnown() {
		return fmt.Errorf("current user status is invalid")
	}
	if !currentStatus.CanTransitionTo(input.Status) {
		return invalidUserStatusTransition(currentStatus, input.Status)
	}
	if currentStatus == input.Status {
		us.logger.Info(
			"user status already applied",
			"user_id", input.UserID,
			"status", currentStatus.String(),
		)
		return nil
	}

	var persistenceFailureMessage string
	switch {
	case currentStatus == valueobject.StatusInactive &&
		input.Status == valueobject.StatusActive:
		// An inactive user has never owned a session. First login initializes
		// the Redis access state atomically with session creation, so activation
		// itself has no Redis state to restore.
		persistenceFailureMessage = "activate user failed"
	case currentStatus == valueobject.StatusActive &&
		input.Status == valueobject.StatusSuspended:
		// Redis is the immediate request-time enforcement boundary. Blocking and
		// generation advancement happen before PostgreSQL so a stale login or
		// snapshot token cannot create or use a session after this operation
		// linearizes. A later PostgreSQL failure intentionally leaves access
		// blocked.
		accessCtx, cancelAccess := context.WithTimeout(
			ctx,
			userStatusAccessStateTimeout,
		)
		err = us.sessionRepo.BlockAndDeleteAllByUser(accessCtx, input.UserID)
		cancelAccess()
		if err != nil {
			us.logger.Error(
				"failed to block user access before suspension",
				"user_id", input.UserID,
				"error", err,
			)
			return fmt.Errorf("block user access before suspension: %w", err)
		}
		persistenceFailureMessage = "failed to persist suspended status; user remains blocked"
	case currentStatus == valueobject.StatusSuspended &&
		input.Status == valueobject.StatusActive:
		// PostgreSQL remains authoritatively suspended while Redis is unblocked.
		// Suspension already removed old sessions and advanced the generation,
		// and login/refresh check PostgreSQL before creating or rotating a
		// session. A database failure therefore remains fail closed and can be
		// retried by repeating this idempotent Redis operation.
		accessCtx, cancelAccess := context.WithTimeout(
			ctx,
			userStatusAccessStateTimeout,
		)
		err = us.sessionRepo.UnblockUser(accessCtx, input.UserID)
		cancelAccess()
		if err != nil {
			us.logger.Error(
				"failed to unblock user before reactivation",
				"user_id", input.UserID,
				"error", err,
			)
			return fmt.Errorf("unblock user before reactivation: %w", err)
		}
		persistenceFailureMessage = "failed to persist active status after Redis unblock; PostgreSQL remains authoritative"
	default:
		return fmt.Errorf("allowed user status transition is unsupported")
	}

	result, err := us.userRepo.UpdateStatus(
		ctx,
		input.UserID,
		currentStatus,
		input.Status,
	)
	if err != nil {
		us.logger.Error(
			persistenceFailureMessage,
			"user_id", input.UserID,
			"error", err,
		)
		return err
	}

	switch result.Outcome {
	case repository.UserStatusUpdateApplied:
		if result.CurrentStatus != input.Status {
			return fmt.Errorf("compare-and-set user status returned an invalid applied result")
		}
	case repository.UserStatusUpdateAlreadyApplied:
		if result.CurrentStatus != input.Status {
			return fmt.Errorf("compare-and-set user status returned an invalid idempotent result")
		}
		us.logger.Info(
			"user status was applied by a concurrent request",
			"user_id", input.UserID,
			"status", input.Status.String(),
		)
		return nil
	case repository.UserStatusUpdateNotFound:
		return fmt.Errorf("compare-and-set user status: %w", errors.ErrUserNotFound)
	case repository.UserStatusUpdateConflict:
		if !result.CurrentStatus.IsKnown() ||
			result.CurrentStatus == input.Status {
			return fmt.Errorf("compare-and-set user status returned an invalid conflict result")
		}
		return invalidUserStatusTransition(result.CurrentStatus, input.Status)
	default:
		return fmt.Errorf("compare-and-set user status returned an invalid outcome")
	}

	us.logger.Info("user status changed successfully", "user_id", input.UserID)
	return nil
}

func invalidUserStatusTransition(
	current valueobject.UserStatus,
	next valueobject.UserStatus,
) error {
	return fmt.Errorf(
		"%w: %s to %s",
		errors.ErrInvalidUserStatusTransition,
		current,
		next,
	)
}
