package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/routes"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authentication"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func TestEveryProtectedRouteAppliesRateLimitAdmissionBeforeAuthentication(t *testing.T) {
	limiter := &recordingAdmissionRateLimiter{
		allowed:    false,
		retryAfter: time.Second,
		count:      1,
	}
	jwt := &recordingAdmissionJWTService{}
	dependencies := testServerDependencies(readinessCheckFunc(func(context.Context) error { return nil }))
	dependencies.JWTService = jwt
	dependencies.RateLimiter = limiter
	dependencies.Logger = discardAdmissionLogger{}

	server, err := NewServer(
		testServerConfig(GinModeRelease),
		dependencies,
		middleware.RateLimitConfig{
			ProtectedIP: middleware.RateLimit{Limit: 1, Window: time.Minute},
		},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	protectedRoutes := 0
	for _, route := range server.RouteClassifications() {
		if route.Class == routes.AuthorizationPublic {
			continue
		}
		protectedRoutes++

		path := concreteAdmissionPath(route.Path)
		request := httptest.NewRequest(route.Method, path, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer must-not-be-parsed")
		request.RemoteAddr = "198.51.100.25:43123"
		recorder := httptest.NewRecorder()
		server.engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusTooManyRequests {
			t.Errorf("%s %s status = %d, want %d", route.Method, route.Path, recorder.Code, http.StatusTooManyRequests)
		}
	}

	if got := jwt.parseCalls.Load(); got != 0 {
		t.Fatalf("JWT parse calls before rate-limit admission = %d, want 0", got)
	}
	if got := len(limiter.calls); got != protectedRoutes {
		t.Fatalf("rate-limit admission calls = %d, want one for each of %d protected routes", got, protectedRoutes)
	}
	for _, call := range limiter.calls {
		if call.actorType != "ip" || call.actorID != "198.51.100.25" {
			t.Errorf("rate-limit actor = %s:%s, want ip:198.51.100.25", call.actorType, call.actorID)
		}
		if call.resource == "" {
			t.Error("route-group admission received an empty Gin route pattern")
		}
	}
}

func TestEveryProtectedRouteAppliesVerifiedUserLimitBeforeAuthorization(t *testing.T) {
	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	limiter := &recordingAdmissionRateLimiter{
		allowedByActor: map[string]bool{
			"ip":   true,
			"user": false,
		},
		retryAfter: time.Second,
		count:      1,
	}
	jwt := &recordingAdmissionJWTService{claims: &valueobject.JWTClaims{
		UserID:            "user-1",
		SessionID:         "session-1",
		CredentialVersion: 1,
		Role:              valueobject.RoleClient,
		Permissions:       permissions,
		TokenType:         valueobject.TokenTypeAccess,
		JTI:               "access-jti",
	}}
	sessions := &recordingAdmissionSessionUseCase{valid: true}
	dependencies := testServerDependencies(readinessCheckFunc(func(context.Context) error { return nil }))
	dependencies.JWTService = jwt
	dependencies.SessionUseCase = sessions
	dependencies.RateLimiter = limiter
	dependencies.Logger = discardAdmissionLogger{}

	server, err := NewServer(
		testServerConfig(GinModeRelease),
		dependencies,
		middleware.RateLimitConfig{
			ProtectedIP: middleware.RateLimit{Limit: 300, Window: time.Minute},
			Private:     middleware.RateLimit{Limit: 60, Window: time.Minute},
		},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	protectedRoutes := 0
	for _, route := range server.RouteClassifications() {
		if route.Class == routes.AuthorizationPublic {
			continue
		}
		protectedRoutes++

		request := httptest.NewRequest(
			route.Method,
			concreteAdmissionPath(route.Path),
			strings.NewReader(`{}`),
		)
		request.Header.Set("Authorization", "Bearer verified-access-token")
		request.RemoteAddr = "198.51.100.25:43123"
		recorder := httptest.NewRecorder()
		server.engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusTooManyRequests {
			t.Errorf("%s %s status = %d, want %d", route.Method, route.Path, recorder.Code, http.StatusTooManyRequests)
		}
	}

	if got := jwt.parseCalls.Load(); got != int64(protectedRoutes) {
		t.Fatalf("JWT parse calls = %d, want %d", got, protectedRoutes)
	}
	if got := sessions.validateCalls.Load(); got != int64(protectedRoutes) {
		t.Fatalf("session validation calls = %d, want %d", got, protectedRoutes)
	}
	if got, want := len(limiter.calls), protectedRoutes*2; got != want {
		t.Fatalf("rate-limit calls = %d, want %d", got, want)
	}
	for index := 0; index < len(limiter.calls); index += 2 {
		ipCall := limiter.calls[index]
		userCall := limiter.calls[index+1]
		if ipCall.actorType != "ip" || ipCall.actorID != "198.51.100.25" {
			t.Errorf("outer actor = %s:%s, want ip:198.51.100.25", ipCall.actorType, ipCall.actorID)
		}
		if userCall.actorType != "user" || userCall.actorID != "user-1" {
			t.Errorf("inner actor = %s:%s, want user:user-1", userCall.actorType, userCall.actorID)
		}
		if ipCall.limit != 300 || ipCall.window != time.Minute {
			t.Errorf("outer limit = %d/%s, want 300/%s", ipCall.limit, ipCall.window, time.Minute)
		}
		if userCall.limit != 60 || userCall.window != time.Minute {
			t.Errorf("inner limit = %d/%s, want 60/%s", userCall.limit, userCall.window, time.Minute)
		}
		if ipCall.resource != userCall.resource {
			t.Errorf("rate-limit resources differ: IP=%q user=%q", ipCall.resource, userCall.resource)
		}
	}
}

func TestPublicAuthenticationRoutesStopBeforeApplicationWorkWhenAdmissionFails(t *testing.T) {
	internalErr := errors.New("private Redis rate-limit failure")
	limiter := &recordingAdmissionRateLimiter{err: internalErr}
	authenticationUseCase := &recordingAdmissionAuthenticationUseCase{}
	logger := &recordingAdmissionLogger{}
	dependencies := testServerDependencies(readinessCheckFunc(func(context.Context) error { return nil }))
	dependencies.AuthenticationUseCase = authenticationUseCase
	dependencies.RateLimiter = limiter
	dependencies.Logger = logger

	server, err := NewServer(
		testServerConfig(GinModeRelease),
		dependencies,
		middleware.RateLimitConfig{
			Auth:    middleware.RateLimit{Limit: 5, Window: time.Minute},
			Private: middleware.RateLimit{Limit: 60, Window: time.Minute},
		},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		path string
		body string
	}{
		{path: "/api/v1/auth/login", body: `{"email":"person@example.com","password":"Secure1!"}`},
		{path: "/api/v1/auth/signup", body: `{"email":"person@example.com","password":"Secure1!"}`},
		{path: "/api/v1/auth/refresh", body: `{"refresh_token":"refresh-token"}`},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept-Language", "en-US")
		request.RemoteAddr = "198.51.100.25:43123"
		server.engine.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("POST %s status = %d, want %d", test.path, recorder.Code, http.StatusInternalServerError)
		}
		if strings.Contains(recorder.Body.String(), internalErr.Error()) {
			t.Errorf("POST %s exposed rate-limit infrastructure error: %s", test.path, recorder.Body.String())
		}
	}

	if got := authenticationUseCase.calls.Load(); got != 0 {
		t.Fatalf("authentication use-case calls after failed admission = %d, want 0", got)
	}
	if got := len(limiter.calls); got != len(tests) {
		t.Fatalf("rate-limit admission calls = %d, want %d", got, len(tests))
	}
	if got := logger.errorCalls.Load(); got != int64(len(tests)) {
		t.Fatalf("rate-limit error log calls = %d, want %d", got, len(tests))
	}
}

func concreteAdmissionPath(path string) string {
	path = strings.ReplaceAll(path, ":id", "01J00000000000000000000000")
	return strings.ReplaceAll(path, ":role", "admin")
}

type admissionRateLimitCall struct {
	actorType string
	actorID   string
	resource  string
	limit     int
	window    time.Duration
}

type recordingAdmissionRateLimiter struct {
	allowed        bool
	allowedByActor map[string]bool
	retryAfter     time.Duration
	count          int64
	err            error
	calls          []admissionRateLimitCall
}

func (l *recordingAdmissionRateLimiter) Allow(
	_ context.Context,
	actorType string,
	actorID string,
	resource string,
	limit int,
	window time.Duration,
) (bool, time.Duration, int64, error) {
	l.calls = append(l.calls, admissionRateLimitCall{
		actorType: actorType,
		actorID:   actorID,
		resource:  resource,
		limit:     limit,
		window:    window,
	})
	allowed := l.allowed
	if l.allowedByActor != nil {
		allowed = l.allowedByActor[actorType]
	}
	return allowed, l.retryAfter, l.count, l.err
}

type recordingAdmissionJWTService struct {
	service.JWTService
	parseCalls atomic.Int64
	claims     *valueobject.JWTClaims
	err        error
}

func (s *recordingAdmissionJWTService) ParseAndValidate(string) (*valueobject.JWTClaims, error) {
	s.parseCalls.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	if s.claims != nil {
		return s.claims, nil
	}
	return nil, errors.New("JWT parsing reached before rate-limit admission")
}

type recordingAdmissionSessionUseCase struct {
	session.UseCase
	valid         bool
	validateCalls atomic.Int64
}

func (u *recordingAdmissionSessionUseCase) ValidateSession(
	context.Context,
	session.ValidateInput,
) (bool, error) {
	u.validateCalls.Add(1)
	return u.valid, nil
}

type recordingAdmissionAuthenticationUseCase struct {
	authentication.UseCase
	calls atomic.Int64
}

func (u *recordingAdmissionAuthenticationUseCase) Login(
	context.Context,
	authentication.LoginInput,
) (authentication.LoginOutput, error) {
	u.calls.Add(1)
	return authentication.LoginOutput{}, errors.New("login use case reached after failed admission")
}

func (u *recordingAdmissionAuthenticationUseCase) Signup(
	context.Context,
	authentication.RegisterInput,
) (authentication.UserOutput, error) {
	u.calls.Add(1)
	return authentication.UserOutput{}, errors.New("signup use case reached after failed admission")
}

func (u *recordingAdmissionAuthenticationUseCase) Refresh(
	context.Context,
	authentication.RefreshInput,
) (authentication.RefreshOutput, error) {
	u.calls.Add(1)
	return authentication.RefreshOutput{}, errors.New("refresh use case reached after failed admission")
}

type discardAdmissionLogger struct{}

func (discardAdmissionLogger) Info(string, ...any)  {}
func (discardAdmissionLogger) Error(string, ...any) {}
func (discardAdmissionLogger) Warn(string, ...any)  {}
func (discardAdmissionLogger) Debug(string, ...any) {}
func (discardAdmissionLogger) Panic(string, ...any) {}

type recordingAdmissionLogger struct {
	errorCalls atomic.Int64
}

func (*recordingAdmissionLogger) Info(string, ...any) {}
func (l *recordingAdmissionLogger) Error(string, ...any) {
	l.errorCalls.Add(1)
}
func (*recordingAdmissionLogger) Warn(string, ...any)  {}
func (*recordingAdmissionLogger) Debug(string, ...any) {}
func (*recordingAdmissionLogger) Panic(string, ...any) {}
