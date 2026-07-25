package authorization

import (
	"context"
	"errors"
	"fmt"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type AuthorizationUseCase struct {
	identityReader IdentityStateReader
	securityReader SecurityStateReader
}

func NewUsecase(
	identityReader IdentityStateReader,
	securityReader SecurityStateReader,
) UseCase {
	return &AuthorizationUseCase{
		identityReader: identityReader,
		securityReader: securityReader,
	}
}

func (us *AuthorizationUseCase) AuthorizeFreshIdentity(
	ctx context.Context,
	principal Principal,
) (Principal, error) {
	if !principal.IsValid() {
		return Principal{}, ErrPrincipalSecurityStateChanged
	}

	state, err := us.identityReader.GetIdentityState(ctx, principal.UserID())
	if err != nil {
		return Principal{}, classifyStateReadError(err)
	}
	if err := validateIdentityState(state); err != nil {
		return Principal{}, err
	}
	if state.UserID != principal.UserID() ||
		state.CredentialVersion != principal.CredentialVersion() {
		return Principal{}, ErrPrincipalSecurityStateChanged
	}
	if err := validateActiveStatus(state.Status); err != nil {
		return Principal{}, err
	}

	return NewPrincipal(
		state.UserID,
		principal.SessionID(),
		state.CredentialVersion,
		principal.Role(),
		principal.Permissions(),
	)
}

func (us *AuthorizationUseCase) AuthorizeFresh(
	ctx context.Context,
	principal Principal,
	required valueobject.Permission,
) (Principal, error) {
	if !principal.IsValid() {
		return Principal{}, ErrPrincipalSecurityStateChanged
	}

	state, err := us.securityReader.GetSecurityState(ctx, principal.UserID())
	if err != nil {
		return Principal{}, classifyStateReadError(err)
	}
	if err := validateSecurityState(state); err != nil {
		return Principal{}, err
	}
	if state.UserID != principal.UserID() ||
		state.CredentialVersion != principal.CredentialVersion() {
		return Principal{}, ErrPrincipalSecurityStateChanged
	}

	if err := validateActiveStatus(state.Status); err != nil {
		return Principal{}, err
	}

	if required != "" {
		if _, err := valueobject.ParsePermission(required.String()); err != nil {
			return Principal{}, fmt.Errorf("required permission is invalid: %w", err)
		}
		if !state.Permissions.Has(required) {
			return Principal{}, ErrPermissionDenied
		}
	}

	return NewPrincipal(
		state.UserID,
		principal.SessionID(),
		state.CredentialVersion,
		state.Role,
		state.Permissions,
	)
}

func classifyStateReadError(err error) error {
	if errors.Is(err, domainErrors.ErrUserNotFound) {
		return fmt.Errorf(
			"%w: %w",
			ErrPrincipalSecurityStateChanged,
			err,
		)
	}
	return err
}

func validateIdentityState(state IdentityState) error {
	if state.UserID == "" {
		return fmt.Errorf("authoritative user identity is empty")
	}
	if state.CredentialVersion <= 0 {
		return fmt.Errorf("authoritative credential version must be positive")
	}
	return nil
}

func validateSecurityState(state SecurityState) error {
	if err := validateIdentityState(IdentityState{
		UserID:            state.UserID,
		Status:            state.Status,
		CredentialVersion: state.CredentialVersion,
	}); err != nil {
		return err
	}
	if !isKnownRole(state.Role) {
		return fmt.Errorf("authoritative user role is invalid")
	}
	return nil
}

func validateActiveStatus(status valueobject.UserStatus) error {
	switch status {
	case valueobject.StatusInactive:
		return ErrPrincipalInactive
	case valueobject.StatusSuspended:
		return ErrPrincipalSuspended
	case valueobject.StatusActive:
		return nil
	default:
		return fmt.Errorf("authoritative user status is invalid")
	}
}
