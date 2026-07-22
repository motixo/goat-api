package user

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestGetUsersListAppliesAuthorizationScopeBeforeRepositoryPagination(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	repo := &recordingUserListRepository{
		users: []*entity.User{{
			ID:        "user-1",
			Email:     "client@example.com",
			Role:      valueobject.RoleClient,
			Status:    valueobject.StatusActive,
			CreatedAt: createdAt,
		}},
		total: 1,
	}
	cache := &fixedUserRoleCache{role: valueobject.RoleOperator}
	usecase := NewUsecase(repo, nil, discardUserListLogger{}, nil, cache, nil)

	output, total, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID: "operator-1",
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

	if !repo.called {
		t.Fatal("repository List was not called")
	}
	if repo.offset != 20 || repo.limit != 10 {
		t.Fatalf("pagination = offset %d, limit %d; want offset 20, limit 10", repo.offset, repo.limit)
	}
	wantFilter := repository.UserListFilter{
		Roles:    []valueobject.UserRole{valueobject.RoleClient},
		Statuses: []valueobject.UserStatus{valueobject.StatusActive},
		Search:   "example",
	}
	if !reflect.DeepEqual(repo.filter, wantFilter) {
		t.Fatalf("repository filter = %#v, want %#v", repo.filter, wantFilter)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	wantOutput := []UserOutput{{
		ID:        "user-1",
		Email:     "client@example.com",
		Role:      "client",
		Status:    "active",
		CreatedAt: createdAt,
	}}
	if !reflect.DeepEqual(output, wantOutput) {
		t.Fatalf("output = %#v, want %#v", output, wantOutput)
	}
}

func TestGetUsersListReturnsEmptyBeforeRepositoryWhenActorHasNoVisibleRoles(t *testing.T) {
	repo := &recordingUserListRepository{}
	cache := &fixedUserRoleCache{role: valueobject.RoleClient}
	usecase := NewUsecase(repo, nil, discardUserListLogger{}, nil, cache, nil)

	output, total, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID: "client-1",
		Offset:  10,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}
	if repo.called {
		t.Fatal("repository List was called with an empty authorization scope")
	}
	if len(output) != 0 || total != 0 {
		t.Fatalf("result = (%#v, %d), want empty output and total 0", output, total)
	}
}

func TestGetUsersListReturnsEmptyBeforeRepositoryWhenRequestedRolesAreOutsideScope(t *testing.T) {
	repo := &recordingUserListRepository{}
	cache := &fixedUserRoleCache{role: valueobject.RoleOperator}
	usecase := NewUsecase(repo, nil, discardUserListLogger{}, nil, cache, nil)

	output, total, err := usecase.GetUserslist(context.Background(), GetListInput{
		ActorID: "operator-1",
		Filter: ListFilter{
			Roles: []valueobject.UserRole{valueobject.RoleAdmin},
		},
		Offset: 10,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetUserslist() error = %v", err)
	}
	if repo.called {
		t.Fatal("repository List was called after the requested role scope became empty")
	}
	if len(output) != 0 || total != 0 {
		t.Fatalf("result = (%#v, %d), want empty output and total 0", output, total)
	}
}

type recordingUserListRepository struct {
	repository.UserRepository
	called bool
	offset int
	limit  int
	filter repository.UserListFilter
	users  []*entity.User
	total  int64
	err    error
}

func (r *recordingUserListRepository) List(_ context.Context, offset, limit int, filter repository.UserListFilter) ([]*entity.User, int64, error) {
	r.called = true
	r.offset = offset
	r.limit = limit
	r.filter = filter
	return r.users, r.total, r.err
}

type fixedUserRoleCache struct {
	service.UserCacheService
	role valueobject.UserRole
	err  error
}

func (c *fixedUserRoleCache) GetUserRole(context.Context, string) (valueobject.UserRole, error) {
	return c.role, c.err
}

type discardUserListLogger struct{}

func (discardUserListLogger) Info(string, ...any)  {}
func (discardUserListLogger) Error(string, ...any) {}
func (discardUserListLogger) Warn(string, ...any)  {}
func (discardUserListLogger) Debug(string, ...any) {}
func (discardUserListLogger) Panic(string, ...any) {}
