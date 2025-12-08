package auth

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

type AuthUseCase struct {
	userRepo       repository.UserRepository
	sessionUC      session.UseCase
	ulidGen        service.IDGenerator
	passwordHasher service.PasswordHasher
	jwtService     service.JWTService
	logger         logger.Logger
	accessTTL      time.Duration
	refreshTTL     time.Duration
	sessionTTL     time.Duration
}

func NewUsecase(
	userRepo repository.UserRepository,
	sessionUC session.UseCase,
	passwordHasher service.PasswordHasher,
	jwtService service.JWTService,
	ulidGen service.IDGenerator,
	logger logger.Logger,
	accessTTL AccessTTL,
	refreshTTL RefreshTTL,
	sessionTTL SessionTTL,

) UseCase {
	return &AuthUseCase{
		userRepo:       userRepo,
		sessionUC:      sessionUC,
		passwordHasher: passwordHasher,
		jwtService:     jwtService,
		logger:         logger,
		ulidGen:        ulidGen,
		accessTTL:      time.Duration(accessTTL),
		refreshTTL:     time.Duration(refreshTTL),
		sessionTTL:     time.Duration(sessionTTL),
	}
}
