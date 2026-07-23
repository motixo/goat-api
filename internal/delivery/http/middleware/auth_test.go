package middleware

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
	valid         bool
	err           error
	validateInput session.ValidateInput
	checked       bool
}

func (s *recordingSessionUseCase) ValidateSession(
	_ context.Context,
	input session.ValidateInput,
) (bool, error) {
	s.checked = true
	s.validateInput = input
	return s.valid, s.err
}

type stubUserCacheService struct {
	service.UserCacheService
	status valueobject.UserStatus
	err    error
	calls  int
}

func (s *stubUserCacheService) GetUserStatus(context.Context, string) (valueobject.UserStatus, error) {
	s.calls++
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
		t.Fatal("expected access-token session to be checked")
	}
	wantValidation := session.ValidateInput{
		UserID:    "user-1",
		SessionID: "session-1",
		JTI:       "access-jti",
	}
	if sessions.validateInput != wantValidation {
		t.Fatalf("session validation input = %#v, want %#v", sessions.validateInput, wantValidation)
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

func TestAuthMiddlewareUsesSharedLocalizedWriter(t *testing.T) {
	tests := []struct {
		name       string
		jwt        *stubJWTService
		token      string
		wantDetail string
	}{
		{
			name:       "delivery error",
			jwt:        &stubJWTService{},
			wantDetail: "اطلاعات ورود ارسال نشده یا معتبر نیست.",
		},
		{
			name:       "mapped semantic error",
			jwt:        &stubJWTService{err: domainErrors.ErrTokenExpired},
			token:      "access-token",
			wantDetail: "زمان ورود شما به پایان رسیده است. لطفاً دوباره وارد شوید.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware := NewAuthMiddleware(
				test.jwt,
				&recordingSessionUseCase{valid: true},
				&stubUserCacheService{status: valueobject.StatusActive},
			)
			recorder := performRequiredMiddlewareRequestWithLanguage(t, middleware, test.token, "fa-IR")

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
			if got, want := recorder.Header().Get("Content-Type"), "application/problem+json"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got, want := recorder.Header().Get("Content-Language"), "fa"; got != want {
				t.Fatalf("Content-Language = %q, want %q", got, want)
			}
			if got, want := recorder.Header().Get("Vary"), "Accept-Language"; got != want {
				t.Fatalf("Vary = %q, want %q", got, want)
			}
			assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
				"type":     "/errors/unauthorized",
				"title":    "لطفاً وارد حساب خود شوید",
				"status":   float64(http.StatusUnauthorized),
				"detail":   test.wantDetail,
				"instance": "/protected",
			})
		})
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

func TestAuthMiddlewareCredentialVersionRejectionPreservesLocalizedContract(t *testing.T) {
	tests := []struct {
		name       string
		locale     string
		wantTitle  string
		wantDetail string
	}{
		{
			name:       "English",
			locale:     "en-US",
			wantTitle:  "Unauthorized",
			wantDetail: "token has been revoked",
		},
		{
			name:       "Persian",
			locale:     "fa-IR",
			wantTitle:  "لطفاً وارد حساب خود شوید",
			wantDetail: "دسترسی شما لغو شده است. لطفاً دوباره وارد شوید.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sessions := &recordingSessionUseCase{valid: false}
			cache := &stubUserCacheService{status: valueobject.StatusActive}
			middleware := NewAuthMiddleware(
				&stubJWTService{claims: &valueobject.JWTClaims{
					UserID:    "user-1",
					SessionID: "session-1",
					TokenType: valueobject.TokenTypeAccess,
					JTI:       "access-jti",
				}},
				sessions,
				cache,
			)

			recorder := performRequiredMiddlewareRequestWithLanguage(
				t,
				middleware,
				"access-token",
				test.locale,
			)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
			assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
				"type":     "/errors/unauthorized",
				"title":    test.wantTitle,
				"status":   float64(http.StatusUnauthorized),
				"detail":   test.wantDetail,
				"instance": "/protected",
			})
			if cache.calls != 0 {
				t.Fatalf("authorization cache calls = %d, want 0 after credential-version rejection", cache.calls)
			}
		})
	}
}

func TestAuthMiddlewareAuthoritativeVersionFailureUsesSafeLocalizedInternalProblem(t *testing.T) {
	lookupErr := stdErrors.New("postgres connection secret must not be exposed")
	tests := []struct {
		locale     string
		wantTitle  string
		wantDetail string
	}{
		{
			locale:     "en",
			wantTitle:  "Internal Server Error",
			wantDetail: "An unexpected error occurred.",
		},
		{
			locale:     "fa",
			wantTitle:  "خطای سرور",
			wantDetail: "مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
		},
	}

	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			middleware := NewAuthMiddleware(
				&stubJWTService{claims: &valueobject.JWTClaims{
					UserID:    "user-1",
					SessionID: "session-1",
					TokenType: valueobject.TokenTypeAccess,
					JTI:       "access-jti",
				}},
				&recordingSessionUseCase{err: lookupErr},
				&stubUserCacheService{status: valueobject.StatusActive},
			)

			recorder := performRequiredMiddlewareRequestWithLanguage(
				t,
				middleware,
				"access-token",
				test.locale,
			)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
			assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
				"type":     "/errors/internal",
				"title":    test.wantTitle,
				"status":   float64(http.StatusInternalServerError),
				"detail":   test.wantDetail,
				"instance": "/protected",
			})
			if strings.Contains(recorder.Body.String(), lookupErr.Error()) {
				t.Fatalf("internal lookup detail leaked: %s", recorder.Body.String())
			}
		})
	}
}

func performRequiredMiddlewareRequest(t *testing.T, middleware *AuthMiddleware) *httptest.ResponseRecorder {
	t.Helper()
	return performRequiredMiddlewareRequestWithLanguage(t, middleware, "access-token", "")
}

func performRequiredMiddlewareRequestWithLanguage(
	t *testing.T,
	middleware *AuthMiddleware,
	token string,
	acceptLanguage string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", middleware.Required(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
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
