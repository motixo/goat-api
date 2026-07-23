package user

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserEmailWriteUseCasesPreserveSemanticConflictAndPostgresCause(t *testing.T) {
	postgresCause := errors.New("postgres email unique violation")
	persistenceErr := fmt.Errorf("%w: %w", domainErrors.ErrEmailAlreadyExists, postgresCause)

	for _, test := range []struct {
		name string
		run  func(UseCase) error
	}{
		{
			name: "create user",
			run: func(usecase UseCase) error {
				_, err := usecase.CreateUser(context.Background(), CreateInput{
					Email:    "duplicate@example.com",
					Password: "Password1!",
					Status:   valueobject.StatusActive,
					Role:     valueobject.RoleClient,
				})
				return err
			},
		},
		{
			name: "full update",
			run: func(usecase UseCase) error {
				return usecase.UpdateUser(context.Background(), UpdateInput{
					UserID:   "user-1",
					Email:    "duplicate@example.com",
					Password: "Password1!",
					Status:   valueobject.StatusActive,
					Role:     valueobject.RoleClient,
				})
			},
		},
		{
			name: "change email",
			run: func(usecase UseCase) error {
				return usecase.ChangeEmail(context.Background(), UpdateEmailInput{
					UserID: "user-1",
					Email:  "duplicate@example.com",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &emailConflictUserRepository{
				createErr: persistenceErr,
				updateErr: persistenceErr,
			}
			usecase := NewUsecase(
				repo,
				emailConflictPasswordHasher{},
				discardUserLogger{},
				nil,
				nil,
				nil,
				nil,
			)

			err := test.run(usecase)

			if !errors.Is(err, persistenceErr) {
				t.Fatalf("use case error = %v, want repository error preserved", err)
			}
			if !errors.Is(err, domainErrors.ErrEmailAlreadyExists) {
				t.Fatalf("errors.Is(ErrEmailAlreadyExists) = false; error = %v", err)
			}
			if !errors.Is(err, postgresCause) {
				t.Fatalf("application error did not preserve PostgreSQL cause: %v", err)
			}
		})
	}
}

type emailConflictUserRepository struct {
	repository.UserRepository
	createErr error
	updateErr error
}

func (r *emailConflictUserRepository) Create(context.Context, *entity.User) error {
	return r.createErr
}

func (r *emailConflictUserRepository) Update(context.Context, *entity.User) error {
	return r.updateErr
}

type emailConflictPasswordHasher struct {
	service.PasswordHasher
}

func (emailConflictPasswordHasher) Hash(context.Context, string) (valueobject.Password, error) {
	return valueobject.PasswordFromHash("$argon2id$email-conflict-test"), nil
}
