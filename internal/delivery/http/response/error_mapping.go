package response

import (
	"errors"
	"net/http"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func MapError(err error) ProblemDescriptor {
	var currentSessionInvalid *auth.CurrentSessionInvalidError

	switch {
	case errors.As(err, &currentSessionInvalid):
		return ProblemDescriptor{
			Type:      "/errors/unauthorized",
			Status:    http.StatusUnauthorized,
			TitleKey:  titleUnauthorized,
			DetailKey: detailCurrentSessionNotFound,
		}
	case errors.Is(err, session.ErrInvalidSessionSelection):
		return ProblemDescriptor{
			Type:      "/errors/validation",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: DetailInvalidRequestPayload,
		}
	case errors.Is(err, domainErrors.ErrPasswordTooShort):
		return ProblemDescriptor{
			Type:      "/errors/invalid-password",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: detailPasswordTooShort,
		}
	case errors.Is(err, domainErrors.ErrPasswordTooLong):
		return ProblemDescriptor{
			Type:      "/errors/invalid-password",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: detailPasswordTooLong,
		}
	case errors.Is(err, domainErrors.ErrPasswordPolicyViolation):
		return ProblemDescriptor{
			Type:      "/errors/invalid-password",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: detailPasswordPolicyViolation,
		}
	case errors.Is(err, domainErrors.ErrInvalidPassword):
		return ProblemDescriptor{
			Type:      "/errors/validation",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: detailCurrentPasswordIncorrect,
		}
	case errors.Is(err, domainErrors.ErrBadRequest), errors.Is(err, domainErrors.ErrInvalidInput):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusBadRequest,
			TitleKey:  titleBadRequest,
			DetailKey: detailProcessingError,
		}
	case errors.Is(err, domainErrors.ErrTokenExpired):
		return ProblemDescriptor{
			Type:      "/errors/unauthorized",
			Status:    http.StatusUnauthorized,
			TitleKey:  titleUnauthorized,
			DetailKey: detailTokenExpired,
		}
	case errors.Is(err, domainErrors.ErrTokenInvalid):
		return ProblemDescriptor{
			Type:      "/errors/unauthorized",
			Status:    http.StatusUnauthorized,
			TitleKey:  titleUnauthorized,
			DetailKey: detailTokenInvalid,
		}
	case errors.Is(err, domainErrors.ErrInvalidCredentials):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusUnauthorized,
			TitleKey:  titleUnauthorized,
			DetailKey: detailInvalidCredentials,
		}
	case errors.Is(err, domainErrors.ErrUnauthorized):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusUnauthorized,
			TitleKey:  titleUnauthorized,
			DetailKey: detailProcessingError,
		}
	case errors.Is(err, domainErrors.ErrAccountSuspended):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusForbidden,
			TitleKey:  titleForbidden,
			DetailKey: detailAccountSuspendedContactSupport,
		}
	case errors.Is(err, domainErrors.ErrForbidden):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusForbidden,
			TitleKey:  titleForbidden,
			DetailKey: detailProcessingError,
		}
	case errors.Is(err, domainErrors.ErrPermissionNotFound),
		errors.Is(err, domainErrors.ErrUserNotFound),
		errors.Is(err, domainErrors.ErrNotFound):
		return ProblemDescriptor{
			Type:      "/errors/not-found",
			Status:    http.StatusNotFound,
			TitleKey:  titleNotFound,
			DetailKey: detailResourceNotFound,
		}
	case errors.Is(err, domainErrors.ErrEmailAlreadyExists):
		return ProblemDescriptor{
			Type:      "/errors/email-already-exists",
			Status:    http.StatusConflict,
			TitleKey:  titleConflict,
			DetailKey: detailEmailAlreadyExists,
		}
	case errors.Is(err, domainErrors.ErrPasswordSameAsCurrent):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusConflict,
			TitleKey:  titleConflict,
			DetailKey: detailPasswordSameAsCurrent,
		}
	case errors.Is(err, domainErrors.ErrPermissionAlreadyExists), errors.Is(err, domainErrors.ErrConflict):
		return ProblemDescriptor{
			Type:      "/errors/conflict",
			Status:    http.StatusConflict,
			TitleKey:  titleConflict,
			DetailKey: detailConflict,
		}
	case errors.Is(err, domainErrors.ErrRateLimitExceeded):
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusTooManyRequests,
			TitleKey:  titleTooManyRequests,
			DetailKey: detailProcessingError,
		}
	default:
		return ProblemDescriptor{
			Type:      "/errors/internal",
			Status:    http.StatusInternalServerError,
			TitleKey:  titleInternalServerError,
			DetailKey: detailUnexpected,
		}
	}
}
