package user

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/user/detail"
	"github.com/motixo/goat-api/internal/usecase/user/emailchange"
	"github.com/motixo/goat-api/internal/usecase/user/listing"
	"github.com/motixo/goat-api/internal/usecase/user/rolechange"
	"github.com/motixo/goat-api/internal/usecase/user/statuschange"
	"github.com/motixo/goat-api/internal/usecase/user/updating"
)

type UserOutput struct {
	ID        string
	Email     string
	Role      string
	Status    string
	CreatedAt time.Time
}

type CreateInput struct {
	Email    string
	Password string
	Status   valueobject.UserStatus
	Role     valueobject.UserRole
}

type UpdateInput struct {
	UserID   string
	Email    string
	Password string
}

type UpdateEmailInput struct {
	UserID string
	Email  string
}

type UpdatePassInput struct {
	UserID      string
	OldPassword string
	NewPassword string
}

type UpdateRoleInput struct {
	UserID    string
	ActorID   string
	ActorRole valueobject.UserRole
	Role      valueobject.UserRole
}

type UpdateStatusInput struct {
	UserID    string
	ActorID   string
	ActorRole valueobject.UserRole
	Status    valueobject.UserStatus
}

type ListFilter struct {
	Statuses []valueobject.UserStatus
	Roles    []valueobject.UserRole
	Search   string
	// MatchNone preserves the meaning of a supplied filter containing no known values.
	MatchNone bool
}

type UserListCriteria = listing.UserListCriteria
type UserListItem = listing.UserListItem
type UserListResult = listing.UserListResult
type UserListReader = listing.UserListReader
type UserDetail = detail.UserDetail
type UserDetailReader = detail.UserDetailReader
type UserEmailUpdateCommand = emailchange.UserEmailUpdateCommand
type UserEmailUpdateWriter = emailchange.UserEmailUpdateWriter
type UserStatusSnapshot = statuschange.UserStatusSnapshot
type UserStatusSnapshotReader = statuschange.UserStatusSnapshotReader
type UserUpdateCommand = updating.UserUpdateCommand
type UserUpdateWriter = updating.UserUpdateWriter
type UserRoleUpdateCommand = rolechange.UserRoleUpdateCommand
type UserRoleUpdateOutcome = rolechange.UserRoleUpdateOutcome
type UserRoleUpdateResult = rolechange.UserRoleUpdateResult
type UserRoleUpdateWriter = rolechange.UserRoleUpdateWriter

const (
	UserRoleUpdateUnknown        = rolechange.UserRoleUpdateUnknown
	UserRoleUpdateApplied        = rolechange.UserRoleUpdateApplied
	UserRoleUpdateAlreadyApplied = rolechange.UserRoleUpdateAlreadyApplied
	UserRoleUpdateConflict       = rolechange.UserRoleUpdateConflict
	UserRoleUpdateNotFound       = rolechange.UserRoleUpdateNotFound
)

type GetListInput struct {
	ActorID   string
	ActorRole valueobject.UserRole
	Filter    ListFilter
	Offset    int
	Limit     int
}
