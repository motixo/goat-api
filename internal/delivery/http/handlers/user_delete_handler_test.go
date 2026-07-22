package handlers

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/user"
)

func TestUserHandlerDeletePreservesSuccessResponse(t *testing.T) {
	var gotUserID string
	usecase := &stubUserDeletionUseCase{
		deleteUser: func(_ context.Context, userID string) error {
			gotUserID = userID
			return nil
		},
	}

	recorder := performUserDeletionRequest(t, "/users/user-1", usecase)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotUserID != "user-1" {
		t.Fatalf("user ID = %q, want %q", gotUserID, "user-1")
	}
	assertUserDeletionJSONEqual(t, recorder.Body.Bytes(), `{"data":"Deleted"}`)
}

func TestUserHandlerDeleteMapsMissingUserToNotFound(t *testing.T) {
	usecase := &stubUserDeletionUseCase{
		deleteUser: func(context.Context, string) error {
			return domainErrors.ErrUserNotFound
		},
	}

	recorder := performUserDeletionRequest(t, "/users/missing-user", usecase)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	assertUserDeletionJSONEqual(t, recorder.Body.Bytes(), `{
		"type": "/errors/not-found",
		"title": "Not Found",
		"status": 404,
		"detail": "The requested resource was not found.",
		"instance": "/users/missing-user"
	}`)
}

func TestUserHandlerDeleteDoesNotReportSuccessWhenRevocationFails(t *testing.T) {
	revocationErr := stdErrors.New("session revocation failed")
	usecase := &stubUserDeletionUseCase{
		deleteUser: func(context.Context, string) error {
			return revocationErr
		},
	}

	recorder := performUserDeletionRequest(t, "/users/user-1", usecase)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	assertUserDeletionJSONEqual(t, recorder.Body.Bytes(), `{
		"type": "/errors/internal",
		"title": "Internal Server Error",
		"status": 500,
		"detail": "An unexpected error occurred.",
		"instance": "/users/user-1"
	}`)
}

func performUserDeletionRequest(t *testing.T, path string, usecase user.UseCase) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.DELETE("/users/:id", NewUserHandler(usecase, discardUserDeletionHandlerLogger{}).DeleteUser)

	request := httptest.NewRequest(http.MethodDelete, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type stubUserDeletionUseCase struct {
	user.UseCase
	deleteUser func(context.Context, string) error
}

func (s *stubUserDeletionUseCase) DeleteUser(ctx context.Context, userID string) error {
	return s.deleteUser(ctx, userID)
}

func assertUserDeletionJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode actual JSON: %v; body = %s", err, got)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON body = %s, want %s", got, want)
	}
}

type discardUserDeletionHandlerLogger struct{}

func (discardUserDeletionHandlerLogger) Info(string, ...any)  {}
func (discardUserDeletionHandlerLogger) Error(string, ...any) {}
func (discardUserDeletionHandlerLogger) Warn(string, ...any)  {}
func (discardUserDeletionHandlerLogger) Debug(string, ...any) {}
func (discardUserDeletionHandlerLogger) Panic(string, ...any) {}
