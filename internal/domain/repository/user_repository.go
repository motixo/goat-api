package repository

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// UserStatusUpdateOutcome classifies the authoritative result of a status
// compare-and-set without collapsing distinct zero-row outcomes into a bool.
type UserStatusUpdateOutcome uint8

const (
	UserStatusUpdateUnknown UserStatusUpdateOutcome = iota
	UserStatusUpdateApplied
	UserStatusUpdateAlreadyApplied
	UserStatusUpdateConflict
	UserStatusUpdateNotFound
)

// UserStatusUpdateResult reports whether this request committed a transition
// and, when present, the authoritative status observed by the repository.
type UserStatusUpdateResult struct {
	Outcome       UserStatusUpdateOutcome
	CurrentStatus valueobject.UserStatus
}

type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	ExistsByID(ctx context.Context, id string) (bool, error)
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	GetCredentialVersion(ctx context.Context, id string) (int64, error)
	UpdatePassword(ctx context.Context, id string, password valueobject.Password) (int64, error)
	Update(ctx context.Context, u *entity.User) error
	UpdateStatus(
		ctx context.Context,
		userID string,
		expected valueobject.UserStatus,
		requested valueobject.UserStatus,
	) (UserStatusUpdateResult, error)
	Delete(ctx context.Context, userID string) error
	List(ctx context.Context, offset, limit int, filters UserListFilter) ([]*entity.User, int64, error)
}
