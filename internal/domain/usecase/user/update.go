package user

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository/dto"
)

func (us *UserUseCase) UpdateUser(ctx context.Context, input UserUpdateInput) error {
	us.logger.Info("update user attempt", "UserID:", input.UserID)

	user, err := us.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		us.logger.Error("user lookup failed", "UserID:", input.UserID)
		return errors.ErrUserNotFound
	}

	updateDTO := dto.UserUpdate{}

	if input.Password != nil && input.OldPassword != nil {
		if !us.passwordHasher.Verify(ctx, *input.OldPassword, user.Password) {
			return errors.ErrInvalidPassword
		}
		hashedPassword, err := us.passwordHasher.Hash(ctx, *input.Password)
		if err != nil {
			us.logger.Error("password hashing failed", "UserID:", input.UserID)
			return err
		}
		updateDTO.Password = &hashedPassword
		us.logger.Info("password updated", "UserID:", input.UserID)
	}

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
