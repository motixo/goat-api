package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	httpMiddleware "github.com/motixo/goat-api/internal/delivery/http/middleware"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	domainService "github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authentication"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

func TestAuthHandlerLoginPreservesRequestAndResponseContract(t *testing.T) {
	accessExpiresAt := time.Date(2026, time.July, 23, 10, 15, 0, 0, time.UTC)
	refreshExpiresAt := accessExpiresAt.Add(24 * time.Hour)
	createdAt := time.Date(2026, time.July, 20, 8, 30, 0, 0, time.UTC)
	var gotInput authentication.LoginInput
	usecase := &stubAuthenticationUseCase{
		login: func(_ context.Context, input authentication.LoginInput) (authentication.LoginOutput, error) {
			gotInput = input
			return authentication.LoginOutput{
				AccessToken:           "access-token",
				AccessTokenExpiresAt:  accessExpiresAt,
				RefreshToken:          "refresh-token",
				RefreshTokenExpiresAt: refreshExpiresAt,
				User: authentication.UserOutput{
					ID:        "user-1",
					Email:     "user@example.com",
					Role:      "client",
					Status:    "active",
					CreatedAt: createdAt,
				},
			}, nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	recorder := performAuthHandlerRequest(t, router, http.MethodPost, "/auth/login", `{
		"email":"user@example.com",
		"password":"Password1!",
		"IP":"client-controlled-ip",
		"Device":"client-controlled-device"
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	wantInput := authentication.LoginInput{
		Email:    "user@example.com",
		Password: "Password1!",
		IP:       "203.0.113.9",
		Device:   "contract-test-agent",
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("Login input = %#v, want %#v", gotInput, wantInput)
	}
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"access_token":"access-token",
			"access_token_expires_at":"2026-07-23T10:15:00Z",
			"refresh_token":"refresh-token",
			"refresh_token_expires_at":"2026-07-24T10:15:00Z",
			"user": {
				"id":"user-1",
				"email":"user@example.com",
				"Role":"client",
				"Status":"active",
				"createdAt":"2026-07-20T08:30:00Z"
			}
		}
	}`)
}

func TestAuthHandlerRegisterPreservesRequestAndResponseContract(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 9, 0, 0, 0, time.UTC)
	var gotInput authentication.RegisterInput
	usecase := &stubAuthenticationUseCase{
		signup: func(_ context.Context, input authentication.RegisterInput) (authentication.UserOutput, error) {
			gotInput = input
			return authentication.UserOutput{
				ID:        "user-2",
				Email:     "new@example.com",
				Role:      "client",
				Status:    "active",
				CreatedAt: createdAt,
			}, nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	recorder := performAuthHandlerRequest(t, router, http.MethodPost, "/auth/signup", `{
		"email":"new@example.com",
		"password":"Password1!"
	}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	wantInput := authentication.RegisterInput{Email: "new@example.com", Password: "Password1!"}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("Signup input = %#v, want %#v", gotInput, wantInput)
	}
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"id":"user-2",
			"email":"new@example.com",
			"Role":"client",
			"Status":"active",
			"createdAt":"2026-07-23T09:00:00Z"
		}
	}`)
}

func TestAuthHandlerRefreshPreservesRequestAndResponseContract(t *testing.T) {
	accessExpiresAt := time.Date(2026, time.July, 23, 10, 15, 0, 0, time.UTC)
	refreshExpiresAt := accessExpiresAt.Add(24 * time.Hour)
	var gotInput authentication.RefreshInput
	usecase := &stubAuthenticationUseCase{
		refresh: func(_ context.Context, input authentication.RefreshInput) (authentication.RefreshOutput, error) {
			gotInput = input
			return authentication.RefreshOutput{
				AccessToken:           "new-access-token",
				AccessTokenExpiresAt:  accessExpiresAt,
				RefreshToken:          "new-refresh-token",
				RefreshTokenExpiresAt: refreshExpiresAt,
			}, nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	recorder := performAuthHandlerRequest(t, router, http.MethodPost, "/auth/refresh", `{
		"refresh_token":"old-refresh-token",
		"IP":"client-controlled-ip",
		"Device":"client-controlled-device"
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	wantInput := authentication.RefreshInput{
		RefreshToken: "old-refresh-token",
		IP:           "203.0.113.9",
		Device:       "contract-test-agent",
	}
	if !reflect.DeepEqual(gotInput, wantInput) {
		t.Fatalf("Refresh input = %#v, want %#v", gotInput, wantInput)
	}
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"data": {
			"access_token":"new-access-token",
			"access_token_expires_at":"2026-07-23T10:15:00Z",
			"refresh_token":"new-refresh-token",
			"refresh_token_expires_at":"2026-07-24T10:15:00Z"
		}
	}`)
}

func TestAuthHandlerLogoutPreservesAuthenticatedContract(t *testing.T) {
	var gotSessionID, gotUserID string
	usecase := &stubAuthenticationUseCase{
		logout: func(_ context.Context, sessionID, userID string) error {
			gotSessionID = sessionID
			gotUserID = userID
			return nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	recorder := performAuthHandlerRequest(t, router, http.MethodPost, "/auth/logout", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotSessionID != "01ARZ3NDEKTSV4RRFFQ69G5FAV" || gotUserID != "authenticated-user" {
		t.Fatalf("Logout principal = session %q, user %q; want authenticated context", gotSessionID, gotUserID)
	}
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{"data":"logout successful"}`)
}

func TestAuthHandlerLogoutPreservesKnownFailureContracts(t *testing.T) {
	currentSessionInvalid := authentication.NewCurrentSessionInvalidError(domainErrors.ErrNotFound)
	for _, variant := range []struct {
		name string
		err  error
	}{
		{name: "direct", err: currentSessionInvalid},
		{name: "wrapped", err: fmt.Errorf("logout failed: %w", currentSessionInvalid)},
	} {
		t.Run(variant.name, func(t *testing.T) {
			usecase := &stubAuthenticationUseCase{
				logout: func(context.Context, string, string) error {
					return variant.err
				},
			}
			recorder := performAuthHandlerRequest(t, newAuthHandlerTestRouter(usecase), http.MethodPost, "/auth/logout", "")

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
			assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
				"type":"/errors/unauthorized",
				"title":"Unauthorized",
				"status":401,
				"detail":"not found",
				"instance":"/auth/logout"
			}`)
		})
	}
}

func TestAuthHandlerLogoutDoesNotExposeUnknownFailures(t *testing.T) {
	usecase := &stubAuthenticationUseCase{
		logout: func(context.Context, string, string) error {
			return errors.New("redis dial tcp 10.0.0.5:6379: connection refused")
		},
	}
	recorder := performAuthHandlerRequest(t, newAuthHandlerTestRouter(usecase), http.MethodPost, "/auth/logout", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"type":"/errors/internal",
		"title":"Internal Server Error",
		"status":500,
		"detail":"An unexpected error occurred.",
		"instance":"/auth/logout"
	}`)
}

func TestAuthHandlerLoginPreservesInvalidCredentialsProblemContract(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		wantLanguage   string
		wantTitle      string
		wantDetail     string
	}{
		{
			name:         "English",
			wantLanguage: "en",
			wantTitle:    "Unauthorized",
			wantDetail:   "Invalid email or password.",
		},
		{
			name:           "Persian",
			acceptLanguage: "fa-IR",
			wantLanguage:   "fa",
			wantTitle:      "لطفاً وارد حساب خود شوید",
			wantDetail:     "ایمیل یا رمز عبور اشتباه است.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubAuthenticationUseCase{
				login: func(context.Context, authentication.LoginInput) (authentication.LoginOutput, error) {
					return authentication.LoginOutput{}, fmt.Errorf(
						"authenticate credentials: %w: %w",
						domainErrors.ErrInvalidCredentials,
						domainService.ErrInvalidStoredPasswordHash,
					)
				},
			}
			recorder := performAuthHandlerRequestWithLanguage(
				t,
				newAuthHandlerTestRouter(usecase),
				http.MethodPost,
				"/auth/login",
				`{"email":"user@example.com","password":"wrong-password"}`,
				test.acceptLanguage,
			)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Language"); got != test.wantLanguage {
				t.Fatalf("Content-Language = %q, want %q", got, test.wantLanguage)
			}
			assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
				"type":"/errors/internal",
				"title":`+strconv.Quote(test.wantTitle)+`,
				"status":401,
				"detail":`+strconv.Quote(test.wantDetail)+`,
				"instance":"/auth/login"
			}`)
		})
	}
}

