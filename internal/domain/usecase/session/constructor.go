package session

import (
	"github.com/mot0x0/gopi/internal/domain/repository"
)

type SessionUsecase struct {
	sessionRepo repository.SessionRepository
}

func NewSessionUsecase(r repository.SessionRepository) SessionUseCase {
	return &SessionUsecase{
		sessionRepo: r,
	}
}
