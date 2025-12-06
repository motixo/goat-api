package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func (us *AuthUseCase) Signup(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	us.logger.Info("signup attempt", "email", input.Email)
	hashedPassword, err := us.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		us.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	rq := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword,
		Status:    valueobject.StatusActive,
		Role:      valueobject.RoleClient,
		CreatedAt: time.Now().UTC(),
	}

	err = us.userRepo.Create(ctx, rq)
	if err != nil {
		us.logger.Error("failed to create user", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	us.logger.Info("user registered successfully", "userID", rq.ID, "email", rq.Email)
	return RegisterOutput{
		User: user.UserResponse{
			ID:        rq.ID,
			Email:     rq.Email,
			Role:      rq.Role.String(),
			CreatedAt: rq.CreatedAt,
		},
	}, nil
}
