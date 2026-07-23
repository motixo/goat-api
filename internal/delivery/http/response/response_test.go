package response

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type mappedProblemCase struct {
	name      string
	err       error
	status    int
	typ       string
	titleKey  TranslationKey
	detailKey TranslationKey
	title     string
	detail    string
}

var mappedProblemCases = []mappedProblemCase{
	{name: "bad request", err: domainErrors.ErrBadRequest, status: http.StatusBadRequest, typ: "/errors/internal", titleKey: titleBadRequest, detailKey: detailProcessingError, title: "Bad Request", detail: "An error occurred while processing your request."},
	{name: "invalid input", err: domainErrors.ErrInvalidInput, status: http.StatusBadRequest, typ: "/errors/internal", titleKey: titleBadRequest, detailKey: detailProcessingError, title: "Bad Request", detail: "An error occurred while processing your request."},
	{name: "password too short", err: domainErrors.ErrPasswordTooShort, status: http.StatusBadRequest, typ: "/errors/invalid-password", titleKey: titleBadRequest, detailKey: detailPasswordTooShort, title: "Bad Request", detail: "Password must be at least 8 characters long."},
	{name: "password too long", err: domainErrors.ErrPasswordTooLong, status: http.StatusBadRequest, typ: "/errors/invalid-password", titleKey: titleBadRequest, detailKey: detailPasswordTooLong, title: "Bad Request", detail: "Password must not exceed 72 characters."},
	{name: "password policy", err: domainErrors.ErrPasswordPolicyViolation, status: http.StatusBadRequest, typ: "/errors/invalid-password", titleKey: titleBadRequest, detailKey: detailPasswordPolicyViolation, title: "Bad Request", detail: "Password must contain uppercase, lowercase, number, and special character."},
	{name: "invalid current password", err: domainErrors.ErrInvalidPassword, status: http.StatusBadRequest, typ: "/errors/validation", titleKey: titleBadRequest, detailKey: detailCurrentPasswordIncorrect, title: "Bad Request", detail: "current password is incorrect"},
	{name: "invalid session selection", err: session.ErrInvalidSessionSelection, status: http.StatusBadRequest, typ: "/errors/validation", titleKey: titleBadRequest, detailKey: DetailInvalidRequestPayload, title: "Bad Request", detail: "Invalid request payload"},
	{name: "unauthorized", err: domainErrors.ErrUnauthorized, status: http.StatusUnauthorized, typ: "/errors/internal", titleKey: titleUnauthorized, detailKey: detailProcessingError, title: "Unauthorized", detail: "An error occurred while processing your request."},
	{name: "token expired", err: domainErrors.ErrTokenExpired, status: http.StatusUnauthorized, typ: "/errors/unauthorized", titleKey: titleUnauthorized, detailKey: detailTokenExpired, title: "Unauthorized", detail: "token has expired"},
	{name: "token invalid", err: domainErrors.ErrTokenInvalid, status: http.StatusUnauthorized, typ: "/errors/unauthorized", titleKey: titleUnauthorized, detailKey: detailTokenInvalid, title: "Unauthorized", detail: "invalid or malformed token"},
	{name: "current session invalid", err: auth.NewCurrentSessionInvalidError(domainErrors.ErrNotFound), status: http.StatusUnauthorized, typ: "/errors/unauthorized", titleKey: titleUnauthorized, detailKey: detailCurrentSessionNotFound, title: "Unauthorized", detail: "not found"},
	{name: "invalid credentials", err: domainErrors.ErrInvalidCredentials, status: http.StatusUnauthorized, typ: "/errors/internal", titleKey: titleUnauthorized, detailKey: detailInvalidCredentials, title: "Unauthorized", detail: "Invalid email or password."},
	{name: "forbidden", err: domainErrors.ErrForbidden, status: http.StatusForbidden, typ: "/errors/internal", titleKey: titleForbidden, detailKey: detailProcessingError, title: "Forbidden", detail: "An error occurred while processing your request."},
	{name: "account suspended", err: domainErrors.ErrAccountSuspended, status: http.StatusForbidden, typ: "/errors/internal", titleKey: titleForbidden, detailKey: detailAccountSuspendedContactSupport, title: "Forbidden", detail: "Your account has been suspended. Please contact support."},
	{name: "not found", err: domainErrors.ErrNotFound, status: http.StatusNotFound, typ: "/errors/not-found", titleKey: titleNotFound, detailKey: detailResourceNotFound, title: "Not Found", detail: "The requested resource was not found."},
	{name: "user not found", err: domainErrors.ErrUserNotFound, status: http.StatusNotFound, typ: "/errors/not-found", titleKey: titleNotFound, detailKey: detailResourceNotFound, title: "Not Found", detail: "The requested resource was not found."},
	{name: "permission not found", err: domainErrors.ErrPermissionNotFound, status: http.StatusNotFound, typ: "/errors/not-found", titleKey: titleNotFound, detailKey: detailResourceNotFound, title: "Not Found", detail: "The requested resource was not found."},
	{name: "conflict", err: domainErrors.ErrConflict, status: http.StatusConflict, typ: "/errors/conflict", titleKey: titleConflict, detailKey: detailConflict, title: "Conflict", detail: "The request conflicts with current state."},
	{name: "email exists", err: domainErrors.ErrEmailAlreadyExists, status: http.StatusConflict, typ: "/errors/email-already-exists", titleKey: titleConflict, detailKey: detailEmailAlreadyExists, title: "Conflict", detail: "This email is already registered."},
	{name: "permission exists", err: domainErrors.ErrPermissionAlreadyExists, status: http.StatusConflict, typ: "/errors/conflict", titleKey: titleConflict, detailKey: detailConflict, title: "Conflict", detail: "The request conflicts with current state."},
	{name: "password unchanged", err: domainErrors.ErrPasswordSameAsCurrent, status: http.StatusConflict, typ: "/errors/internal", titleKey: titleConflict, detailKey: detailPasswordSameAsCurrent, title: "Conflict", detail: "Passwords can't be same"},
	{name: "rate limited", err: domainErrors.ErrRateLimitExceeded, status: http.StatusTooManyRequests, typ: "/errors/internal", titleKey: titleTooManyRequests, detailKey: detailProcessingError, title: "Too Many Requests", detail: "An error occurred while processing your request."},
	{name: "internal", err: domainErrors.ErrInternal, status: http.StatusInternalServerError, typ: "/errors/internal", titleKey: titleInternalServerError, detailKey: detailUnexpected, title: "Internal Server Error", detail: "An unexpected error occurred."},
	{name: "inactive user", err: domainErrors.ErrUserInactive, status: http.StatusInternalServerError, typ: "/errors/internal", titleKey: titleInternalServerError, detailKey: detailUnexpected, title: "Internal Server Error", detail: "An unexpected error occurred."},
	{name: "password hashing failed", err: domainErrors.ErrPasswordHashingFailed, status: http.StatusInternalServerError, typ: "/errors/internal", titleKey: titleInternalServerError, detailKey: detailUnexpected, title: "Internal Server Error", detail: "An unexpected error occurred."},
	{name: "unknown", err: errors.New("postgres connection details"), status: http.StatusInternalServerError, typ: "/errors/internal", titleKey: titleInternalServerError, detailKey: detailUnexpected, title: "Internal Server Error", detail: "An unexpected error occurred."},
}

