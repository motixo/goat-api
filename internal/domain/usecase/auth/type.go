package auth

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/usecase/user"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	IP       string `json:"-"`
	Device   string `json:"-"`
}

type LoginOutput struct {
	AccessToken           string            `json:"access_token"`
	AccessTokenExpiresAt  time.Time         `json:"access_token_expires_at"`
	RefreshToken          string            `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time         `json:"refresh_token_expires_at"`
	User                  user.UserResponse `json:"user"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
	IP           string `json:"-"`
	Device       string `json:"-"`
}

type RefreshOutput struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AccessTTL time.Duration
type RefreshTTL time.Duration
type SessionTTL time.Duration
