package repositories

import "github.com/mot0x0/gopi/internal/domain/entity"

type UserRepository interface {
	Create(user *entity.User) error
	FindByID(id string) (*entity.User, error)
	FindByEmail(email string) (*entity.User, error)
	Update(user *entity.User) error
	Delete(id string) error
}