func TestAuthHandlerLoginLocalizesInactiveAccountProblem(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		wantLanguage   string
		wantTitle      string
		wantDetail     string
	}{
		{
			name:         "English",
			wantLanguage: "en",
			wantTitle:    "Unauthorized",
			wantDetail:   "account not activated.",
		},
		{
			name:           "Persian",
			acceptLanguage: "fa-IR",
			wantLanguage:   "fa",
			wantTitle:      "لطفاً وارد حساب خود شوید",
			wantDetail:     "حساب شما هنوز فعال نشده است.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubAuthenticationUseCase{
				login: func(context.Context, authentication.LoginInput) (authentication.LoginOutput, error) {
					return authentication.LoginOutput{}, fmt.Errorf(
						"authenticate account: %w",
						authorization.ErrPrincipalInactive,
					)
				},
			}
			recorder := performAuthHandlerRequestWithLanguage(
				t,
				newAuthHandlerTestRouter(usecase),
				http.MethodPost,
				"/auth/login",
				`{"email":"inactive@example.com","password":"Password1!"}`,
				test.acceptLanguage,
			)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code,
					http.StatusUnauthorized,
					recorder.Body.String(),
				)
			}
			if got := recorder.Header().Get("Content-Language"); got != test.wantLanguage {
				t.Fatalf("Content-Language = %q, want %q", got, test.wantLanguage)
			}
			assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
				"type":"/errors/unauthorized",
				"title":`+strconv.Quote(test.wantTitle)+`,
				"status":401,
				"detail":`+strconv.Quote(test.wantDetail)+`,
				"instance":"/auth/login"
			}`)
		})
	}
}

func TestAuthHandlerRejectsMalformedJSONBeforeUseCase(t *testing.T) {
	usecase := &stubAuthenticationUseCase{
		login: func(context.Context, authentication.LoginInput) (authentication.LoginOutput, error) {
			t.Fatal("Login called for malformed JSON")
			return authentication.LoginOutput{}, nil
		},
		signup: func(context.Context, authentication.RegisterInput) (authentication.UserOutput, error) {
			t.Fatal("Signup called for malformed JSON")
			return authentication.UserOutput{}, nil
		},
		refresh: func(context.Context, authentication.RefreshInput) (authentication.RefreshOutput, error) {
			t.Fatal("Refresh called for malformed JSON")
			return authentication.RefreshOutput{}, nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	tests := []struct {
		path string
		body string
	}{
		{path: "/auth/login", body: `{"email":42,"password":"Password1!"}`},
		{path: "/auth/signup", body: `{"email":42,"password":"Password1!"}`},
		{path: "/auth/refresh", body: `{"refresh_token":42}`},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := performAuthHandlerRequest(t, router, http.MethodPost, test.path, test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
				"type":"/errors/validation",
				"title":"Bad Request",
				"status":400,
				"detail":"Invalid request payload",
				"instance":"`+test.path+`"
			}`)
		})
	}
}

