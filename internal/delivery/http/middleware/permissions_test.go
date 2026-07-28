package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

type recordingAuthorizationUseCase struct {
	authorization.UseCase
	fresh              authorization.Principal
	err                error
	identityCalls      int
	authorizationCalls int
	principal          authorization.Principal
	required           valueobject.Permission
}

func (u *recordingAuthorizationUseCase) AuthorizeFreshIdentity(
	_ context.Context,
	principal authorization.Principal,
) (authorization.Principal, error) {
	u.identityCalls++
	u.principal = principal
	return u.fresh, u.err
}

func (u *recordingAuthorizationUseCase) AuthorizeFresh(
	_ context.Context,
	principal authorization.Principal,
	required valueobject.Permission,
) (authorization.Principal, error) {
	u.authorizationCalls++
	u.principal = principal
	u.required = required
	return u.fresh, u.err
}

func TestPermMiddlewareSnapshotUsesOnlyVerifiedPrincipal(t *testing.T) {
	principal := testPrincipal(t, valueobject.RoleClient, valueobject.PermUserRead)
	authorizationUC := &recordingAuthorizationUseCase{}
	middleware := NewPermMiddleware(authorizationUC)
	nextCalled := false

	recorder := performPermissionRequest(
		t,
		principal,
		middleware.RequireSnapshot(valueobject.PermUserRead),
		func(*gin.Context) {
			nextCalled = true
		},
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if !nextCalled {
		t.Fatal("snapshot-authorized handler was not called")
	}
	if authorizationUC.identityCalls != 0 || authorizationUC.authorizationCalls != 0 {
		t.Fatalf(
			"fresh lookups: identity=%d authorization=%d, want 0",
			authorizationUC.identityCalls,
			authorizationUC.authorizationCalls,
		)
	}
}

func TestPermMiddlewareSnapshotRejectsMissingPermission(t *testing.T) {
	principal := testPrincipal(t, valueobject.RoleClient, valueobject.PermUserRead)
	middleware := NewPermMiddleware(&recordingAuthorizationUseCase{})

	recorder := performPermissionRequest(
		t,
		principal,
		middleware.RequireSnapshot(valueobject.PermUserDelete),
		nil,
	)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	assertMiddlewareProblem(t, recorder.Body.Bytes(), map[string]any{
		"type":     "/errors/forbidden",
		"title":    "Forbidden",
		"status":   float64(http.StatusForbidden),
		"detail":   "insufficient permissions",
		"instance": "/protected",
	})
}

func TestPermMiddlewareFreshPerformsOneAuthoritativeLookupAndReplacesSnapshot(t *testing.T) {
	stale := testPrincipal(t, valueobject.RoleClient, valueobject.PermUserRead)
	fresh := testPrincipal(t, valueobject.RoleAdmin, valueobject.PermFullAccess)
	authorizationUC := &recordingAuthorizationUseCase{fresh: fresh}
	middleware := NewPermMiddleware(authorizationUC)
	var got authorization.Principal

	recorder := performPermissionRequest(
		t,
		stale,
		middleware.RequireFreshAuthorization(valueobject.PermUserDelete),
		func(c *gin.Context) {
			var ok bool
			got, ok = PrincipalFrom(c)
			if !ok {
				t.Fatal("fresh principal was not attached")
			}
		},
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if authorizationUC.authorizationCalls != 1 || authorizationUC.identityCalls != 0 {
		t.Fatalf(
			"authoritative lookups: identity=%d authorization=%d, want 0/1",
			authorizationUC.identityCalls,
			authorizationUC.authorizationCalls,
		)
	}
	if authorizationUC.required != valueobject.PermUserDelete {
		t.Fatalf("required permission = %q, want %q", authorizationUC.required, valueobject.PermUserDelete)
	}
	if got.Role() != valueobject.RoleAdmin || !got.Permissions().Has(valueobject.PermFullAccess) {
		t.Fatalf("request principal was not replaced with fresh state: role=%s permissions=%v", got.Role(), got.Permissions().Values())
	}
}

func TestPermMiddlewareFreshIdentityPerformsOneIdentityLookupWithoutPermissionCheck(
	t *testing.T,
) {
	stale := testPrincipal(t, valueobject.RoleClient, valueobject.PermUserRead)
	fresh := testPrincipal(t, valueobject.RoleClient, valueobject.PermUserRead)
	authorizationUC := &recordingAuthorizationUseCase{fresh: fresh}
	middleware := NewPermMiddleware(authorizationUC)

	recorder := performPermissionRequest(
		t,
		stale,
		middleware.FreshIdentity(),
		nil,
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusNoContent,
			recorder.Body.String(),
		)
	}
	if authorizationUC.identityCalls != 1 ||
		authorizationUC.authorizationCalls != 0 {
		t.Fatalf(
			"authoritative lookups: identity=%d authorization=%d, want 1/0",
			authorizationUC.identityCalls,
			authorizationUC.authorizationCalls,
		)
	}
	if authorizationUC.required != "" {
		t.Fatalf(
			"fresh identity required permission = %q, want empty",
			authorizationUC.required,
		)
	}
}

func performPermissionRequest(
	t *testing.T,
	principal authorization.Principal,
	permissionMiddleware gin.HandlerFunc,
	next func(*gin.Context),
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(
		"/protected",
		func(c *gin.Context) {
			SetPrincipal(c, principal)
			c.Next()
		},
		permissionMiddleware,
		func(c *gin.Context) {
			if next != nil {
				next(c)
			}
			c.Status(http.StatusNoContent)
		},
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	return recorder
}

func testPrincipal(
	t testing.TB,
	role valueobject.UserRole,
	permissions ...valueobject.Permission,
) authorization.Principal {
	t.Helper()
	set, err := valueobject.NewPermissionSet(permissions)
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	principal, err := authorization.NewPrincipal("user-1", "session-1", 7, role, set)
	if err != nil {
		t.Fatalf("build principal: %v", err)
	}
	return principal
}

func BenchmarkSnapshotAuthorization(b *testing.B) {
	gin.SetMode(gin.TestMode)
	principal := testPrincipal(b, valueobject.RoleClient, valueobject.PermUserRead)
	middleware := NewPermMiddleware(&recordingAuthorizationUseCase{})
	router := gin.New()
	router.GET(
		"/protected",
		func(c *gin.Context) {
			SetPrincipal(c, principal)
			c.Next()
		},
		middleware.RequireSnapshot(valueobject.PermUserRead),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)

	b.ReportAllocs()
	for b.Loop() {
		router.ServeHTTP(httptest.NewRecorder(), request)
	}
	b.ReportMetric(0, "postgres_auth_queries/op")
}

func BenchmarkFreshIdentityAuthorization(b *testing.B) {
	gin.SetMode(gin.TestMode)
	principal := testPrincipal(b, valueobject.RoleClient, valueobject.PermUserRead)
	authorizationUC := &recordingAuthorizationUseCase{fresh: principal}
	middleware := NewPermMiddleware(authorizationUC)
	router := gin.New()
	router.GET(
		"/protected",
		func(c *gin.Context) {
			SetPrincipal(c, principal)
			c.Next()
		},
		middleware.FreshIdentity(),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		router.ServeHTTP(httptest.NewRecorder(), request)
	}
	b.StopTimer()
	b.ReportMetric(
		float64(authorizationUC.identityCalls)/float64(b.N),
		"postgres_identity_queries/op",
	)
}

func BenchmarkFreshAuthorization(b *testing.B) {
	gin.SetMode(gin.TestMode)
	principal := testPrincipal(b, valueobject.RoleClient, valueobject.PermUserRead)
	authorizationUC := &recordingAuthorizationUseCase{fresh: principal}
	middleware := NewPermMiddleware(authorizationUC)
	router := gin.New()
	router.GET(
		"/protected",
		func(c *gin.Context) {
			SetPrincipal(c, principal)
			c.Next()
		},
		middleware.RequireFreshAuthorization(valueobject.PermUserRead),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		router.ServeHTTP(httptest.NewRecorder(), request)
	}
	b.StopTimer()
	b.ReportMetric(
		float64(authorizationUC.authorizationCalls)/float64(b.N),
		"postgres_auth_queries/op",
	)
}