func TestMapErrorClassifiesAndPreservesEnglishProblemContracts(t *testing.T) {
	for _, test := range mappedProblemCases {
		for _, variant := range []struct {
			name string
			err  error
		}{
			{name: "direct", err: test.err},
			{name: "wrapped", err: fmt.Errorf("use case context: %w", test.err)},
		} {
			t.Run(test.name+"/"+variant.name, func(t *testing.T) {
				wantDescriptor := ProblemDescriptor{
					Type:      test.typ,
					Status:    test.status,
					TitleKey:  test.titleKey,
					DetailKey: test.detailKey,
				}
				if got := MapError(variant.err); !reflect.DeepEqual(got, wantDescriptor) {
					t.Fatalf("MapError() = %#v, want %#v", got, wantDescriptor)
				}

				recorder := renderMappedError(t, variant.err, "")
				assertProblemHTTPContract(t, recorder, "en", "Accept-Language", Problem{
					Type:     test.typ,
					Title:    test.title,
					Status:   test.status,
					Detail:   test.detail,
					Instance: "/resource",
				})
			})
		}
	}
}

func TestPersianCatalogRendersEveryTranslationKey(t *testing.T) {
	for _, key := range translationKeys {
		t.Run(string(key), func(t *testing.T) {
			if _, exists := problemLocalizer.catalogs[localePersian][key]; !exists {
				t.Fatalf("Persian catalog does not define %q", key)
			}

			params := translationParamsForTest(key)
			persian := problemLocalizer.translate(localePersian, key, params)
			english := problemLocalizer.translate(localeEnglish, key, params)
			if persian == "" {
				t.Fatal("Persian translation is empty")
			}
			if persian == english {
				t.Fatalf("Persian translation %q unexpectedly equals English", persian)
			}
			if strings.Contains(persian, string(key)) || strings.Contains(persian, "{{") {
				t.Fatalf("Persian translation leaked key or template syntax: %q", persian)
			}
		})
	}
}

