package permission

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPermissionMutationsUseOnlyAuthoritativeRepository(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(UseCase) error
		wantCalls []string
	}{
		{
			name: "create",
			mutate: func(usecase UseCase) error {
				_, err := usecase.Create(context.Background(), CreateInput{
					Role:   valueobject.RoleAdmin,
					Action: valueobject.PermFullAccess,
				})
				return err
			},
			wantCalls: []string{"permission.create"},
		},
		{
			name: "delete",
			mutate: func(usecase UseCase) error {
				return usecase.Delete(context.Background(), "permission-1")
			},
			wantCalls: []string{"permission.delete"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := &permissionMutationRecorder{}
			usecase := NewUsecase(
				&permissionMutationRepository{recorder: recorder},
				permissionMutationLogger{},
			)

			if err := test.mutate(usecase); err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			if got := recorder.calls; !reflect.DeepEqual(got, test.wantCalls) {
				t.Fatalf("calls = %v, want %v", got, test.wantCalls)
			}
		})
	}
}

func TestPermissionMutationsPropagatePersistenceFailures(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(UseCase) error
		configure func(*permissionMutationRepository, error)
		wantCall  string
	}{
		{
			name: "create",
			mutate: func(usecase UseCase) error {
				_, err := usecase.Create(context.Background(), CreateInput{
					Role:   valueobject.RoleAdmin,
					Action: valueobject.PermFullAccess,
				})
				return err
			},
			configure: func(repo *permissionMutationRepository, err error) {
				repo.createErr = err
			},
			wantCall: "permission.create",
		},
		{
			name: "delete",
			mutate: func(usecase UseCase) error {
				return usecase.Delete(context.Background(), "permission-1")
			},
			configure: func(repo *permissionMutationRepository, err error) {
				repo.deleteErr = err
			},
			wantCall: "permission.delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			persistenceErr := errors.New("postgres write failed")
			recorder := &permissionMutationRecorder{}
			repo := &permissionMutationRepository{recorder: recorder}
			test.configure(repo, persistenceErr)
			usecase := NewUsecase(repo, permissionMutationLogger{})

			err := test.mutate(usecase)
			if !errors.Is(err, persistenceErr) {
				t.Fatalf("mutation error = %v, want persistence error", err)
			}
			if got, want := recorder.calls, []string{test.wantCall}; !reflect.DeepEqual(got, want) {
				t.Fatalf("calls = %v, want %v", got, want)
			}
		})
	}
}

type permissionMutationRecorder struct {
	calls []string
}

func (r *permissionMutationRecorder) record(call string) {
	r.calls = append(r.calls, call)
}

type permissionMutationRepository struct {
	repository.PermissionRepository
	recorder  *permissionMutationRecorder
	createErr error
	deleteErr error
}

func (r *permissionMutationRepository) Create(context.Context, *entity.Permission) error {
	r.recorder.record("permission.create")
	return r.createErr
}

func (r *permissionMutationRepository) Delete(context.Context, string) error {
	r.recorder.record("permission.delete")
	return r.deleteErr
}

type permissionMutationLogger struct{}

func (permissionMutationLogger) Info(string, ...any)  {}
func (permissionMutationLogger) Error(string, ...any) {}
func (permissionMutationLogger) Warn(string, ...any)  {}
func (permissionMutationLogger) Debug(string, ...any) {}
func (permissionMutationLogger) Panic(string, ...any) {}
