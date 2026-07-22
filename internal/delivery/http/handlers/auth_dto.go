package handlers

import (
	"time"

	"github.com/motixo/goat-api/internal/usecase/auth"
)

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authUserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"createdAt"`
}

type loginResponse struct {
	AccessToken           string           `json:"access_token"`
	AccessTokenExpiresAt  time.Time        `json:"access_token_expires_at"`
	RefreshToken          string           `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time        `json:"refresh_token_expires_at"`
	User                  authUserResponse `json:"user"`
}

type refreshResponse struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func newAuthUserResponse(output auth.UserOutput) authUserResponse {
	return authUserResponse{
		ID:        output.ID,
		Email:     output.Email,
		Role:      output.Role,
		Status:    output.Status,
		CreatedAt: output.CreatedAt,
	}
}

func newLoginResponse(output auth.LoginOutput) loginResponse {
	return loginResponse{
		AccessToken:           output.AccessToken,
		AccessTokenExpiresAt:  output.AccessTokenExpiresAt,
		RefreshToken:          output.RefreshToken,
		RefreshTokenExpiresAt: output.RefreshTokenExpiresAt,
		User:                  newAuthUserResponse(output.User),
	}
}

func newRefreshResponse(output auth.RefreshOutput) refreshResponse {
	return refreshResponse{
		AccessToken:           output.AccessToken,
		AccessTokenExpiresAt:  output.AccessTokenExpiresAt,
		RefreshToken:          output.RefreshToken,
		RefreshTokenExpiresAt: output.RefreshTokenExpiresAt,
	}
}
