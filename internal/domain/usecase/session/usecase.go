package session

import (
	"github.com/motixo/goth-api/internal/domain/repository"
	"github.com/motixo/goth-api/internal/domain/service"
)

type SessionUseCase struct {
	sessionRepo repository.SessionRepository
	logger      service.Logger
}

func NewUsecase(
	r repository.SessionRepository,
	logger service.Logger,
	ulidGen service.IDGenerator,
) UseCase {
	return &SessionUseCase{
		sessionRepo: r,
		logger:      logger,
	}
}
