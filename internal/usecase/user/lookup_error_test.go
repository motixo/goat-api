package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserIDLookupUseCasesPreserveSemanticNotFoundAndSQLCause(t *testing.T) {
	translatedErr := fmt.Errorf("%w: %w", domainErrors.ErrUserNotFound, sql.ErrNoRows)

	for _, variant := range []struct {
		name string
		err  error
	}{
		{name: "direct", err: translatedErr},
		{name: "wrapped", err: fmt.Errorf("repository lookup: %w", translatedErr)},
	} {
		for _, operation := range []struct {
			name string
			run  func(UseCase) error
		}{
			{
				name: "get user",
				run: func(usecase UseCase) error {
					_, err := usecase.GetUser(context.Background(), "user-1")
					return err
				},
			},
			{
				name: "change password",
				run: func(usecase UseCase) error {
					return usecase.ChangePassword(context.Background(), UpdatePassInput{
						UserID:      "user-1",
						OldPassword: "OldPassword1!",
						NewPassword: "NewPassword1!",
					})
				},
			},
		} {
			t.Run(operation.name+"/"+variant.name, func(t *testing.T) {
				repo := &lookupUserRepository{findByIDErr: variant.err}
				usecase := NewUsecase(repo, nil, discardUserLogger{}, nil, nil, nil, nil)

				err := operation.run(usecase)

				if !errors.Is(err, domainErrors.ErrUserNotFound) {
					t.Fatalf("errors.Is(ErrUserNotFound) = false; error = %v", err)
				}
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("errors.Is(sql.ErrNoRows) = false; error = %v", err)
				}
				if repo.findByIDCalls != 1 {
					t.Fatalf("FindByID calls = %d, want 1", repo.findByIDCalls)
				}
			})
		}
	}
}

func TestUserIDLookupUseCasesPreserveUnknownFailures(t *testing.T) {
	lookupErr := errors.New("postgres connection unavailable")

	for _, operation := range []struct {
		name string
		run  func(UseCase) error
	}{
		{
			name: "get user",
			run: func(usecase UseCase) error {
				_, err := usecase.GetUser(context.Background(), "user-1")
				return err
			},
		},
		{
			name: "change password",
			run: func(usecase UseCase) error {
				return usecase.ChangePassword(context.Background(), UpdatePassInput{
					UserID:      "user-1",
					OldPassword: "OldPassword1!",
					NewPassword: "NewPassword1!",
				})
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			repo := &lookupUserRepository{findByIDErr: lookupErr}
			usecase := NewUsecase(repo, nil, discardUserLogger{}, nil, nil, nil, nil)

			err := operation.run(usecase)

			if !errors.Is(err, lookupErr) {
				t.Fatalf("use case error = %v, want unknown lookup cause", err)
			}
			if errors.Is(err, domainErrors.ErrUserNotFound) {
				t.Fatalf("unknown lookup error was misclassified as ErrUserNotFound: %v", err)
			}
		})
	}
}

func TestGetUserSuccessfulLookupRemainsUnchanged(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 18, 0, 0, 0, time.UTC)
	repo := &lookupUserRepository{user: &entity.User{
		ID:        "user-1",
		Email:     "user@example.com",
		Role:      valueobject.RoleClient,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
	}}
	usecase := NewUsecase(repo, nil, discardUserLogger{}, nil, nil, nil, nil)

	output, err := usecase.GetUser(context.Background(), "user-1")

	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	want := UserOutput{
		ID:        "user-1",
		Email:     "user@example.com",
		Role:      "client",
		Status:    "active",
		CreatedAt: createdAt,
	}
	if output != want {
		t.Fatalf("GetUser() output = %#v, want %#v", output, want)
	}
}

type lookupUserRepository struct {
	repository.UserRepository
	user          *entity.User
	findByIDErr   error
	findByIDCalls int
}

func (r *lookupUserRepository) FindByID(context.Context, string) (*entity.User, error) {
	r.findByIDCalls++
	return r.user, r.findByIDErr
}
