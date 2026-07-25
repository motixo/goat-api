package authorization

import (
	"fmt"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// Principal contains only identity and authorization values that have already
// passed token and server-owned session validation. Its fields are private so
// request consumers cannot mutate the verified snapshot.
type Principal struct {
	userID            string
	sessionID         string
	credentialVersion int64
	role              valueobject.UserRole
	permissions       valueobject.PermissionSet
}

func NewPrincipal(
	userID string,
	sessionID string,
	credentialVersion int64,
	role valueobject.UserRole,
	permissions valueobject.PermissionSet,
) (Principal, error) {
	if userID == "" || sessionID == "" {
		return Principal{}, fmt.Errorf("principal identity is incomplete")
	}
	if credentialVersion <= 0 {
		return Principal{}, fmt.Errorf("principal credential version must be positive")
	}
	if !isKnownRole(role) {
		return Principal{}, fmt.Errorf("principal role is invalid")
	}
	return Principal{
		userID:            userID,
		sessionID:         sessionID,
		credentialVersion: credentialVersion,
		role:              role,
		permissions:       permissions,
	}, nil
}

func (p Principal) UserID() string {
	return p.userID
}

func (p Principal) SessionID() string {
	return p.sessionID
}

func (p Principal) CredentialVersion() int64 {
	return p.credentialVersion
}

func (p Principal) Role() valueobject.UserRole {
	return p.role
}

func (p Principal) Permissions() valueobject.PermissionSet {
	return p.permissions
}

func (p Principal) IsValid() bool {
	return p.userID != "" &&
		p.sessionID != "" &&
		p.credentialVersion > 0 &&
		isKnownRole(p.role)
}

type SecurityState struct {
	UserID            string
	Status            valueobject.UserStatus
	Role              valueobject.UserRole
	CredentialVersion int64
	Permissions       valueobject.PermissionSet
}

type IdentityState struct {
	UserID            string
	Status            valueobject.UserStatus
	CredentialVersion int64
}

func isKnownRole(role valueobject.UserRole) bool {
	switch role {
	case valueobject.RoleClient, valueobject.RoleOperator, valueobject.RoleAdmin:
		return true
	default:
		return false
	}
}
