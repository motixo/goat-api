package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RegisterOutput struct {
	User UserResponse `json:"user"`
}

type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	GetByID(ctx context.Context, id string) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
}

type UserUsecase struct {
	userRepo UserRepository
}

func NewUserUsecase(r UserRepository) UserUseCase {
	return &UserUsecase{
		userRepo: r,
	}
}

func (u *UserUsecase) Register(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	hashedPassword, err := valueobject.NewPassword(input.Password)
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

	err = u.userRepo.Create(ctx, rq)
	if err != nil {
		return RegisterOutput{}, err
	}

	return RegisterOutput{
		User: UserResponse{
			ID:        rq.ID,
			Email:     rq.Email,
			CreatedAt: rq.CreatedAt,
		},
	}, nil
}

func (u *UserUsecase) ValidateToken(ctx context.Context, token string) (string, error) {
	// TODO: implement
	return "", nil
}
