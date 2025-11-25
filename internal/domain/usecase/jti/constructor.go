package jti

import "github.com/mot0x0/gopi/internal/domain/repository"

type JTIUsecase struct {
	jtiRepo repository.JTIRepository
}

func NewJTIUsecase(r repository.JTIRepository) JTIUseCase {
	return &JTIUsecase{
		jtiRepo: r,
	}
}