func TestAuthHandlerLocalizesValidationProblems(t *testing.T) {
	usecase := &stubAuthenticationUseCase{
		login: func(context.Context, authentication.LoginInput) (authentication.LoginOutput, error) {
			t.Fatal("Login called for malformed JSON")
			return authentication.LoginOutput{}, nil
		},
	}
	recorder := performAuthHandlerRequestWithLanguage(
		t,
		newAuthHandlerTestRouter(usecase),
		http.MethodPost,
		"/auth/login",
		`{"email":42}`,
		"fa-IR",
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
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
	assertAuthHandlerJSONEqual(t, recorder.Body.Bytes(), `{
		"type":"/errors/validation",
		"title":"درخواست نامعتبر",
		"status":400,
		"detail":"اطلاعات واردشده معتبر نیست.",
		"instance":"/auth/login"
	}`)
}

func TestAuthHandlerPreservesExistingMissingFieldBindingBehavior(t *testing.T) {
	loginCalled := false
	registerCalled := false
	refreshCalled := false
	usecase := &stubAuthenticationUseCase{
		login: func(_ context.Context, input authentication.LoginInput) (authentication.LoginOutput, error) {
			loginCalled = input.Email == "" && input.Password == ""
			return authentication.LoginOutput{}, nil
		},
		signup: func(_ context.Context, input authentication.RegisterInput) (authentication.UserOutput, error) {
			registerCalled = input.Email == "not-an-email" && input.Password == ""
			return authentication.UserOutput{}, nil
		},
		refresh: func(_ context.Context, input authentication.RefreshInput) (authentication.RefreshOutput, error) {
			refreshCalled = input.RefreshToken == ""
			return authentication.RefreshOutput{}, nil
		},
	}
	router := newAuthHandlerTestRouter(usecase)

	tests := []struct {
		path       string
		body       string
		wantStatus int
	}{
		{path: "/auth/login", body: `{}`, wantStatus: http.StatusOK},
		{path: "/auth/signup", body: `{"email":"not-an-email"}`, wantStatus: http.StatusCreated},
		{path: "/auth/refresh", body: `{}`, wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		recorder := performAuthHandlerRequest(t, router, http.MethodPost, test.path, test.body)
		if recorder.Code != test.wantStatus {
			t.Fatalf("%s status = %d, want %d; body = %s", test.path, recorder.Code, test.wantStatus, recorder.Body.String())
		}
	}
	if !loginCalled || !registerCalled || !refreshCalled {
		t.Fatalf("missing-field calls = login %t, signup %t, refresh %t; want all true", loginCalled, registerCalled, refreshCalled)
	}
}

func newAuthHandlerTestRouter(usecase authentication.UseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAuthHandler(usecase, discardAuthHandlerLogger{})
	router.POST("/auth/login", handler.Login)
	router.POST("/auth/signup", handler.Register)
	router.POST("/auth/refresh", handler.Refresh)
	router.POST("/auth/logout", func(c *gin.Context) {
		setHandlerTestPrincipal(
			c,
			"authenticated-user",
			"01ARZ3NDEKTSV4RRFFQ69G5FAV",
			valueobject.RoleClient,
		)
		c.Next()
	}, handler.Logout)
	return router
}

func performAuthHandlerRequest(
	t *testing.T,
	router http.Handler,
	method, path, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	return performAuthHandlerRequestWithLanguage(t, router, method, path, body, "")
}

func performAuthHandlerRequestWithLanguage(
	t *testing.T,
	router http.Handler,
	method, path, body, acceptLanguage string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "203.0.113.9:4321"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "contract-test-agent")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertAuthHandlerJSONEqual(t *testing.T, got []byte, want string) {
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

type stubAuthenticationUseCase struct {
	authentication.UseCase
	login   func(context.Context, authentication.LoginInput) (authentication.LoginOutput, error)
	signup  func(context.Context, authentication.RegisterInput) (authentication.UserOutput, error)
	refresh func(context.Context, authentication.RefreshInput) (authentication.RefreshOutput, error)
	logout  func(context.Context, string, string) error
}

func (s *stubAuthenticationUseCase) Login(ctx context.Context, input authentication.LoginInput) (authentication.LoginOutput, error) {
	return s.login(ctx, input)
}

func (s *stubAuthenticationUseCase) Signup(ctx context.Context, input authentication.RegisterInput) (authentication.UserOutput, error) {
	return s.signup(ctx, input)
}

func (s *stubAuthenticationUseCase) Refresh(ctx context.Context, input authentication.RefreshInput) (authentication.RefreshOutput, error) {
	return s.refresh(ctx, input)
}

func (s *stubAuthenticationUseCase) Logout(ctx context.Context, sessionID, userID string) error {
	return s.logout(ctx, sessionID, userID)
}

type discardAuthHandlerLogger struct{}

func (discardAuthHandlerLogger) Info(string, ...any)  {}
func (discardAuthHandlerLogger) Error(string, ...any) {}
func (discardAuthHandlerLogger) Warn(string, ...any)  {}
func (discardAuthHandlerLogger) Debug(string, ...any) {}
func (discardAuthHandlerLogger) Panic(string, ...any) {}

func setHandlerTestPrincipal(
	c *gin.Context,
	userID string,
	sessionID string,
	role valueobject.UserRole,
) {
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermFullAccess,
	})
	if err != nil {
		panic(err)
	}
	principal, err := authorization.NewPrincipal(
		userID,
		sessionID,
		7,
		role,
		permissions,
	)
	if err != nil {
		panic(err)
	}
	httpMiddleware.SetPrincipal(c, principal)
}
