package user

import (
	"context"
)

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
