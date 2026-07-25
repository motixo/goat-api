package errors

import "errors"

var (
	ErrEmailAlreadyExists          = errors.New("email already exists")
	ErrUserNotFound                = errors.New("user not found")
	ErrUserInactive                = errors.New("user is not active")
	ErrAccountSuspended            = errors.New("user is suspended")
	ErrUserAccessBlocked           = errors.New("user access is blocked")
	ErrInvalidUserStatusTransition = errors.New("invalid user status transition")
)
