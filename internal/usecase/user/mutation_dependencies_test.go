package user

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserAuthorizationMutationsUseOnlyAuthoritativeAdapters(t *testing.T) {
	tests := []struct {
		name      string
		change    func(UseCase) error
		wantCalls []string
	}{
		{
			name: "role change",
			change: func(usecase UseCase) error {
				return usecase.ChangeRole(context.Background(), UpdateRoleInput{
					UserID: "user-1",
					Role:   valueobject.RoleOperator,
				})
			},
			wantCalls: []string{"user.update"},
		},
		{
			name: "status change",
			change: func(usecase UseCase) error {
				return usecase.ChangeStatus(context.Background(), UpdateStatusInput{
					UserID:    "user-1",
					ActorID:   "admin-1",
					ActorRole: valueobject.RoleAdmin,
					Status:    valueobject.StatusSuspended,
				})
			},
			wantCalls: []string{
				"user.find_by_id",
				"session.block_delete_all",
				"user.update_status",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := &mutationCallRecorder{}
			usecase := NewUsecase(
				&mutationUserRepository{
					recorder: recorder,
					user: &entity.User{
						ID:     "user-1",
						Role:   valueobject.RoleClient,
						Status: valueobject.StatusActive,
					},
				},
				nil,
				mutationLogger{},
				&mutationSessionRepository{recorder: recorder},
				nil,
			)

			if err := test.change(usecase); err != nil {
				t.Fatalf("change error = %v", err)
			}
			if got := recorder.calls; !reflect.DeepEqual(got, test.wantCalls) {
				t.Fatalf("calls = %v, want %v", got, test.wantCalls)
			}
		})
	}
}

func TestUserAuthorizationMutationsPropagatePersistenceFailures(t *testing.T) {
	tests := []struct {
		name      string
		change    func(UseCase) error
		wantCalls []string
	}{
		{
			name: "role change",
			change: func(usecase UseCase) error {
				return usecase.ChangeRole(context.Background(), UpdateRoleInput{
					UserID: "user-1",
					Role:   valueobject.RoleOperator,
				})
			},
			wantCalls: []string{"user.update"},
		},
		{
			name: "status change",
			change: func(usecase UseCase) error {
				return usecase.ChangeStatus(context.Background(), UpdateStatusInput{
					UserID:    "user-1",
					ActorID:   "admin-1",
					ActorRole: valueobject.RoleAdmin,
					Status:    valueobject.StatusSuspended,
				})
			},
			wantCalls: []string{
				"user.find_by_id",
				"session.block_delete_all",
				"user.update_status",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			persistenceErr := errors.New("postgres update failed")
			recorder := &mutationCallRecorder{}
			usecase := NewUsecase(
				&mutationUserRepository{
					recorder: recorder,
					err:      persistenceErr,
					user: &entity.User{
						ID:     "user-1",
						Role:   valueobject.RoleClient,
						Status: valueobject.StatusActive,
					},
				},
				nil,
				mutationLogger{},
				&mutationSessionRepository{recorder: recorder},
				nil,
			)

			err := test.change(usecase)
			if !errors.Is(err, persistenceErr) {
				t.Fatalf("change error = %v, want persistence error", err)
			}
			if got := recorder.calls; !reflect.DeepEqual(got, test.wantCalls) {
				t.Fatalf("calls = %v, want %v", got, test.wantCalls)
			}
		})
	}
}

type mutationCallRecorder struct {
	calls []string
}

func (r *mutationCallRecorder) record(call string) {
	r.calls = append(r.calls, call)
}

type mutationUserRepository struct {
	repository.UserRepository
	recorder *mutationCallRecorder
	err      error
	user     *entity.User
}

func (r *mutationUserRepository) FindByID(context.Context, string) (*entity.User, error) {
	r.recorder.record("user.find_by_id")
	return r.user, nil
}

func (r *mutationUserRepository) Update(context.Context, *entity.User) error {
	r.recorder.record("user.update")
	return r.err
}

func (r *mutationUserRepository) UpdateStatus(
	_ context.Context,
	_ string,
	_ valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	r.recorder.record("user.update_status")
	return repository.UserStatusUpdateResult{
		Outcome:       repository.UserStatusUpdateApplied,
		CurrentStatus: requested,
	}, r.err
}

type mutationSessionRepository struct {
	repository.SessionRepository
	recorder *mutationCallRecorder
}

func (r *mutationSessionRepository) BlockAndDeleteAllByUser(
	context.Context,
	string,
) error {
	r.recorder.record("session.block_delete_all")
	return nil
}

type mutationLogger struct{}

func (mutationLogger) Info(string, ...any)  {}
func (mutationLogger) Error(string, ...any) {}
func (mutationLogger) Warn(string, ...any)  {}
func (mutationLogger) Debug(string, ...any) {}
func (mutationLogger) Panic(string, ...any) {}
