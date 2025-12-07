package user

import (
	"context"
)

func (us *UserUseCase) GetUserslist(ctx context.Context) ([]*UserResponse, error) {
	us.logger.Info("Fetching users List")
	users, err := us.userRepo.List(ctx)
	if err != nil {
		us.logger.Error("Failed to fetch users List", "error", err)
		return nil, err
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
	return response, nil
}
