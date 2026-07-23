package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
)

const (
	passwordChangeCleanupStageSessionRevocation = "session_revocation"
	passwordChangeSessionCleanupTimeout         = 2 * time.Second
	userDeletionSessionCleanupTimeout           = 2 * time.Second
)

type UserUseCase struct {
	userRepo                    repository.UserRepository
	passwordHasher              service.PasswordHasher
	userCache                   service.UserCacheService
	sessionRepo                 repository.SessionRepository
	publisher                   event.Publisher
	logger                      pkg.Logger
	passwordChangeCleanupMetric PasswordChangeCleanupMetrics
}

func NewUsecase(
	r repository.UserRepository,
	passwordHasher service.PasswordHasher,
	logger pkg.Logger,
	sessionRepo repository.SessionRepository,
	userCache service.UserCacheService,
	publisher event.Publisher,
	passwordChangeCleanupMetric PasswordChangeCleanupMetrics,
) UseCase {
	return &UserUseCase{
		userRepo:                    r,
		passwordHasher:              passwordHasher,
		sessionRepo:                 sessionRepo,
		userCache:                   userCache,
		publisher:                   publisher,
		logger:                      logger,
		passwordChangeCleanupMetric: passwordChangeCleanupMetric,
	}
}

func (us *UserUseCase) CreateUser(ctx context.Context, input CreateInput) (UserOutput, error) {

	us.logger.Info("create user attempt", "email", input.Email)
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

	actorRole, err := us.userCache.GetUserRole(ctx, input.ActorID)
	if err != nil {
		us.logger.Error("change user status faild", "target_id", input.ActorID, "error", err)
		return []UserOutput{}, 0, err
	}
	allowedRoles := valueobject.VisibleRoles(actorRole)
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
	cleanupErr := us.sessionRepo.DeleteAllByUser(cleanupCtx, userID)
	cancelCleanup()
	if cleanupErr != nil {
		us.logger.Error("failed to revoke user sessions", "user_id", userID, "error", cleanupErr)
		return fmt.Errorf("revoke sessions before user deletion: %w", cleanupErr)
	}

	if err := us.userCache.ClearCache(ctx, userID); err != nil {
		us.logger.Error("failed to invalidate user cache before deletion", "user_id", userID, "error", err)
		return fmt.Errorf("invalidate cache before user deletion: %w", err)
	}

	// PostgreSQL and Redis do not share a transaction. Delete durable user state
	// only after session revocation and cache invalidation are known to succeed.
	// A database failure can therefore leave an existing user logged out, but a
	// known revocation failure is never reported as a successful user deletion.
	// A session created after the atomic Redis cleanup can remain stored, but a
	// successful user-row deletion makes it fail authoritative session validation.
	if err := us.userRepo.Delete(ctx, userID); err != nil {
		us.logger.Error("failed to delete user", "user_id", userID, "error", err)
		return err
	}

	us.logger.Info("user deleted successfully", "user_id", userID)
	return nil
}

func (us *UserUseCase) UpdateUser(ctx context.Context, input UpdateInput) error {
	us.logger.Info("update user attempt", "target_id", input.UserID)
	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "user_id", input.UserID, "error", err)
		return err
	}

	usr := entity.User{
		ID:       input.UserID,
		Email:    input.Email,
		Password: hashedPassword,
		Status:   input.Status,
		Role:     input.Role,
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

	// The authorization cache is not invalidated here. Password changes do not
	// alter the role or status used by authorization, and cached timestamps are
	// not consulted for authorization decisions.
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
	if us.passwordChangeCleanupMetric != nil {
		us.passwordChangeCleanupMetric.RecordPasswordChangeCleanupFailure(stage)
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

	us.publisher.Publish(ctx, event.UserUpdatedEvent{
		UserID: usr.ID,
	})

	us.logger.Info("user role changed successfully", "user_id:", input.UserID)
	return nil
}

func (us *UserUseCase) ChangeStatus(ctx context.Context, input UpdateStatusInput) error {
	us.logger.Info("change status attempt", "target_id", input.UserID, "actor_id", input.ActorID)

	actorRole, err := us.userCache.GetUserRole(ctx, input.ActorID)
	if err != nil {
		us.logger.Error("change user status faild", "target_id", input.UserID, "actor_id", input.ActorID, "error", err)
		return err
	}

	userRole, err := us.userCache.GetUserRole(ctx, input.UserID)
	if err != nil {
		us.logger.Error("change user status faild", "target_id", input.UserID, "actor_id", input.ActorID, "error", err)
		return err
	}

	if !actorRole.CanModifyTargetRole(userRole) {
		us.logger.Error("user not permission to perform this action", "target_id", input.UserID, "actor_id", input.ActorID)
		return errors.ErrForbidden
	}

	usr := &entity.User{
		ID:     input.UserID,
		Status: input.Status,
	}
	if err := us.userRepo.Update(ctx, usr); err != nil {
		us.logger.Error("change user status faild", "user_id", input.UserID, "error", err)
		return err
	}

	us.publisher.Publish(ctx, event.UserUpdatedEvent{
		UserID:    usr.ID,
		UpdatedBy: input.ActorID,
	})

	us.logger.Info("user status changed successfully", "user_id", input.UserID)
	return nil
}
