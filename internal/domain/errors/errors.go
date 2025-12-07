package errors

import (
	"errors"
	"net/http"
)

var (
	ErrInternal     = errors.New("internal server error")
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrTokenExpired = errors.New("token expired")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
)

func HTTPStatus(err error) int {
	switch err {
	case ErrUnauthorized, ErrTokenExpired, ErrInvalidCredentials:
		return http.StatusUnauthorized
	case ErrForbidden:
		return http.StatusForbidden
	case ErrNotFound, ErrUserNotFound:
		return http.StatusNotFound
	case ErrConflict, ErrEmailAlreadyExists, ErrPasswordSameAsCurrent:
		return http.StatusConflict

	case ErrBadRequest,
		ErrPasswordTooShort,
		ErrPasswordTooLong,
		ErrPasswordPolicyViolation,
		ErrInvalidInput:
		return http.StatusBadRequest

	default:
		return http.StatusInternalServerError
	}
}
