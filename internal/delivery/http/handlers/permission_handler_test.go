package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/permission"
)

func TestPermissionHandlerGetPermissionsUsesDefaultPaginationAndPreservesEnvelope(t *testing.T) {
	createdAt := time.Date(2026, time.July, 22, 8, 30, 0, 0, time.UTC)
	var gotOffset, gotLimit int
	usecase := &stubPermissionUseCase{
		getPermissions: func(_ context.Context, offset, limit int) ([]permission.PermissionOutput, int64, error) {
			gotOffset = offset
			gotLimit = limit
			return []permission.PermissionOutput{{
				ID:        "permission-1",
				Role:      "admin",
				Action:    "full_access",
				CreatedAt: createdAt,
			}}, 1, nil
		},
	}

	recorder := performPermissionRequest(t, http.MethodGet, "/permission", "", usecase, func(router *gin.Engine, handler *PermissionHandler) {
		router.GET("/permission", handler.GetPermissions)
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotOffset != 0 || gotLimit != 10 {
		t.Fatalf("pagination = offset %d, limit %d; want offset 0, limit 10", gotOffset, gotLimit)
	}

	assertJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"data": [{
				"id": "permission-1",
				"role": "admin",
				"action": "full_access",
				"created_at": "2026-07-22T08:30:00Z"
			}],
			"meta": {"page": 1, "limit": 10, "total": 1, "total_pages": 1}
		}
	}`)
}

func TestPermissionHandlerCreateMapsDeliveryRequestAndResponse(t *testing.T) {
	createdAt := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)
	var gotInput permission.CreateInput
	usecase := &stubPermissionUseCase{
		create: func(_ context.Context, input permission.CreateInput) (permission.PermissionOutput, error) {
			gotInput = input
			return permission.PermissionOutput{
				ID:        "permission-2",
				Role:      input.Role.String(),
				Action:    input.Action.String(),
				CreatedAt: createdAt,
			}, nil
		},
	}

	recorder := performPermissionRequest(t, http.MethodPost, "/permission", `{"role":"operator","action":"user:read"}`, usecase, func(router *gin.Engine, handler *PermissionHandler) {
		router.POST("/permission", handler.CreatePermissin)
	})

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	wantInput := permission.CreateInput{
		Role:   valueobject.RoleOperator,
		Action: valueobject.PermUserRead,
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("application input = %#v, want %#v", gotInput, wantInput)
	}

	assertJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"id": "permission-2",
			"role": "operator",
			"action": "user:read",
			"created_at": "2026-07-22T09:00:00Z"
		}
	}`)
}

func TestPermissionHandlerCreateRejectsUnknownPermission(t *testing.T) {
	called := false
	usecase := &stubPermissionUseCase{
		create: func(context.Context, permission.CreateInput) (permission.PermissionOutput, error) {
			called = true
			return permission.PermissionOutput{}, nil
		},
	}

	recorder := performPermissionRequest(t, http.MethodPost, "/permission", `{"role":"admin","action":"database:drop"}`, usecase, func(router *gin.Engine, handler *PermissionHandler) {
		router.POST("/permission", handler.CreatePermissin)
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if called {
		t.Fatal("use case was called for an unknown permission")
	}
	assertJSONEqual(t, recorder.Body.Bytes(), `{
		"type": "/errors/validation",
		"title": "Bad Request",
		"status": 400,
		"detail": "Invalid request payload",
		"instance": "/permission"
	}`)
}

func performPermissionRequest(
	t *testing.T,
	method string,
	path string,
	body string,
	usecase permission.UseCase,
	register func(*gin.Engine, *PermissionHandler),
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	register(router, NewPermissionHandler(usecase, discardHandlerLogger{}))

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertJSONEqual(t *testing.T, got []byte, want string) {
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

type stubPermissionUseCase struct {
	create               func(context.Context, permission.CreateInput) (permission.PermissionOutput, error)
	getPermissions       func(context.Context, int, int) ([]permission.PermissionOutput, int64, error)
	getPermissionsByRole func(context.Context, valueobject.UserRole) ([]permission.PermissionOutput, error)
	delete               func(context.Context, string) error
}

func (s *stubPermissionUseCase) Create(ctx context.Context, input permission.CreateInput) (permission.PermissionOutput, error) {
	return s.create(ctx, input)
}

func (s *stubPermissionUseCase) GetPermissions(ctx context.Context, offset, limit int) ([]permission.PermissionOutput, int64, error) {
	return s.getPermissions(ctx, offset, limit)
}

func (s *stubPermissionUseCase) GetPermissionsByRole(ctx context.Context, role valueobject.UserRole) ([]permission.PermissionOutput, error) {
	return s.getPermissionsByRole(ctx, role)
}

func (s *stubPermissionUseCase) Delete(ctx context.Context, permissionID string) error {
	return s.delete(ctx, permissionID)
}

type discardHandlerLogger struct{}

func (discardHandlerLogger) Info(string, ...any)  {}
func (discardHandlerLogger) Error(string, ...any) {}
func (discardHandlerLogger) Warn(string, ...any)  {}
func (discardHandlerLogger) Debug(string, ...any) {}
func (discardHandlerLogger) Panic(string, ...any) {}
