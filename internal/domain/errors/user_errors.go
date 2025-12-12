package errors

import "errors"

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInactive       = errors.New("user is not active")
	ErrAccountSuspended   = errors.New("user is suspended")
)
