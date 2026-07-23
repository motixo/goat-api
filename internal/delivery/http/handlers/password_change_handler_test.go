package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/user"
)

func TestUserHandlerChangePasswordPreservesLocalizedErrorContracts(t *testing.T) {
	infrastructureErr := errors.New("redis connection details must not be exposed")
	tests := []struct {
		name       string
		locale     string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "incorrect current password in English",
			locale:     "en",
			err:        domainErrors.ErrInvalidPassword,
			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"type":"/errors/validation",
				"title":"Bad Request",
				"status":400,
				"detail":"current password is incorrect",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "incorrect current password in Persian",
			locale:     "fa",
			err:        domainErrors.ErrInvalidPassword,
			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"type":"/errors/validation",
				"title":"درخواست نامعتبر",
				"status":400,
				"detail":"رمز عبور فعلی اشتباه است.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "invalid new password in English",
			locale:     "en-US",
			err:        domainErrors.ErrPasswordTooShort,
			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"type":"/errors/invalid-password",
				"title":"Bad Request",
				"status":400,
				"detail":"Password must be at least 8 characters long.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "invalid new password in Persian",
			locale:     "fa-IR",
			err:        domainErrors.ErrPasswordTooShort,
			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"type":"/errors/invalid-password",
				"title":"درخواست نامعتبر",
				"status":400,
				"detail":"رمز عبور باید حداقل ۸ کاراکتر باشد.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "user not found in English",
			locale:     "en",
			err:        domainErrors.ErrUserNotFound,
			wantStatus: http.StatusNotFound,
			wantBody: `{
				"type":"/errors/not-found",
				"title":"Not Found",
				"status":404,
				"detail":"The requested resource was not found.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "user not found in Persian",
			locale:     "fa",
			err:        domainErrors.ErrUserNotFound,
			wantStatus: http.StatusNotFound,
			wantBody: `{
				"type":"/errors/not-found",
				"title":"پیدا نشد",
				"status":404,
				"detail":"اطلاعات موردنظر پیدا نشد.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "infrastructure failure is safe in English",
			locale:     "en",
			err:        infrastructureErr,
			wantStatus: http.StatusInternalServerError,
			wantBody: `{
				"type":"/errors/internal",
				"title":"Internal Server Error",
				"status":500,
				"detail":"An unexpected error occurred.",
				"instance":"/user/change-password"
			}`,
		},
		{
			name:       "infrastructure failure is safe in Persian",
			locale:     "fa",
			err:        infrastructureErr,
			wantStatus: http.StatusInternalServerError,
			wantBody: `{
				"type":"/errors/internal",
				"title":"خطای سرور",
				"status":500,
				"detail":"مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
				"instance":"/user/change-password"
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usecase := &stubUserHandlerUseCase{
				changePassword: func(context.Context, user.UpdatePassInput) error {
					return fmt.Errorf("change password: %w", test.err)
				},
			}
			request := httptest.NewRequest(
				http.MethodPatch,
				"/user/change-password",
				strings.NewReader(`{
					"current_password":"OldPassword1!",
					"new_password":"NewPassword1!"
				}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept-Language", test.locale)
			recorder := httptest.NewRecorder()

			newUserHandlerTestRouter(usecase).ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			assertUserHandlerJSONEqual(t, recorder.Body.Bytes(), test.wantBody)
		})
	}
}