func TestDeliveryEnglishTranslationsPreserveExistingContracts(t *testing.T) {
	expected := map[TranslationKey]string{
		DetailInvalidRequestPayload:        "Invalid request payload",
		DetailInvalidPaginationParams:      "invalid pagination params",
		DetailAuthenticationContextMissing: "authentication context missing",
		DetailInvalidUserRole:              "invalid user role: auditor",
		DetailMissingAuthorizationHeader:   "missing or invalid Authorization header",
		DetailAccessTokenRequired:          "access token required",
		DetailTokenRevoked:                 "token has been revoked",
		DetailAccountNotActivated:          "account not activated.",
		DetailAccountSuspended:             "account suspended.",
		DetailContactSupport:               "someting went wrong, contact support.",
		DetailAuthenticationRequired:       "authentication required",
		DetailInvalidUserContext:           "invalid user context",
		DetailInsufficientPermissions:      "insufficient permissions",
		DetailRateLimitExceeded:            "Limit exceeded. Please try again in 2s.",
	}

	for key, want := range expected {
		if got := problemLocalizer.translate(localeEnglish, key, translationParamsForTest(key)); got != want {
			t.Errorf("English translation %q = %q, want %q", key, got, want)
		}
	}
}

func TestResolveLocaleFromAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   locale
	}{
		{name: "missing", header: "", want: localeEnglish},
		{name: "English", header: "en", want: localeEnglish},
		{name: "American English", header: "en-US", want: localeEnglish},
		{name: "Persian", header: "fa", want: localePersian},
		{name: "Iranian Persian", header: "fa-IR", want: localePersian},
		{name: "weighted Persian", header: "en-US;q=0.3, fa-IR;q=0.9", want: localePersian},
		{name: "weighted English", header: "fa;q=0.2, en;q=0.8", want: localeEnglish},
		{name: "unsupported then Persian", header: "fr-FR;q=1, fa;q=0.7", want: localePersian},
		{name: "unsupported", header: "de-DE", want: localeEnglish},
		{name: "wildcard", header: "*", want: localeEnglish},
		{name: "malformed weight", header: "fa;q=invalid", want: localeEnglish},
		{name: "malformed tag", header: "fa-@-IR", want: localeEnglish},
		{name: "Persian disabled", header: "fa;q=0, en;q=0.5", want: localeEnglish},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveLocale(test.header); got != test.want {
				t.Fatalf("resolveLocale(%q) = %q, want %q", test.header, got, test.want)
			}
		})
	}
}

