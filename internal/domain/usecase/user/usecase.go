package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserUseCase struct {
	userRepo       repository.UserRepository
	passwordHasher service.PasswordHasher
	userCache      service.UserCacheService
	sessionRepo    repository.SessionRepository
	logger         service.Logger
}

func NewUsecase(
	r repository.UserRepository,
	passwordHasher service.PasswordHasher,
	logger service.Logger,
	sessionRepo repository.SessionRepository,
	userCache service.UserCacheService,
) UseCase {
	return &UserUseCase{
		userRepo:       r,
		passwordHasher: passwordHasher,
		sessionRepo:    sessionRepo,
		userCache:      userCache,
		logger:         logger,
	}
}

func (us *UserUseCase) CreateUser(ctx context.Context, input CreateInput) (UserResponse, error) {

	us.logger.Info("create user attempt", "email", input.Email)
	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return UserResponse{}, err
	}

	usr := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword,
		Status:    input.Status,
		Role:      input.Role,
		CreatedAt: time.Now().UTC(),
	}

	err = us.userRepo.Create(ctx, usr)
	if err != nil {
		us.logger.Error("failed to create user", "email", input.Email, "error", err)
		return UserResponse{}, err
	}

	us.logger.Info("user created successfully", "userID", usr.ID, "email", usr.Email)
	return UserResponse{
		ID:        usr.ID,
		Email:     usr.Email,
		Role:      usr.Role.String(),
		Status:    usr.Status.String(),
		CreatedAt: usr.CreatedAt,
	}, nil
}

func (us *UserUseCase) GetUser(ctx context.Context, userID string) (UserResponse, error) {
	us.logger.Info("Fetching user by ID", "userID:", userID)
	user, err := us.userRepo.FindByID(ctx, userID)
	if err != nil {
		us.logger.Error("Failed to fetch user", "userID", userID, "error", err)
		return UserResponse{}, err
	}
	response := UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role.String(),
		Status:    user.Status.String(),
		CreatedAt: user.CreatedAt,
	}
	us.logger.Info("User fetched successfully", "userID:", userID)
	return response, nil
}

func (us *UserUseCase) GetUserslist(ctx context.Context, actorID string, input GetListInput) ([]UserResponse, int64, error) {
	us.logger.Info("Fetching users List")

	actorRole, err := us.userCache.GetUserRole(ctx, input.ActorID)
	allowedRoles := valueobject.VisibleRoles(actorRole)
	if err != nil {
		us.logger.Error("change user status faild", "target_id", input.ActorID, "error", err)
		return []UserResponse{}, 0, err
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
			return []UserResponse{}, 0, nil
		}
		input.Filter.Roles = effectiveRoles
	} else {
		input.Filter.Roles = allowedRoles
	}

	users, total, err := us.userRepo.List(ctx, input.Offset, input.Limit, input.Filter)
	if err != nil {
		us.logger.Error("Failed to fetch users List", "error", err)
		return []UserResponse{}, 0, err
	}

	response := make([]UserResponse, 0, len(users))
	for _, usr := range users {
		r := UserResponse{
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
	us.logger.Info("Attempting to delete user", "TargetUserID:", userID)
	if err := us.userRepo.Delete(ctx, userID); err != nil {
		us.logger.Error("Failed to delete user", "Error:", err)
		return err
	}

	sessions, _, err := us.sessionRepo.ListByUser(ctx, userID, 0, 0)
	if err != nil {
		us.logger.Error("field to fetch user sessions", "UserID:", userID)
		return nil
	}

	if len(sessions) == 0 {
		return nil
	}

	targets := make([]string, 0, len(sessions))
	for i := range sessions {
		targets[i] = sessions[i].ID
	}
	if err := us.sessionRepo.Delete(ctx, targets); err != nil {
		us.logger.Error("filed to delete user sessions", "user_id:", userID, "error", err)
		return nil
	}

	if err := us.userCache.ClearCache(ctx, userID); err != nil {
		us.logger.Error("clear user cache faild", "user_id", userID, "error", err)
	}

	us.logger.Info("User deleted successfully", "target_user_id:", userID)
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

	usr := &entity.User{
		ID:       user.ID,
		Password: hashedPassword,
	}
	if err := us.userRepo.Update(ctx, usr); err != nil {
		us.logger.Error("user update failed", "user_id", input.UserID, "error", err)
		return err
	}

	sessions, _, err := us.sessionRepo.ListByUser(ctx, user.ID, 0, 0)
	if err != nil {
		us.logger.Error("field to fetch user sessions", "user_id", input.UserID, "error", err)
		return nil
	}

	if len(sessions) == 0 {
		return nil
	}

	targets := make([]string, len(sessions))
	for i := range sessions {
		targets[i] = sessions[i].ID
	}
	if err := us.sessionRepo.Delete(ctx, targets); err != nil {
		us.logger.Error("filed to delete user sessions", "user_id", input.UserID, "error", err)
		return nil
	}

	us.logger.Info("password updated and sessions removed", "user_id", input.UserID)
	return nil

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
	if err := us.userCache.ClearCache(ctx, input.UserID); err != nil {
		us.logger.Error("clear user cache faild", "user_id", input.UserID, "error", err)
	}
	us.logger.Info("user role changed successfully", "UserID:", input.UserID)
	return nil
}

func (us *UserUseCase) ChangeStatus(ctx context.Context, input UpdateStatusInput) error {
	us.logger.Info("change status attempt", "user_id", input.UserID, "target_id", input.ActorID)

	actorRole, err := us.userCache.GetUserRole(ctx, input.ActorID)
	if err != nil {
		us.logger.Error("change user status faild", "user_id", input.UserID, "target_id", input.ActorID, "error", err)
		return err
	}

	userRole, err := us.userCache.GetUserRole(ctx, input.UserID)
	if err != nil {
		us.logger.Error("change user status faild", "user_id", input.UserID, "target_id", input.ActorID, "error", err)
		return err
	}

	if !actorRole.CanModifyTargetRole(userRole) {
		us.logger.Error("user not permission to perform this action", "user_id", input.UserID, "target_id", input.ActorID)
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
	if err := us.userCache.ClearCache(ctx, input.UserID); err != nil {
		us.logger.Error("clear user cache faild", "user_id", input.UserID, "error", err)
	}
	us.logger.Info("user status changed successfully", "user_id", input.UserID)
	return nil
}
