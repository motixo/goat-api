package user

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/repository/dto"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type UserUseCase struct {
	userRepo       repository.UserRepository
	passwordHasher service.PasswordHasher
	sessionRepo    repository.SessionRepository
	logger         logger.Logger
}

func NewUsecase(
	r repository.UserRepository,
	passwordHasher service.PasswordHasher,
	logger logger.Logger,
	sessionRepo repository.SessionRepository,
) UseCase {
	return &UserUseCase{
		userRepo:       r,
		passwordHasher: passwordHasher,
		sessionRepo:    sessionRepo,
		logger:         logger,
	}
}

func (us *UserUseCase) GetUser(ctx context.Context, userID string) (*UserResponse, error) {
	us.logger.Info("Fetching user by ID", "userID:", userID)
	user, err := us.userRepo.FindByID(ctx, userID)
	if err != nil {
		us.logger.Error("Failed to fetch user", "userID", userID, "error", err)
		return nil, err
	}
	response := &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Role:      user.Role.String(),
		Status:    user.Status.String(),
		CreatedAt: user.CreatedAt,
	}
	us.logger.Info("User fetched successfully", "userID:", userID)
	return response, nil
}

func (us *UserUseCase) GetUserslist(ctx context.Context, offset, limit int) ([]*UserResponse, int64, error) {
	us.logger.Info("Fetching users List")
	users, total, err := us.userRepo.List(ctx, offset, limit)
	if err != nil {
		us.logger.Error("Failed to fetch users List", "error", err)
		return nil, 0, err
	}

	response := make([]*UserResponse, 0, len(users))
	for _, usr := range users {
		r := &UserResponse{
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
		us.logger.Error("filed to delete user sessions", "userID:", userID)
		return nil
	}

	us.logger.Info("User deleted successfully", "TargetUserID:", userID)
	return nil
}

func (us *UserUseCase) UpdateUser(ctx context.Context, input UserUpdateInput) error {
	us.logger.Info("update user attempt", "UserID:", input.UserID)

	updateDTO := dto.UserUpdate{}

	if input.Email != nil {
		updateDTO.Email = input.Email
		us.logger.Info("email updated", "UserID:", input.UserID, "NewEmail:", *input.Email)
	}

	if input.Role != nil {
		updateDTO.Role = input.Role
		us.logger.Info("role updated", "UserID:", input.UserID, "NewRole:", *input.Role)
	}

	if input.Status != nil {
		updateDTO.Status = input.Status
		us.logger.Info("status updated", "UserID:", input.UserID, "NewStatus:", *input.Status)
	}

	if err := us.userRepo.Update(ctx, input.UserID, updateDTO); err != nil {
		us.logger.Error("user update failed", "UserID:", input.UserID)
		return err
	}

	us.logger.Info("user successfully updated", "UserID:", input.UserID)
	return nil
}

func (us *UserUseCase) ChangePassword(ctx context.Context, input UpdatePassInput) error {

	if input.OldPassword == input.NewPassword {
		us.logger.Error("passwords are same", "UserID:", input.UserID)
		return errors.ErrPasswordSameAsCurrent
	}

	updateDTO := dto.UserUpdate{}

	user, err := us.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		us.logger.Error("user lookup failed", "UserID:", input.UserID, "Error:", err)
		return errors.ErrUserNotFound
	}

	if !us.passwordHasher.Verify(ctx, input.OldPassword, user.Password) {
		return errors.ErrInvalidPassword
	}

	hashedPassword, err := us.passwordHasher.Hash(ctx, input.NewPassword)
	if err != nil {
		us.logger.Error("password hashing failed", "UserID:", input.UserID, "Error:", err)
		return err
	}

	updateDTO.Password = &hashedPassword
	if err := us.userRepo.Update(ctx, input.UserID, updateDTO); err != nil {
		us.logger.Error("user update failed", "UserID:", input.UserID, "Error:", err)
		return err
	}

	sessions, _, err := us.sessionRepo.ListByUser(ctx, user.ID, 0, 0)
	if err != nil {
		us.logger.Error("field to fetch user sessions", "UserID:", input.UserID, "Error:", err)
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
		us.logger.Error("filed to delete user sessions", "userID:", input.UserID, "Error:", err)
		return nil
	}

	us.logger.Info("password updated and sessions removed", "UserID:", input.UserID)
	return nil

}
