package handlers

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func TestSessionHandlerDeleteUsesAuthenticatedPrincipal(t *testing.T) {
	var gotInput session.DeleteSessionsInput
	usecase := &stubSessionDeletionUseCase{
		deleteSessions: func(_ context.Context, input session.DeleteSessionsInput) error {
			gotInput = input
			return nil
		},
	}

	recorder := performSessionDeletionRequest(t, `{
		"UserID":"client-supplied-user",
		"CurrentSession":"01ARZ3NDEKTSV4RRFFQ69G5FAX",
		"session_ids":["01ARZ3NDEKTSV4RRFFQ69G5FAW"]
	}`, usecase)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotInput.UserID != "authenticated-user" {
		t.Fatalf("application user ID = %q, want authenticated principal", gotInput.UserID)
	}
	if gotInput.CurrentSession != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("current session = %q, want authenticated session", gotInput.CurrentSession)
	}
	assertSessionHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"Revoked"}`)
}

func TestSessionHandlerDeleteOthersPreservesSuccessContract(t *testing.T) {
	var gotInput session.DeleteSessionsInput
	usecase := &stubSessionDeletionUseCase{
		deleteSessions: func(_ context.Context, input session.DeleteSessionsInput) error {
			gotInput = input
			return nil
		},
	}

	recorder := performSessionDeletionRequest(t, `{"others":true}`, usecase)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !gotInput.RemoveOthers {
		t.Fatal("remove-others flag was not forwarded")
	}
	if gotInput.UserID != "authenticated-user" {
		t.Fatalf("application user ID = %q, want authenticated principal", gotInput.UserID)
	}
	if gotInput.CurrentSession != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("current session = %q, want authenticated session", gotInput.CurrentSession)
	}
	assertSessionHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"Revoked"}`)
}

func TestSessionHandlerDeleteHidesMissingAndForeignOwnership(t *testing.T) {
	usecase := &stubSessionDeletionUseCase{
		deleteSessions: func(context.Context, session.DeleteSessionsInput) error {
			return domainErrors.ErrNotFound
		},
	}

	recorder := performSessionDeletionRequest(t, `{"session_ids":["01ARZ3NDEKTSV4RRFFQ69G5FAX"]}`, usecase)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	assertSessionHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"type":"/errors/not-found",
		"title":"Not Found",
		"status":404,
		"detail":"The requested resource was not found.",
		"instance":"/user/sessions"
	}`)
}

func TestSessionHandlerDeleteMapsMalformedSessionIDToBadRequest(t *testing.T) {
	usecase := &stubSessionDeletionUseCase{
		deleteSessions: func(context.Context, session.DeleteSessionsInput) error {
			return domainErrors.ErrInvalidInput
		},
	}

	recorder := performSessionDeletionRequest(t, `{"session_ids":["not-a-ulid"]}`, usecase)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	assertSessionHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"type":"/errors/validation",
		"title":"Bad Request",
		"status":400,
		"detail":"Invalid request payload",
		"instance":"/user/sessions"
	}`)
}

func TestSessionHandlerDeleteKeepsInfrastructureFailuresInternal(t *testing.T) {
	usecase := &stubSessionDeletionUseCase{
		deleteSessions: func(context.Context, session.DeleteSessionsInput) error {
			return stdErrors.New("redis unavailable")
		},
	}

	recorder := performSessionDeletionRequest(t, `{"session_ids":["01ARZ3NDEKTSV4RRFFQ69G5FAV"]}`, usecase)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
}

func performSessionDeletionRequest(t *testing.T, body string, usecase session.UseCase) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.DELETE("/user/sessions", func(c *gin.Context) {
		c.Set("user_id", "authenticated-user")
		c.Set("session_id", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
		c.Next()
	}, NewSessionHandler(usecase, discardSessionHandlerLogger{}).DeleteSessions)

	request := httptest.NewRequest(http.MethodDelete, "/user/sessions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertSessionHandlerJSONEqual(t *testing.T, got []byte, want string) {
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

type stubSessionDeletionUseCase struct {
	session.UseCase
	deleteSessions func(context.Context, session.DeleteSessionsInput) error
}

func (s *stubSessionDeletionUseCase) DeleteSessions(ctx context.Context, input session.DeleteSessionsInput) error {
	return s.deleteSessions(ctx, input)
}

type discardSessionHandlerLogger struct{}

func (discardSessionHandlerLogger) Info(string, ...any)  {}
func (discardSessionHandlerLogger) Error(string, ...any) {}
func (discardSessionHandlerLogger) Warn(string, ...any)  {}
func (discardSessionHandlerLogger) Debug(string, ...any) {}
func (discardSessionHandlerLogger) Panic(string, ...any) {}