func TestWriteProblemNegotiatesExactHTTPContract(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		err            error
		wantLanguage   string
		wantProblem    Problem
	}{
		{
			name:         "missing language defaults to English",
			err:          domainErrors.ErrNotFound,
			wantLanguage: "en",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			},
		},
		{
			name:           "English",
			acceptLanguage: "en",
			err:            domainErrors.ErrTokenExpired,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/unauthorized",
				Title:    "Unauthorized",
				Status:   http.StatusUnauthorized,
				Detail:   "token has expired",
				Instance: "/resource",
			},
		},
		{
			name:           "American English",
			acceptLanguage: "en-US",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			},
		},
		{
			name:           "Persian",
			acceptLanguage: "fa",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "fa",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "پیدا نشد",
				Status:   http.StatusNotFound,
				Detail:   "اطلاعات موردنظر پیدا نشد.",
				Instance: "/resource",
			},
		},
		{
			name:           "Iranian Persian",
			acceptLanguage: "fa-IR",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "fa",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "پیدا نشد",
				Status:   http.StatusNotFound,
				Detail:   "اطلاعات موردنظر پیدا نشد.",
				Instance: "/resource",
			},
		},
		{
			name:           "weighted Persian preference",
			acceptLanguage: "en-US;q=0.3, fa-IR;q=0.9",
			err:            fmt.Errorf("wrapped: %w", domainErrors.ErrNotFound),
			wantLanguage:   "fa",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "پیدا نشد",
				Status:   http.StatusNotFound,
				Detail:   "اطلاعات موردنظر پیدا نشد.",
				Instance: "/resource",
			},
		},
		{
			name:           "weighted English preference",
			acceptLanguage: "fa;q=0.2, en;q=0.8",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			},
		},
		{
			name:           "unsupported language defaults to English",
			acceptLanguage: "de-DE",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			},
		},
		{
			name:           "malformed language defaults to English",
			acceptLanguage: "fa;q=invalid",
			err:            domainErrors.ErrNotFound,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			},
		},
		{
			name:           "validation error",
			acceptLanguage: "en-US",
			err:            session.ErrInvalidSessionSelection,
			wantLanguage:   "en",
			wantProblem: Problem{
				Type:     "/errors/validation",
				Title:    "Bad Request",
				Status:   http.StatusBadRequest,
				Detail:   "Invalid request payload",
				Instance: "/resource",
			},
		},
		{
			name:           "unknown internal error remains safe",
			acceptLanguage: "fa-IR",
			err:            errors.New("postgres password=secret"),
			wantLanguage:   "fa",
			wantProblem: Problem{
				Type:     "/errors/internal",
				Title:    "خطای سرور",
				Status:   http.StatusInternalServerError,
				Detail:   "مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
				Instance: "/resource",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := renderMappedError(t, test.err, test.acceptLanguage)
			assertProblemHTTPContract(t, recorder, test.wantLanguage, "Accept-Language", test.wantProblem)
		})
	}
}

func TestWriteProblemMergesExistingVaryValues(t *testing.T) {
	tests := []struct {
		name         string
		existingVary []string
		wantVary     string
	}{
		{
			name:         "preserves unrelated values",
			existingVary: []string{"Origin", "Accept-Encoding"},
			wantVary:     "Origin, Accept-Encoding, Accept-Language",
		},
		{
			name:         "canonicalizes and removes duplicate tokens",
			existingVary: []string{"Origin, accept-language", "Accept-Encoding, ACCEPT-LANGUAGE", "Origin"},
			wantVary:     "Origin, Accept-Language, Accept-Encoding",
		},
		{
			name:         "preserves wildcard that already varies on every request field",
			existingVary: []string{"*"},
			wantVary:     "*",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := make(http.Header)
			headers["Vary"] = append([]string(nil), test.existingVary...)
			recorder := renderDescriptorWithHeaders(
				t,
				MapError(domainErrors.ErrNotFound),
				"en",
				headers,
			)
			assertProblemHTTPContract(t, recorder, "en", test.wantVary, Problem{
				Type:     "/errors/not-found",
				Title:    "Not Found",
				Status:   http.StatusNotFound,
				Detail:   "The requested resource was not found.",
				Instance: "/resource",
			})
		})
	}
}

func TestWriteProblemRepeatedCallsDoNotDuplicateVary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/resource", func(c *gin.Context) {
		descriptor := MapError(domainErrors.ErrNotFound)
		WriteProblem(c, descriptor)
		WriteProblem(c, descriptor)
	})

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Vary"), "Accept-Language"; got != want {
		t.Fatalf("Vary = %q, want %q", got, want)
	}
}

func TestSuccessResponseHTTPHeadersRemainUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/resource", func(c *gin.Context) {
		OK(c, gin.H{"status": "ok"})
	})

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Accept-Language", "fa-IR")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	if got := recorder.Header().Get("Content-Language"); got != "" {
		t.Fatalf("Content-Language = %q, want no localization header", got)
	}
	if got := recorder.Header().Get("Vary"); got != "" {
		t.Fatalf("Vary = %q, want no localization header", got)
	}
	assertProblemJSON(t, recorder.Body.Bytes(), map[string]any{
		"data": map[string]any{"status": "ok"},
	})
}

