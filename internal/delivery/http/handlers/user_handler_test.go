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
	"github.com/motixo/goat-api/internal/usecase/user"
)

const userHandlerTargetID = "11111111-1111-4111-8111-111111111111"

func TestUserHandlerCreatePreservesRequestAndResponseContract(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	var gotInput user.CreateInput
	usecase := &stubUserHandlerUseCase{
		createUser: func(_ context.Context, input user.CreateInput) (user.UserOutput, error) {
			gotInput = input
			return user.UserOutput{
				ID:        "user-1",
				Email:     "user@example.com",
				Role:      "operator",
				Status:    "active",
				CreatedAt: createdAt,
			}, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodPost, "/user", `{
		"email":"user@example.com",
		"password":"Password1!",
		"status":"active",
		"role":"operator"
	}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	wantInput := user.CreateInput{
		Email:    "user@example.com",
		Password: "Password1!",
		Status:   valueobject.StatusActive,
		Role:     valueobject.RoleOperator,
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("CreateUser input = %#v, want %#v", gotInput, wantInput)
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"id":"user-1",
			"email":"user@example.com",
			"Role":"operator",
			"Status":"active",
			"createdAt":"2026-07-23T08:30:00Z"
		}
	}`)
}

func TestUserHandlerCreatePreservesExistingEmailValidationBehavior(t *testing.T) {
	called := false
	usecase := &stubUserHandlerUseCase{
		createUser: func(_ context.Context, input user.CreateInput) (user.UserOutput, error) {
			called = input.Email == ""
			return user.UserOutput{}, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodPost, "/user", `{
		"password":"Password1!",
		"status":"active",
		"role":"client"
	}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if !called {
		t.Fatal("CreateUser was not called with the existing empty-email binding behavior")
	}
}

func TestUserHandlerGetPreservesResponseContract(t *testing.T) {
	createdAt := time.Date(2026, time.July, 22, 9, 15, 0, 0, time.UTC)
	var gotUserID string
	usecase := &stubUserHandlerUseCase{
		getUser: func(_ context.Context, userID string) (user.UserOutput, error) {
			gotUserID = userID
			return user.UserOutput{
				ID:        userID,
				Email:     "target@example.com",
				Role:      "client",
				Status:    "suspended",
				CreatedAt: createdAt,
			}, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodGet, "/user/target-user", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotUserID != "target-user" {
		t.Fatalf("GetUser ID = %q, want target-user", gotUserID)
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"id":"target-user",
			"email":"target@example.com",
			"Role":"client",
			"Status":"suspended",
			"createdAt":"2026-07-22T09:15:00Z"
		}
	}`)
}

func TestUserHandlerGetCurrentUserUsesAuthenticatedPrincipal(t *testing.T) {
	var gotUserID string
	usecase := &stubUserHandlerUseCase{
		getUser: func(_ context.Context, userID string) (user.UserOutput, error) {
			gotUserID = userID
			return user.UserOutput{ID: userID}, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodGet, "/user?user_id=client-controlled", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotUserID != "authenticated-user" {
		t.Fatalf("GetUser ID = %q, want authenticated-user", gotUserID)
	}
}

func TestUserHandlerListPreservesFiltersPaginationAndResponseContract(t *testing.T) {
	createdAt := time.Date(2026, time.July, 21, 7, 0, 0, 0, time.UTC)
	var gotInput user.GetListInput
	usecase := &stubUserHandlerUseCase{
		getUsersList: func(_ context.Context, input user.GetListInput) ([]user.UserOutput, int64, error) {
			gotInput = input
			return []user.UserOutput{{
				ID:        "user-2",
				Email:     "Example@domain.test",
				Role:      "admin",
				Status:    "active",
				CreatedAt: createdAt,
			}}, 101, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodGet,
		"/user/list?page=2&limit=101&role=client&role=admin&status=active&search=Example&user_id=client-controlled&sort=ignored", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	wantInput := user.GetListInput{
		ActorID: "authenticated-user",
		Filter: user.ListFilter{
			Roles:    []valueobject.UserRole{valueobject.RoleClient, valueobject.RoleAdmin},
			Statuses: []valueobject.UserStatus{valueobject.StatusActive},
			Search:   "Example",
		},
		Offset: 100,
		Limit:  100,
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("GetUserslist input = %#v, want %#v", gotInput, wantInput)
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"data": [{
				"id":"user-2",
				"email":"Example@domain.test",
				"Role":"admin",
				"Status":"active",
				"createdAt":"2026-07-21T07:00:00Z"
			}],
			"meta":{"page":2,"limit":100,"total":101,"total_pages":2}
		}
	}`)
}

func TestUserHandlerListPreservesInvalidFilterContract(t *testing.T) {
	var gotInput user.GetListInput
	usecase := &stubUserHandlerUseCase{
		getUsersList: func(_ context.Context, input user.GetListInput) ([]user.UserOutput, int64, error) {
			gotInput = input
			return []user.UserOutput{}, 0, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodGet, "/user/list?role=invalid&status=invalid", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	wantFilter := user.ListFilter{
		Roles:    []valueobject.UserRole{valueobject.RoleUnknown},
		Statuses: []valueobject.UserStatus{valueobject.StatusUnknown},
	}
	if !reflect.DeepEqual(gotInput.Filter, wantFilter) {
		t.Fatalf("filter = %#v, want %#v", gotInput.Filter, wantFilter)
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"data": [],
			"meta":{"page":1,"limit":10,"total":0,"total_pages":0}
		}
	}`)
}

func TestUserHandlerListPreservesMalformedPaginationProblem(t *testing.T) {
	usecase := &stubUserHandlerUseCase{
		getUsersList: func(context.Context, user.GetListInput) ([]user.UserOutput, int64, error) {
			t.Fatal("GetUserslist called for malformed pagination")
			return nil, 0, nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodGet, "/user/list?page=not-a-number", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"type":"/errors/validation",
		"title":"Bad Request",
		"status":400,
		"detail":"invalid pagination params",
		"instance":"/user/list"
	}`)
}

func TestUserHandlerUpdatePreservesRequestAndResponseContract(t *testing.T) {
	var gotInput user.UpdateInput
	usecase := &stubUserHandlerUseCase{
		updateUser: func(_ context.Context, input user.UpdateInput) error {
			gotInput = input
			return nil
		},
	}
	router := newUserHandlerTestRouter(usecase)

	recorder := performUserHandlerRequest(t, router, http.MethodPut, "/user/"+userHandlerTargetID, `{
		"UserID":"client-controlled",
		"email":"updated@example.com",
		"password":"NewPassword1!",
		"status":"inactive",
		"role":"operator"
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	wantInput := user.UpdateInput{
		UserID:   userHandlerTargetID,
		Email:    "updated@example.com",
		Password: "NewPassword1!",
		Status:   valueobject.StatusInactive,
		Role:     valueobject.RoleOperator,
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("UpdateUser input = %#v, want %#v", gotInput, wantInput)
	}
	assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"user updated successfully"}`)
}

func TestUserHandlerChangeCommandsPreserveMappingsAndContracts(t *testing.T) {
	t.Run("email", func(t *testing.T) {
		var gotInput user.UpdateEmailInput
		usecase := &stubUserHandlerUseCase{
			changeEmail: func(_ context.Context, input user.UpdateEmailInput) error {
				gotInput = input
				return nil
			},
		}
		router := newUserHandlerTestRouter(usecase)
		recorder := performUserHandlerRequest(t, router, http.MethodPatch, "/user/change-email", `{
			"UserID":"client-controlled",
			"email":"new@example.com"
		}`)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		wantInput := user.UpdateEmailInput{UserID: "authenticated-user", Email: "new@example.com"}
		if !reflect.DeepEqual(gotInput, wantInput) {
			t.Fatalf("ChangeEmail input = %#v, want %#v", gotInput, wantInput)
		}
		assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"user email updated successfully"}`)
	})

	t.Run("password", func(t *testing.T) {
		var gotInput user.UpdatePassInput
		usecase := &stubUserHandlerUseCase{
			changePassword: func(_ context.Context, input user.UpdatePassInput) error {
				gotInput = input
				return nil
			},
		}
		router := newUserHandlerTestRouter(usecase)
		recorder := performUserHandlerRequest(t, router, http.MethodPatch, "/user/change-password", `{
			"UserID":"client-controlled",
			"current_password":"OldPassword1!",
			"new_password":"NewPassword1!"
		}`)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		wantInput := user.UpdatePassInput{
			UserID:      "authenticated-user",
			OldPassword: "OldPassword1!",
			NewPassword: "NewPassword1!",
		}
		if !reflect.DeepEqual(gotInput, wantInput) {
			t.Fatalf("ChangePassword input = %#v, want %#v", gotInput, wantInput)
		}
		assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"password updated successfully"}`)
	})

	t.Run("role", func(t *testing.T) {
		var gotInput user.UpdateRoleInput
		usecase := &stubUserHandlerUseCase{
			changeRole: func(_ context.Context, input user.UpdateRoleInput) error {
				gotInput = input
				return nil
			},
		}
		router := newUserHandlerTestRouter(usecase)
		recorder := performUserHandlerRequest(t, router, http.MethodPatch, "/user/"+userHandlerTargetID+"/change-role", `{
			"UserID":"client-controlled",
			"role":"admin"
		}`)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		wantInput := user.UpdateRoleInput{UserID: userHandlerTargetID, Role: valueobject.RoleAdmin}
		if !reflect.DeepEqual(gotInput, wantInput) {
			t.Fatalf("ChangeRole input = %#v, want %#v", gotInput, wantInput)
		}
		assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"role updated successfully"}`)
	})

	t.Run("status", func(t *testing.T) {
		var gotInput user.UpdateStatusInput
		usecase := &stubUserHandlerUseCase{
			changeStatus: func(_ context.Context, input user.UpdateStatusInput) error {
				gotInput = input
				return nil
			},
		}
		router := newUserHandlerTestRouter(usecase)
		recorder := performUserHandlerRequest(t, router, http.MethodPatch, "/user/"+userHandlerTargetID+"/change-status", `{
			"UserID":"client-controlled",
			"ActorID":"client-controlled",
			"status":"suspended"
		}`)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		wantInput := user.UpdateStatusInput{
			UserID:  userHandlerTargetID,
			ActorID: "authenticated-user",
			Status:  valueobject.StatusSuspended,
		}
		if !reflect.DeepEqual(gotInput, wantInput) {
			t.Fatalf("ChangeStatus input = %#v, want %#v", gotInput, wantInput)
		}
		assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"status updated successfully"}`)
	})
}

func TestUserHandlerPreservesRequiredFieldBinding(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create password", method: http.MethodPost, path: "/user", body: `{"status":"active","role":"client"}`},
		{name: "email", method: http.MethodPatch, path: "/user/change-email", body: `{}`},
		{name: "password current", method: http.MethodPatch, path: "/user/change-password", body: `{"new_password":"NewPassword1!"}`},
		{name: "role", method: http.MethodPatch, path: "/user/" + userHandlerTargetID + "/change-role", body: `{}`},
		{name: "status", method: http.MethodPatch, path: "/user/" + userHandlerTargetID + "/change-status", body: `{}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := newUserHandlerTestRouter(&stubUserHandlerUseCase{})
			recorder := performUserHandlerRequest(t, router, test.method, test.path, test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), `{
				"type":"/errors/validation",
				"title":"Bad Request",
				"status":400,
				"detail":"Invalid request payload",
				"instance":"`+test.path+`"
			}`)
		})
	}
}

