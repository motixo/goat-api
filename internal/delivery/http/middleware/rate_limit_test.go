package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

type discardRateLimitLogger struct{}

func (discardRateLimitLogger) Info(string, ...any)  {}
func (discardRateLimitLogger) Error(string, ...any) {}
func (discardRateLimitLogger) Warn(string, ...any)  {}
func (discardRateLimitLogger) Debug(string, ...any) {}
func (discardRateLimitLogger) Panic(string, ...any) {}
