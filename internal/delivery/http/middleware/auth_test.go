package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type stubJWTService struct {
	service.JWTService
	claims *valueobject.JWTClaims
	err    error
}

func (s *stubJWTService) ParseAndValidate(string) (*valueobject.JWTClaims, error) {
	return s.claims, s.err
}

type recordingSessionUseCase struct {
	session.UseCase
	valid   bool
	err     error
	checked bool
}

func (s *recordingSessionUseCase) IsJTIValid(context.Context, string) (bool, error) {
	s.checked = true
	return s.valid, s.err
}

type stubUserCacheService struct {
	service.UserCacheService
	status valueobject.UserStatus
	err    error
}

func (s *stubUserCacheService) GetUserStatus(context.Context, string) (valueobject.UserStatus, error) {
	return s.status, s.err
}

func TestAuthMiddlewareRequiredAllowsAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessions := &recordingSessionUseCase{valid: true}
	middleware := NewAuthMiddleware(
		&stubJWTService{claims: &valueobject.JWTClaims{
			UserID:    "user-1",
			SessionID: "session-1",
			TokenType: valueobject.TokenTypeAccess,
			JTI:       "access-jti",
		}},
		sessions,
		&stubUserCacheService{status: valueobject.StatusActive},
	)

	var gotUserID, gotSessionID string
	router := gin.New()
	router.GET("/protected", middleware.Required(), func(c *gin.Context) {
		gotUserID = c.GetString(string(UserIDKey))
		gotSessionID = c.GetString(string(SessionIDKey))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if !sessions.checked {
		t.Fatal("expected access-token JTI to be checked")
	}
	if gotUserID != "user-1" {
		t.Fatalf("user ID = %q, want %q", gotUserID, "user-1")
	}
	if gotSessionID != "session-1" {
		t.Fatalf("session ID = %q, want %q", gotSessionID, "session-1")
	}
}

func TestAuthMiddlewareRequiredRejectsRefreshTokenBeforeSessionLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessions := &recordingSessionUseCase{valid: true}
	middleware := NewAuthMiddleware(
		&stubJWTService{claims: &valueobject.JWTClaims{
			UserID:    "user-1",
			TokenType: valueobject.TokenTypeRefresh,
			JTI:       "refresh-jti",
		}},
		sessions,
		&stubUserCacheService{status: valueobject.StatusActive},
	)

	nextCalled := false
	router := gin.New()
	router.GET("/protected", middleware.Required(), func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer refresh-token")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Fatal("protected handler was called for a refresh token")
	}
	if sessions.checked {
		t.Fatal("refresh-token JTI was checked before token purpose was rejected")
	}
}
