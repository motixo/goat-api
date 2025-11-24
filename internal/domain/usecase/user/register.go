package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

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

func (u *UserUsecase) Register(ctx context.Context, email string, password string) (*entity.User, error) {
	hashedPassword, err := valueobject.NewPassword(password)
	if err != nil {
		return nil, err
	}

	rq := &entity.User{
		ID:        uuid.New().String(),
		Email:     email,
		Password:  hashedPassword.Value(),
		Status:    valueobject.StatusInactive,
		CreatedAt: time.Now().UTC(),
	}

	err = u.userRepo.Create(ctx, rq)
	if err != nil {
		return nil, err
	}
	return rq, nil
}

func (u *UserUsecase) ValidateToken(ctx context.Context, token string) (string, error) {
	// TODO: implement
	return "", nil
}
