package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterOutput struct {
	User user.UserResponse `json:"user"`
}

func (a *AuthUseCase) Signup(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	hashedPassword, err := a.passwordService.Hash(ctx, input.Password)
	if err != nil {
		return RegisterOutput{}, err
	}

	rq := &entity.User{
		ID:        uuid.New().String(),
		Email:     input.Email,
		Password:  hashedPassword.Value(),
		Status:    valueobject.StatusInactive,
		CreatedAt: time.Now().UTC(),
	}

	err = a.userRepo.Create(ctx, rq)
	if err != nil {
		return RegisterOutput{}, err
	}

	return RegisterOutput{
		User: user.UserResponse{
			ID:        rq.ID,
			Email:     rq.Email,
			CreatedAt: rq.CreatedAt,
		},
	}, nil
}