func TestWriteProblemChangesOnlyLocalizedFields(t *testing.T) {
	for _, test := range mappedProblemCases {
		t.Run(test.name, func(t *testing.T) {
			english := decodeProblem(t, renderMappedError(t, test.err, "en-US").Body.Bytes())
			persian := decodeProblem(t, renderMappedError(t, test.err, "fa-IR").Body.Bytes())

			if english.Type != persian.Type ||
				english.Status != persian.Status ||
				english.Instance != persian.Instance ||
				!reflect.DeepEqual(english.Metadata, persian.Metadata) {
				t.Fatalf("stable fields differ: English %#v, Persian %#v", english, persian)
			}
			if english.Title != test.title || english.Detail != test.detail {
				t.Fatalf("English problem = %#v", english)
			}
		})
	}
}

func TestPersianNotFoundProblemContract(t *testing.T) {
	problem := decodeProblem(t, renderMappedError(t, domainErrors.ErrNotFound, "fa-IR").Body.Bytes())
	if problem.Title != "پیدا نشد" || problem.Detail != "اطلاعات موردنظر پیدا نشد." {
		t.Fatalf("Persian problem = %#v", problem)
	}
}

func TestMissingPersianTranslationFallsBackToEnglish(t *testing.T) {
	persianCatalog := make(translationCatalog, len(problemLocalizer.catalogs[localePersian]))
	for key, message := range problemLocalizer.catalogs[localePersian] {
		persianCatalog[key] = message
	}
	delete(persianCatalog, detailConflict)

	localizerWithMissingTranslation := localizer{catalogs: map[locale]translationCatalog{
		localeEnglish: problemLocalizer.catalogs[localeEnglish],
		localePersian: persianCatalog,
	}}
	if got, want := localizerWithMissingTranslation.translate(localePersian, detailConflict, nil), "The request conflicts with current state."; got != want {
		t.Fatalf("fallback translation = %q, want %q", got, want)
	}
}

func TestParameterizedProblemTranslations(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		descriptor ProblemDescriptor
		wantTitle  string
		wantDetail string
	}{
		{
			name:   "English invalid role",
			header: "en-US",
			descriptor: ProblemDescriptor{
				Type:      "/errors/validation",
				Status:    http.StatusBadRequest,
				TitleKey:  titleBadRequest,
				DetailKey: DetailInvalidUserRole,
				Params:    TranslationParams{"Role": "auditor"},
			},
			wantTitle:  "Bad Request",
			wantDetail: "invalid user role: auditor",
		},
		{
			name:   "Persian rate limit",
			header: "fa-IR",
			descriptor: ProblemDescriptor{
				Type:      "/errors/rate-limit",
				Status:    http.StatusTooManyRequests,
				TitleKey:  titleTooManyRequests,
				DetailKey: DetailRateLimitExceeded,
				Params:    TranslationParams{"RetryAfter": "2s"},
				Metadata:  map[string]any{"retry_after": "2s"},
			},
			wantTitle:  "تعداد درخواست\u200cها بیش از حد مجاز است",
			wantDetail: "تعداد درخواست\u200cهای شما بیش از حد مجاز است. لطفاً پس از 2s دوباره تلاش کنید.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			problem := decodeProblem(t, renderDescriptor(t, test.descriptor, test.header).Body.Bytes())
			if problem.Title != test.wantTitle || problem.Detail != test.wantDetail {
				t.Fatalf("localized problem = %#v", problem)
			}
		})
	}
}

