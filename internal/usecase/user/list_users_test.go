package user

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestGetUsersListAppliesAuthorizationScopeBeforeRepositoryPagination(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	reader := &recordingUserListReader{
		result: UserListResult{
			Items: []UserListItem{{
				ID:        "user-1",
				Email:     "client@example.com",
				Role:      valueobject.RoleClient,
				Status:    valueobject.StatusActive,
				CreatedAt: createdAt,
			}},
			Total: 1,
		},
	}
	usecase := NewUsecase(Dependencies{ListReader: reader, Logger: discardUserListLogger{}})

	result, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID:   "operator-1",
		ActorRole: valueobject.RoleOperator,
		Filter: ListFilter{
			Roles:    []valueobject.UserRole{valueobject.RoleAdmin, valueobject.RoleClient},
			Statuses: []valueobject.UserStatus{valueobject.StatusActive},
			Search:   "example",
		},
		Offset: 20,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}

	if !reader.called {
		t.Fatal("repository List was not called")
	}
	if reader.offset != 20 || reader.limit != 10 {
		t.Fatalf("pagination = offset %d, limit %d; want offset 20, limit 10", reader.offset, reader.limit)
	}
	wantCriteria := UserListCriteria{
		Roles:    []valueobject.UserRole{valueobject.RoleClient},
		Statuses: []valueobject.UserStatus{valueobject.StatusActive},
		Search:   "example",
	}
	if !reflect.DeepEqual(reader.criteria, wantCriteria) {
		t.Fatalf("reader criteria = %#v, want %#v", reader.criteria, wantCriteria)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	wantItems := []UserListItem{{
		ID:        "user-1",
		Email:     "client@example.com",
		Role:      valueobject.RoleClient,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
	}}
	if !reflect.DeepEqual(result.Items, wantItems) {
		t.Fatalf("items = %#v, want %#v", result.Items, wantItems)
	}
}

func TestGetUsersListReturnsEmptyBeforeRepositoryWhenActorHasNoVisibleRoles(t *testing.T) {
	reader := &recordingUserListReader{}
	usecase := NewUsecase(Dependencies{ListReader: reader, Logger: discardUserListLogger{}})

	result, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID:   "client-1",
		ActorRole: valueobject.RoleClient,
		Offset:    10,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}
	if reader.called {
		t.Fatal("repository List was called with an empty authorization scope")
	}
	if len(result.Items) != 0 || result.Total != 0 {
		t.Fatalf("result = %#v, want empty items and total 0", result)
	}
}

func TestGetUsersListReturnsEmptyBeforeRepositoryWhenRequestedRolesAreOutsideScope(t *testing.T) {
	reader := &recordingUserListReader{}
	usecase := NewUsecase(Dependencies{ListReader: reader, Logger: discardUserListLogger{}})

	result, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID:   "operator-1",
		ActorRole: valueobject.RoleOperator,
		Filter: ListFilter{
			Roles: []valueobject.UserRole{valueobject.RoleAdmin},
		},
		Offset: 10,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}
	if reader.called {
		t.Fatal("repository List was called after the requested role scope became empty")
	}
	if len(result.Items) != 0 || result.Total != 0 {
		t.Fatalf("result = %#v, want empty items and total 0", result)
	}
}

func TestGetUsersListAuthorizesBeforeReturningAnUnmatchableFilter(t *testing.T) {
	reader := &recordingUserListReader{}
	usecase := NewUsecase(Dependencies{ListReader: reader, Logger: discardUserListLogger{}})

	result, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID:   "operator-1",
		ActorRole: valueobject.RoleOperator,
		Filter:    ListFilter{MatchNone: true},
		Offset:    10,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}
	if reader.called {
		t.Fatal("repository List was called for a filter that cannot match a user")
	}
	if len(result.Items) != 0 || result.Total != 0 {
		t.Fatalf("result = %#v, want empty items and total 0", result)
	}
}

type recordingUserListReader struct {
	called   bool
	offset   int
	limit    int
	criteria UserListCriteria
	result   UserListResult
	err      error
}

func (r *recordingUserListReader) ListUsers(
	_ context.Context,
	offset int,
	limit int,
	criteria UserListCriteria,
) (UserListResult, error) {
	r.called = true
	r.offset = offset
	r.limit = limit
	r.criteria = criteria
	return r.result, r.err
}

type discardUserListLogger struct{}

func (discardUserListLogger) Info(string, ...any)  {}
func (discardUserListLogger) Error(string, ...any) {}
func (discardUserListLogger) Warn(string, ...any)  {}
func (discardUserListLogger) Debug(string, ...any) {}
func (discardUserListLogger) Panic(string, ...any) {}
