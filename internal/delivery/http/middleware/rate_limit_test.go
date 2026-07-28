package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

type rejectingRateLimiter struct{}

func (rejectingRateLimiter) Allow(
	context.Context,
	string,
	string,
	string,
	int,
	time.Duration,
) (bool, time.Duration, int64, error) {
	return false, 1500 * time.Millisecond, 5, nil
}

type failingRateLimiter struct {
	err error
}

func (l failingRateLimiter) Allow(
	context.Context,
	string,
	string,
	string,
	int,
	time.Duration,
) (bool, time.Duration, int64, error) {
	return false, 0, 0, l.err
}

func TestRateLimitMiddlewareUsesLocalizedParameterizedProblem(t *testing.T) {
	tests := []struct {
		name     string
		language string
		resolved string
		title    string
		detail   string
	}{
		{
			name:     "English",
			language: "en-US",
			resolved: "en",
			title:    "Too Many Requests",
			detail:   "Limit exceeded. Please try again in 2s.",
		},
		{
			name:     "Persian",
			language: "fa-IR",
			resolved: "fa",
			title:    "تعداد درخواست\u200cها بیش از حد مجاز است",
			detail:   "تعداد درخواست\u200cهای شما بیش از حد مجاز است. لطفاً پس از 2s دوباره تلاش کنید.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			middleware := NewRateLimitMiddleware(rejectingRateLimiter{}, discardRateLimitLogger{})
			nextCalled := false
			router := gin.New()
			router.GET("/limited", middleware.Handler(RateLimit{
				Limit:  5,
				Window: time.Minute,
			}), func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/limited", nil)
			request.Header.Set("Accept-Language", test.language)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if nextCalled {
				t.Fatal("next handler was called after rate limiting")
			}
			if recorder.Header().Get("Retry-After") != "1" {
				t.Fatalf("Retry-After = %q, want %q", recorder.Header().Get("Retry-After"), "1")
			}
			if got, want := recorder.Header().Get("Content-Type"), "application/problem+json"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got := recorder.Header().Get("Content-Language"); got != test.resolved {
				t.Fatalf("Content-Language = %q, want %q", got, test.resolved)
			}
			if got, want := recorder.Header().Get("Vary"), "Accept-Language"; got != want {
				t.Fatalf("Vary = %q, want %q", got, want)
			}
			assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
				"type":     "/errors/rate-limit",
				"title":    test.title,
				"status":   float64(http.StatusTooManyRequests),
				"detail":   test.detail,
				"instance": "/limited",
				"metadata": map[string]any{
					"limit":       float64(5),
					"window":      "1m0s",
					"retry_after": "2s",
				},
			})
		})
	}
}

func TestRateLimitMiddlewareFailsClosedWithLocalizedSafeProblem(t *testing.T) {
	tests := []struct {
		name     string
		language string
		resolved string
		title    string
		detail   string
	}{
		{
			name:     "English",
			language: "en-US",
			resolved: "en",
			title:    "Internal Server Error",
			detail:   "An unexpected error occurred.",
		},
		{
			name:     "Persian",
			language: "fa-IR",
			resolved: "fa",
			title:    "خطای سرور",
			detail:   "مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			internalErr := errors.New("redis://rate-user:secret@private-host unavailable")
			logger := &recordingRateLimitLogger{}
			middleware := NewRateLimitMiddleware(
				failingRateLimiter{err: internalErr},
				logger,
			)
			nextCalled := false
			router := gin.New()
			router.GET("/limited", middleware.Handler(RateLimit{
				Limit:  5,
				Window: time.Minute,
			}), func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/limited", nil)
			request.Header.Set("Accept-Language", test.language)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if nextCalled {
				t.Fatal("next handler was called after rate-limit enforcement failed")
			}
			if got := logger.errorCalls.Load(); got != 1 {
				t.Fatalf("rate-limit error log calls = %d, want 1", got)
			}
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
			if got, want := recorder.Header().Get("Content-Type"), "application/problem+json"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got := recorder.Header().Get("Content-Language"); got != test.resolved {
				t.Fatalf("Content-Language = %q, want %q", got, test.resolved)
			}
			if strings.Contains(recorder.Body.String(), internalErr.Error()) ||
				strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("Problem response exposed rate-limit infrastructure details: %s", recorder.Body.String())
			}
			assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
				"type":     "/errors/internal",
				"title":    test.title,
				"status":   float64(http.StatusInternalServerError),
				"detail":   test.detail,
				"instance": "/limited",
			})
		})
	}
}

