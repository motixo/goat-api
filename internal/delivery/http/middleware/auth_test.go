package middleware

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
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

func TestAuthMiddlewareDelegatesTokenErrorsToCentralMapper(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
		wantDetail string
	}{
		{name: "expired", err: domainErrors.ErrTokenExpired, wantStatus: http.StatusUnauthorized, wantType: "/errors/unauthorized", wantDetail: "token has expired"},
		{name: "invalid", err: domainErrors.ErrTokenInvalid, wantStatus: http.StatusUnauthorized, wantType: "/errors/unauthorized", wantDetail: "invalid or malformed token"},
	}

	for _, test := range tests {
		for _, variant := range []struct {
			name string
			err  error
		}{
			{name: "direct", err: test.err},
			{name: "wrapped", err: fmt.Errorf("validate access token: %w", test.err)},
		} {
			t.Run(test.name+"/"+variant.name, func(t *testing.T) {
				sessions := &recordingSessionUseCase{valid: true}
				middleware := NewAuthMiddleware(
					&stubJWTService{err: variant.err},
					sessions,
					&stubUserCacheService{status: valueobject.StatusActive},
				)

				recorder := performRequiredMiddlewareRequest(t, middleware)
				if recorder.Code != test.wantStatus {
					t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
				}
				assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
					"type":     test.wantType,
					"title":    "Unauthorized",
					"status":   float64(test.wantStatus),
					"detail":   test.wantDetail,
					"instance": "/protected",
				})
				if sessions.checked {
					t.Fatal("session lookup occurred after token validation failed")
				}
			})
		}
	}
}

func TestAuthMiddlewareKeepsUnknownTokenFailuresInternal(t *testing.T) {
	internalErr := stdErrors.New("jwt key provider unavailable: secret details")
	middleware := NewAuthMiddleware(
		&stubJWTService{err: internalErr},
		&recordingSessionUseCase{valid: true},
		&stubUserCacheService{status: valueobject.StatusActive},
	)

	recorder := performRequiredMiddlewareRequest(t, middleware)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
		"type":     "/errors/internal",
		"title":    "Internal Server Error",
		"status":   float64(http.StatusInternalServerError),
		"detail":   "An unexpected error occurred.",
		"instance": "/protected",
	})
}

func performRequiredMiddlewareRequest(t *testing.T, middleware *AuthMiddleware) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", middleware.Required(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertMiddlewareProblem(t *testing.T, body []byte, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode problem JSON: %v; body = %s", err, body)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problem = %#v, want %#v", got, want)
	}
}
