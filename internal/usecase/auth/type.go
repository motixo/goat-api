package auth

import (
	"time"
)

type LoginInput struct {
	Email    string
	Password string
	IP       string
	Device   string
}

type UserOutput struct {
	ID        string
	Email     string
	Role      string
	Status    string
	CreatedAt time.Time
}

type LoginOutput struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	User                  UserOutput
}

type RefreshInput struct {
	RefreshToken string
	IP           string
	Device       string
}

type RefreshOutput struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type RegisterInput struct {
	Email    string
	Password string
}

type AccessTTL time.Duration
type RefreshTTL time.Duration
type SessionTTL time.Duration