func newUserHandlerTestRouter(usecase user.UseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewUserHandler(usecase, discardUserHandlerLogger{})
	authenticated := func(c *gin.Context) {
		c.Set("user_id", "authenticated-user")
		c.Next()
	}

	router.POST("/user", handler.CreateUser)
	router.GET("/user", authenticated, handler.GetUser)
	router.GET("/user/list", authenticated, handler.GetUserList)
	router.GET("/user/:id", handler.GetUser)
	router.PUT("/user/:id", handler.UpdateUser)
	router.PATCH("/user/change-email", authenticated, handler.ChangeEmail)
	router.PATCH("/user/change-password", authenticated, handler.ChangePassword)
	router.PATCH("/user/:id/change-role", handler.ChangeRole)
	router.PATCH("/user/:id/change-status", authenticated, handler.ChangeStatus)
	return router
}

func performUserHandlerRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertUserHandlerJSONEqual(t *testing.T, got []byte, want string) {
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

type stubUserHandlerUseCase struct {
	user.UseCase
	createUser     func(context.Context, user.CreateInput) (user.UserOutput, error)
	getUser        func(context.Context, string) (user.UserOutput, error)
	getUsersList   func(context.Context, user.GetListInput) ([]user.UserOutput, int64, error)
	updateUser     func(context.Context, user.UpdateInput) error
	changeEmail    func(context.Context, user.UpdateEmailInput) error
	changePassword func(context.Context, user.UpdatePassInput) error
	changeRole     func(context.Context, user.UpdateRoleInput) error
	changeStatus   func(context.Context, user.UpdateStatusInput) error
}

func (s *stubUserHandlerUseCase) CreateUser(ctx context.Context, input user.CreateInput) (user.UserOutput, error) {
	return s.createUser(ctx, input)
}

func (s *stubUserHandlerUseCase) GetUser(ctx context.Context, userID string) (user.UserOutput, error) {
	return s.getUser(ctx, userID)
}

func (s *stubUserHandlerUseCase) GetUserslist(ctx context.Context, input user.GetListInput) ([]user.UserOutput, int64, error) {
	return s.getUsersList(ctx, input)
}

func (s *stubUserHandlerUseCase) UpdateUser(ctx context.Context, input user.UpdateInput) error {
	return s.updateUser(ctx, input)
}

func (s *stubUserHandlerUseCase) ChangeEmail(ctx context.Context, input user.UpdateEmailInput) error {
	return s.changeEmail(ctx, input)
}

func (s *stubUserHandlerUseCase) ChangePassword(ctx context.Context, input user.UpdatePassInput) error {
	return s.changePassword(ctx, input)
}

func (s *stubUserHandlerUseCase) ChangeRole(ctx context.Context, input user.UpdateRoleInput) error {
	return s.changeRole(ctx, input)
}

func (s *stubUserHandlerUseCase) ChangeStatus(ctx context.Context, input user.UpdateStatusInput) error {
	return s.changeStatus(ctx, input)
}

type discardUserHandlerLogger struct{}

func (discardUserHandlerLogger) Info(string, ...any)  {}
func (discardUserHandlerLogger) Error(string, ...any) {}
func (discardUserHandlerLogger) Warn(string, ...any)  {}
func (discardUserHandlerLogger) Debug(string, ...any) {}
func (discardUserHandlerLogger) Panic(string, ...any) {}
