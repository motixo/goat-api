package user

import (
	"context"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestGetUserUsesCredentialFreeDetailReader(t *testing.T) {
	createdAt := time.Date(2026, time.July, 27, 12, 30, 0, 0, time.UTC)
	reader := &recordingUserDetailReader{detail: UserDetail{
		ID:        "11111111-1111-4111-8111-111111111111",
		Email:     "detail@example.com",
		Role:      valueobject.RoleOperator,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
	}}
	fullRepository := &panicOnDetailFullUserRepository{}
	usecase := NewUsecase(Dependencies{
		UserRepository: fullRepository,
		DetailReader:   reader,
		Logger:         discardUserLogger{},
	})

	got, err := usecase.GetUser(context.Background(), reader.detail.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got != reader.detail {
		t.Fatalf("GetUser() = %#v, want %#v", got, reader.detail)
	}
	if reader.calls != 1 || reader.gotID != reader.detail.ID {
		t.Fatalf("FindDetailByID calls/id = %d/%q, want 1/%q", reader.calls, reader.gotID, reader.detail.ID)
	}
}

func TestGetUserPreservesReadableInactiveAndSuspendedDetails(t *testing.T) {
	for _, status := range []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusSuspended,
	} {
		t.Run(status.String(), func(t *testing.T) {
			reader := &recordingUserDetailReader{detail: UserDetail{
				ID:     "33333333-3333-4333-8333-333333333333",
				Status: status,
			}}
			usecase := NewUsecase(Dependencies{
				UserRepository: &panicOnDetailFullUserRepository{},
				DetailReader:   reader,
				Logger:         discardUserLogger{},
			})

			got, err := usecase.GetUser(context.Background(), reader.detail.ID)
			if err != nil {
				t.Fatalf("GetUser() error = %v", err)
			}
			if got.Status != status {
				t.Fatalf("GetUser() status = %s, want %s", got.Status, status)
			}
		})
	}
}

type recordingUserDetailReader struct {
	detail UserDetail
	err    error
	calls  int
	gotID  string
}

func (r *recordingUserDetailReader) FindDetailByID(
	_ context.Context,
	id string,
) (UserDetail, error) {
	r.calls++
	r.gotID = id
	return r.detail, r.err
}

type panicOnDetailFullUserRepository struct {
	repository.UserRepository
}

func (*panicOnDetailFullUserRepository) FindByID(context.Context, string) (*entity.User, error) {
	panic("read-only User detail loaded the full aggregate")
}
