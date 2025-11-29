package session

import (
	"github.com/mot0x0/goth-api/internal/domain/service"
)

type SessionUseCase struct {
	sessionRepo Repository
	ulidGen     *service.ULIDGenerator
}

func NewUsecase(r Repository, ulidGen *service.ULIDGenerator) UseCase {
	return &SessionUseCase{
		sessionRepo: r,
		ulidGen:     ulidGen,
	}
}