func TestAuthenticatedRateLimitUsesVerifiedPrincipalInsteadOfIPOrUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingActorRateLimiter{}
	middleware := NewRateLimitMiddleware(limiter, discardRateLimitLogger{})
	principals := map[string]authorization.Principal{
		"first":  newRateLimitTestPrincipal(t, "user-1"),
		"second": newRateLimitTestPrincipal(t, "user-2"),
	}

	router := gin.New()
	router.GET(
		"/limited",
		func(c *gin.Context) {
			SetPrincipal(c, principals[c.GetHeader("X-Test-Principal")])
		},
		middleware.Authenticated(RateLimit{Limit: 60, Window: time.Minute}),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	for _, requestValues := range []struct {
		principal string
		userAgent string
	}{
		{principal: "first", userAgent: "same-device/1.0"},
		{principal: "second", userAgent: "same-device/1.0"},
	} {
		request := httptest.NewRequest(http.MethodGet, "/limited", nil)
		request.RemoteAddr = "198.51.100.25:43123"
		request.Header.Set("User-Agent", requestValues.userAgent)
		request.Header.Set("X-Test-Principal", requestValues.principal)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
	}

	if len(limiter.calls) != 2 {
		t.Fatalf("rate-limit calls = %d, want 2", len(limiter.calls))
	}
	for index, wantUserID := range []string{"user-1", "user-2"} {
		call := limiter.calls[index]
		if call.actorType != "user" || call.actorID != wantUserID {
			t.Errorf("call %d actor = %s:%s, want user:%s", index, call.actorType, call.actorID, wantUserID)
		}
		if call.resource != "/limited" {
			t.Errorf("call %d resource = %q, want %q", index, call.resource, "/limited")
		}
	}
}

func TestAuthenticatedRateLimitFailsClosedWithoutVerifiedPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingActorRateLimiter{}
	middleware := NewRateLimitMiddleware(limiter, discardRateLimitLogger{})
	nextCalled := false
	router := gin.New()
	router.GET(
		"/limited",
		middleware.Authenticated(RateLimit{Limit: 60, Window: time.Minute}),
		func(c *gin.Context) {
			nextCalled = true
			c.Status(http.StatusNoContent)
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/limited", nil)
	request.Header.Set("Accept-Language", "en-US")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if nextCalled {
		t.Fatal("next handler was called without a verified principal")
	}
	if len(limiter.calls) != 0 {
		t.Fatalf("rate-limit calls = %d, want 0", len(limiter.calls))
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
		"type":     "/errors/unauthorized",
		"title":    "Unauthorized",
		"status":   float64(http.StatusUnauthorized),
		"detail":   "authentication required",
		"instance": "/limited",
	})
}

type actorRateLimitCall struct {
	actorType string
	actorID   string
	resource  string
}

type recordingActorRateLimiter struct {
	calls []actorRateLimitCall
}

func (l *recordingActorRateLimiter) Allow(
	_ context.Context,
	actorType string,
	actorID string,
	resource string,
	_ int,
	_ time.Duration,
) (bool, time.Duration, int64, error) {
	l.calls = append(l.calls, actorRateLimitCall{
		actorType: actorType,
		actorID:   actorID,
		resource:  resource,
	})
	return true, time.Minute, 1, nil
}

func newRateLimitTestPrincipal(t *testing.T, userID string) authorization.Principal {
	t.Helper()
	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	principal, err := authorization.NewPrincipal(
		userID,
		"session-1",
		1,
		valueobject.RoleClient,
		permissions,
	)
	if err != nil {
		t.Fatalf("build principal: %v", err)
	}
	return principal
}

type discardRateLimitLogger struct{}

func (discardRateLimitLogger) Info(string, ...any)  {}
func (discardRateLimitLogger) Error(string, ...any) {}
func (discardRateLimitLogger) Warn(string, ...any)  {}
func (discardRateLimitLogger) Debug(string, ...any) {}
func (discardRateLimitLogger) Panic(string, ...any) {}

type recordingRateLimitLogger struct {
	errorCalls atomic.Int64
}

func (*recordingRateLimitLogger) Info(string, ...any) {}
func (l *recordingRateLimitLogger) Error(string, ...any) {
	l.errorCalls.Add(1)
}
func (*recordingRateLimitLogger) Warn(string, ...any)  {}
func (*recordingRateLimitLogger) Debug(string, ...any) {}
func (*recordingRateLimitLogger) Panic(string, ...any) {}
