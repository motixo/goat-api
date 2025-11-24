package errors

import (
	"errors"
	"net/http"
)

var (
	ErrInternal     = errors.New("internal server error")
	ErrBadRequest   = errors.New("bad request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

func HTTPStatus(err error) int {
	switch err {
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrForbidden:
		return http.StatusForbidden
	case ErrNotFound, ErrUserNotFound:
		return http.StatusNotFound
	case ErrConflict, ErrEmailAlreadyExists:
		return http.StatusConflict

	case ErrBadRequest,
		ErrPasswordTooShort,
		ErrPasswordTooLong,
		ErrPasswordPolicyViolation:
		return http.StatusBadRequest

	default:
		return http.StatusInternalServerError
	}
}