func TestUnknownErrorsNeverLeakInternalDetailsOrTranslationKeys(t *testing.T) {
	internalDetail := "pq: password=secret redis=10.0.0.5 jwt-private-key"
	for _, header := range []string{"en", "fa-IR"} {
		t.Run(header, func(t *testing.T) {
			recorder := renderMappedError(t, errors.New(internalDetail), header)
			body := recorder.Body.String()
			for _, forbidden := range []string{internalDetail, "password=secret", "detail.", "title."} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("public problem leaked %q: %s", forbidden, body)
				}
			}
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestEmailConflictPreservesExactLocalizedProblemContracts(t *testing.T) {
	postgresErr := &pq.Error{
		Code:       "23505",
		Constraint: "users_email_key",
		Message:    "duplicate key value violates unique constraint users_email_key",
		Detail:     "Key (email)=(private@example.com) already exists.",
	}
	err := fmt.Errorf(
		"create user: %w",
		fmt.Errorf("%w: %w", domainErrors.ErrEmailAlreadyExists, postgresErr),
	)

	for _, test := range []struct {
		name         string
		header       string
		wantLanguage string
		wantTitle    string
		wantDetail   string
	}{
		{
			name:         "English",
			header:       "en-US",
			wantLanguage: "en",
			wantTitle:    "Conflict",
			wantDetail:   "This email is already registered.",
		},
		{
			name:         "Persian",
			header:       "fa-IR",
			wantLanguage: "fa",
			wantTitle:    "امکان انجام درخواست وجود ندارد",
			wantDetail:   "قبلاً حسابی با این ایمیل ثبت شده است.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := renderMappedError(t, err, test.header)
			assertProblemHTTPContract(t, recorder, test.wantLanguage, "Accept-Language", Problem{
				Type:     "/errors/email-already-exists",
				Title:    test.wantTitle,
				Status:   http.StatusConflict,
				Detail:   test.wantDetail,
				Instance: "/resource",
			})
			if strings.Contains(recorder.Body.String(), "private@example.com") ||
				strings.Contains(recorder.Body.String(), "users_email_key") {
				t.Fatalf("localized conflict leaked PostgreSQL details: %s", recorder.Body.String())
			}
		})
	}
}

func TestUnclassifiedPostgresErrorsRemainSafeLocalizedInternalProblems(t *testing.T) {
	err := &pq.Error{
		Code:       "23505",
		Constraint: "users_pkey",
		Message:    "duplicate key value exposes database detail",
		Detail:     "Key (id)=(private-user-id) already exists.",
	}

	for _, test := range []struct {
		name         string
		header       string
		wantLanguage string
		wantTitle    string
		wantDetail   string
	}{
		{
			name:         "English",
			header:       "en",
			wantLanguage: "en",
			wantTitle:    "Internal Server Error",
			wantDetail:   "An unexpected error occurred.",
		},
		{
			name:         "Persian",
			header:       "fa",
			wantLanguage: "fa",
			wantTitle:    "خطای سرور",
			wantDetail:   "مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := renderMappedError(t, err, test.header)
			assertProblemHTTPContract(t, recorder, test.wantLanguage, "Accept-Language", Problem{
				Type:     "/errors/internal",
				Title:    test.wantTitle,
				Status:   http.StatusInternalServerError,
				Detail:   test.wantDetail,
				Instance: "/resource",
			})
			if strings.Contains(recorder.Body.String(), "private-user-id") ||
				strings.Contains(recorder.Body.String(), "users_pkey") {
				t.Fatalf("internal problem leaked PostgreSQL details: %s", recorder.Body.String())
			}
		})
	}
}

func TestUserIDLookupNotFoundPreservesExactLocalizedProblemContracts(t *testing.T) {
	err := fmt.Errorf(
		"get user: %w",
		fmt.Errorf("%w: %w", domainErrors.ErrUserNotFound, sql.ErrNoRows),
	)

	for _, test := range []struct {
		name         string
		header       string
		wantLanguage string
		wantTitle    string
		wantDetail   string
	}{
		{
			name:         "English",
			header:       "en-US",
			wantLanguage: "en",
			wantTitle:    "Not Found",
			wantDetail:   "The requested resource was not found.",
		},
		{
			name:         "Persian",
			header:       "fa-IR",
			wantLanguage: "fa",
			wantTitle:    "پیدا نشد",
			wantDetail:   "اطلاعات موردنظر پیدا نشد.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := renderMappedError(t, err, test.header)
			assertProblemHTTPContract(t, recorder, test.wantLanguage, "Accept-Language", Problem{
				Type:     "/errors/not-found",
				Title:    test.wantTitle,
				Status:   http.StatusNotFound,
				Detail:   test.wantDetail,
				Instance: "/resource",
			})
			if strings.Contains(recorder.Body.String(), sql.ErrNoRows.Error()) {
				t.Fatalf("localized not-found problem leaked SQL details: %s", recorder.Body.String())
			}
		})
	}
}

