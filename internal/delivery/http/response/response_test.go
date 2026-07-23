package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func TestMapErrorPreservesPublicProblemContracts(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		typ    string
		title  string
		detail string
	}{
		{name: "bad request", err: domainErrors.ErrBadRequest, status: http.StatusBadRequest, typ: "/errors/internal", title: "Bad Request", detail: "An error occurred while processing your request."},
		{name: "invalid input", err: domainErrors.ErrInvalidInput, status: http.StatusBadRequest, typ: "/errors/internal", title: "Bad Request", detail: "An error occurred while processing your request."},
		{name: "password too short", err: domainErrors.ErrPasswordTooShort, status: http.StatusBadRequest, typ: "/errors/invalid-password", title: "Bad Request", detail: "Password must be at least 8 characters long."},
		{name: "password too long", err: domainErrors.ErrPasswordTooLong, status: http.StatusBadRequest, typ: "/errors/invalid-password", title: "Bad Request", detail: "Password must not exceed 72 characters."},
		{name: "password policy", err: domainErrors.ErrPasswordPolicyViolation, status: http.StatusBadRequest, typ: "/errors/invalid-password", title: "Bad Request", detail: "Password must contain uppercase, lowercase, number, and special character."},
		{name: "invalid current password", err: domainErrors.ErrInvalidPassword, status: http.StatusBadRequest, typ: "/errors/validation", title: "Bad Request", detail: "current password is incorrect"},
		{name: "invalid session selection", err: session.ErrInvalidSessionSelection, status: http.StatusBadRequest, typ: "/errors/validation", title: "Bad Request", detail: "Invalid request payload"},
		{name: "unauthorized", err: domainErrors.ErrUnauthorized, status: http.StatusUnauthorized, typ: "/errors/internal", title: "Unauthorized", detail: "An error occurred while processing your request."},
		{name: "token expired", err: domainErrors.ErrTokenExpired, status: http.StatusUnauthorized, typ: "/errors/unauthorized", title: "Unauthorized", detail: "token has expired"},
		{name: "token invalid", err: domainErrors.ErrTokenInvalid, status: http.StatusUnauthorized, typ: "/errors/unauthorized", title: "Unauthorized", detail: "invalid or malformed token"},
		{name: "current session invalid", err: auth.NewCurrentSessionInvalidError(domainErrors.ErrNotFound), status: http.StatusUnauthorized, typ: "/errors/unauthorized", title: "Unauthorized", detail: "not found"},
		{name: "invalid credentials", err: domainErrors.ErrInvalidCredentials, status: http.StatusUnauthorized, typ: "/errors/internal", title: "Unauthorized", detail: "Invalid email or password."},
		{name: "forbidden", err: domainErrors.ErrForbidden, status: http.StatusForbidden, typ: "/errors/internal", title: "Forbidden", detail: "An error occurred while processing your request."},
		{name: "account suspended", err: domainErrors.ErrAccountSuspended, status: http.StatusForbidden, typ: "/errors/internal", title: "Forbidden", detail: "Your account has been suspended. Please contact support."},
		{name: "not found", err: domainErrors.ErrNotFound, status: http.StatusNotFound, typ: "/errors/not-found", title: "Not Found", detail: "The requested resource was not found."},
		{name: "user not found", err: domainErrors.ErrUserNotFound, status: http.StatusNotFound, typ: "/errors/not-found", title: "Not Found", detail: "The requested resource was not found."},
		{name: "permission not found", err: domainErrors.ErrPermissionNotFound, status: http.StatusNotFound, typ: "/errors/not-found", title: "Not Found", detail: "The requested resource was not found."},
		{name: "conflict", err: domainErrors.ErrConflict, status: http.StatusConflict, typ: "/errors/conflict", title: "Conflict", detail: "The request conflicts with current state."},
		{name: "email exists", err: domainErrors.ErrEmailAlreadyExists, status: http.StatusConflict, typ: "/errors/email-already-exists", title: "Conflict", detail: "This email is already registered."},
		{name: "permission exists", err: domainErrors.ErrPermissionAlreadyExists, status: http.StatusConflict, typ: "/errors/conflict", title: "Conflict", detail: "The request conflicts with current state."},
		{name: "password unchanged", err: domainErrors.ErrPasswordSameAsCurrent, status: http.StatusConflict, typ: "/errors/internal", title: "Conflict", detail: "Passwords can't be same"},
		{name: "rate limited", err: domainErrors.ErrRateLimitExceeded, status: http.StatusTooManyRequests, typ: "/errors/internal", title: "Too Many Requests", detail: "An error occurred while processing your request."},
		{name: "internal", err: domainErrors.ErrInternal, status: http.StatusInternalServerError, typ: "/errors/internal", title: "Internal Server Error", detail: "An unexpected error occurred."},
		{name: "inactive user", err: domainErrors.ErrUserInactive, status: http.StatusInternalServerError, typ: "/errors/internal", title: "Internal Server Error", detail: "An unexpected error occurred."},
		{name: "password hashing failed", err: domainErrors.ErrPasswordHashingFailed, status: http.StatusInternalServerError, typ: "/errors/internal", title: "Internal Server Error", detail: "An unexpected error occurred."},
		{name: "unknown", err: errors.New("postgres connection details"), status: http.StatusInternalServerError, typ: "/errors/internal", title: "Internal Server Error", detail: "An unexpected error occurred."},
	}

	for _, test := range tests {
		for _, variant := range []struct {
			name string
			err  error
		}{
			{name: "direct", err: test.err},
			{name: "wrapped", err: fmt.Errorf("use case context: %w", test.err)},
		} {
			t.Run(test.name+"/"+variant.name, func(t *testing.T) {
				wantProblem := Problem{
					Type:   test.typ,
					Title:  test.title,
					Status: test.status,
					Detail: test.detail,
				}
				if got := MapError(variant.err); !reflect.DeepEqual(got, wantProblem) {
					t.Fatalf("MapError() = %#v, want %#v", got, wantProblem)
				}

				recorder := renderMappedError(t, variant.err)
				if recorder.Code != test.status {
					t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.status, recorder.Body.String())
				}
				assertProblemJSON(t, recorder.Body.Bytes(), map[string]any{
					"type":     test.typ,
					"title":    test.title,
					"status":   float64(test.status),
					"detail":   test.detail,
					"instance": "/resource",
				})
			})
		}
	}
}

func renderMappedError(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/resource", func(c *gin.Context) {
		WriteProblem(c, MapError(err))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/resource", nil))
	return recorder
}

func assertProblemJSON(t *testing.T, body []byte, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode problem JSON: %v; body = %s", err, body)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problem = %#v, want %#v", got, want)
	}
}
