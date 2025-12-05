package user

import (
	"context"

	"github.com/motixo/goth-api/internal/domain/entity"
)

func (us *UserUseCase) GetUser(ctx context.Context, userID string) (*entity.User, error) {
	us.logger.Info("Fetching user by ID", "userID:", userID)
	user, err := us.userRepo.FindByID(ctx, userID)
	if err != nil {
		us.logger.Error("Failed to fetch user", "userID", userID, "error", err)
		return nil, err
	}
	us.logger.Info("User fetched successfully", "userID:", userID)
	return user, nil
}
