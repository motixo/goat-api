package repositories

import "github.com/mot0x0/gopi/internal/domain/entity"

type SessionRepository interface {
	Create(session *entity.Session) error
	FindByToken(token string) (*entity.Session, error)
	FindByUserID(userID string) (*entity.Session, error)
	Delete(token string) error
	DeleteByUserID(userID string) error
}