func TestUnknownUserIDLookupFailureRemainsSafeLocalizedInternalProblem(t *testing.T) {
	err := fmt.Errorf("find user by ID: %w", errors.New("postgres host=db.internal password=secret"))

	for _, test := range []struct {
		name         string
		header       string
		wantLanguage string
		wantTitle    string
		wantDetail   string
	}{
		{
			name:         "English",
			header:       "en",
			wantLanguage: "en",
			wantTitle:    "Internal Server Error",
			wantDetail:   "An unexpected error occurred.",
		},
		{
			name:         "Persian",
			header:       "fa",
			wantLanguage: "fa",
			wantTitle:    "خطای سرور",
			wantDetail:   "مشکلی پیش آمد. لطفاً دوباره تلاش کنید.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := renderMappedError(t, err, test.header)
			assertProblemHTTPContract(t, recorder, test.wantLanguage, "Accept-Language", Problem{
				Type:     "/errors/internal",
				Title:    test.wantTitle,
				Status:   http.StatusInternalServerError,
				Detail:   test.wantDetail,
				Instance: "/resource",
			})
			if strings.Contains(recorder.Body.String(), "db.internal") ||
				strings.Contains(recorder.Body.String(), "password=secret") {
				t.Fatalf("localized internal problem leaked PostgreSQL details: %s", recorder.Body.String())
			}
		})
	}
}

func TestUnknownTranslationKeyUsesSafeEnglishFallback(t *testing.T) {
	unknownKey := TranslationKey("detail.postgres.internal")
	recorder := renderDescriptor(t, ProblemDescriptor{
		Type:      "/errors/internal",
		Status:    http.StatusInternalServerError,
		TitleKey:  unknownKey,
		DetailKey: unknownKey,
	}, "fa")

	problem := decodeProblem(t, recorder.Body.Bytes())
	if problem.Title != "An unexpected error occurred." || problem.Detail != "An unexpected error occurred." {
		t.Fatalf("safe fallback problem = %#v", problem)
	}
	if strings.Contains(recorder.Body.String(), string(unknownKey)) {
		t.Fatalf("response leaked translation key: %s", recorder.Body.String())
	}
}

func translationParamsForTest(key TranslationKey) TranslationParams {
	switch key {
	case DetailInvalidUserRole:
		return TranslationParams{"Role": "auditor"}
	case DetailRateLimitExceeded:
		return TranslationParams{"RetryAfter": "2s"}
	default:
		return nil
	}
}

func renderMappedError(t *testing.T, err error, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	return renderDescriptor(t, MapError(err), acceptLanguage)
}

func renderDescriptor(t *testing.T, descriptor ProblemDescriptor, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	return renderDescriptorWithHeaders(t, descriptor, acceptLanguage, nil)
}

func renderDescriptorWithHeaders(
	t *testing.T,
	descriptor ProblemDescriptor,
	acceptLanguage string,
	headers http.Header,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/resource", func(c *gin.Context) {
		for name, values := range headers {
			for _, value := range values {
				c.Writer.Header().Add(name, value)
			}
		}
		WriteProblem(c, descriptor)
	})

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertProblemHTTPContract(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantLanguage string,
	wantVary string,
	wantProblem Problem,
) {
	t.Helper()

	if got, want := recorder.Header().Get("Content-Type"), "application/problem+json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got := recorder.Header().Get("Content-Language"); got != wantLanguage {
		t.Errorf("Content-Language = %q, want %q", got, wantLanguage)
	}
	if got := recorder.Header().Get("Vary"); got != wantVary {
		t.Errorf("Vary = %q, want %q", got, wantVary)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want no new caching policy", got)
	}

	if got := decodeProblem(t, recorder.Body.Bytes()); !reflect.DeepEqual(got, wantProblem) {
		t.Fatalf("Problem = %#v, want %#v", got, wantProblem)
	}
}

func decodeProblem(t *testing.T, body []byte) Problem {
	t.Helper()
	var problem Problem
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatalf("decode problem JSON: %v; body = %s", err, body)
	}
	return problem
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
