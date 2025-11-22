package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mtextr/gopi/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

type UserUsecase struct {
	repo      UserRepository
	jwtSecret string
}

func NewUserUsecase(r UserRepository, secret string) *UserUsecase {
	return &UserUsecase{
		repo:      r,
		jwtSecret: secret,
	}
}

// HashFuncs
func (u *UserUsecase) HashPassword(password string) (string, error) {
	salted := password + u.jwtSecret
	bytes, err := bcrypt.GenerateFromPassword([]byte(salted), bcrypt.DefaultCost)
	return string(bytes), err
}

// func CheckPassword(hashedPassword, password string) error {
// 	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
// }

func (u *UserUsecase) RegisterUser(ctx context.Context, user *domain.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now().UTC()

	hashed, err := u.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashed

	return u.repo.Create(ctx, user)
}
