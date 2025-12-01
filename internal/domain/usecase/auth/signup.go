package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mot0x0/goth-api/internal/domain/entity"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterOutput struct {
	User user.UserResponse `json:"user"`
}

func (a *AuthUseCase) Signup(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	a.logger.Info("signup attempt", "email", input.Email)
	hashedPassword, err := a.passwordHasher.Hash(ctx, input.Password)
	if err != nil {
		a.logger.Error("failed to hash password", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	rq := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword.Value(),
		Status:    valueobject.StatusActive,
		CreatedAt: time.Now().UTC(),
	}

	err = a.userRepo.Create(ctx, rq)
	if err != nil {
		a.logger.Error("failed to create user", "email", input.Email, "error", err)
		return RegisterOutput{}, err
	}

	a.logger.Info("user registered successfully", "userID", rq.ID, "email", rq.Email)
	return RegisterOutput{
		User: user.UserResponse{
			ID:        rq.ID,
			Email:     rq.Email,
			CreatedAt: rq.CreatedAt,
		},
	}, nil
}
