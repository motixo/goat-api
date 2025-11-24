package user

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

func (u *UserUsecase) Login(ctx context.Context, email, password string) (*entity.User, string, string, error) {
	user, err := u.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", "", err
	}
	if user == nil {
		return nil, "", "", errors.ErrNotFound
	}

	p := valueobject.PasswordFromHash(user.Password)
	if !p.Check(password) {
		return nil, "", "", errors.ErrUnauthorized
	}
	return user, "", "", nil
}
