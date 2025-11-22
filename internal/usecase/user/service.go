package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/mtextr/gopi/internal/domain"
)

type UserUsecase struct {
	// TODO: add repo
}

// NewUserUsecase constructor
func NewUserUsecase() *UserUsecase {
	return &UserUsecase{}
}

func (u *UserUsecase) Create(ctx context.Context, user *domain.User) error {
	user.ID = uuid.New().String()
	// TODO: insert to database
	return nil
}
