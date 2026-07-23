// internal/delivery/http/response/response.go
package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Metadata any    `json:"metadata,omitempty"`
}

// Success
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func WriteProblem(c *gin.Context, problem Problem) {
	problem.Instance = c.Request.URL.Path
	c.AbortWithStatusJSON(problem.Status, problem)
}

func MapError(err error) Problem {
	var currentSessionInvalid *auth.CurrentSessionInvalidError

	switch {
	case errors.As(err, &currentSessionInvalid):
		return Problem{
			Type:   "/errors/unauthorized",
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "not found",
		}
	case errors.Is(err, session.ErrInvalidSessionSelection):
		return Problem{
			Type:   "/errors/validation",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Invalid request payload",
		}
	case errors.Is(err, domainErrors.ErrPasswordTooShort):
		return Problem{
			Type:   "/errors/invalid-password",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Password must be at least 8 characters long.",
		}
	case errors.Is(err, domainErrors.ErrPasswordTooLong):
		return Problem{
			Type:   "/errors/invalid-password",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Password must not exceed 72 characters.",
		}
	case errors.Is(err, domainErrors.ErrPasswordPolicyViolation):
		return Problem{
			Type:   "/errors/invalid-password",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Password must contain uppercase, lowercase, number, and special character.",
		}
	case errors.Is(err, domainErrors.ErrInvalidPassword):
		return Problem{
			Type:   "/errors/validation",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "current password is incorrect",
		}
	case errors.Is(err, domainErrors.ErrBadRequest), errors.Is(err, domainErrors.ErrInvalidInput):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "An error occurred while processing your request.",
		}
	case errors.Is(err, domainErrors.ErrTokenExpired):
		return Problem{
			Type:   "/errors/unauthorized",
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "token has expired",
		}
	case errors.Is(err, domainErrors.ErrTokenInvalid):
		return Problem{
			Type:   "/errors/unauthorized",
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "invalid or malformed token",
		}
	case errors.Is(err, domainErrors.ErrInvalidCredentials):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "Invalid email or password.",
		}
	case errors.Is(err, domainErrors.ErrUnauthorized):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "An error occurred while processing your request.",
		}
	case errors.Is(err, domainErrors.ErrAccountSuspended):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "Your account has been suspended. Please contact support.",
		}
	case errors.Is(err, domainErrors.ErrForbidden):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Forbidden",
			Status: http.StatusForbidden,
			Detail: "An error occurred while processing your request.",
		}
	case errors.Is(err, domainErrors.ErrPermissionNotFound),
		errors.Is(err, domainErrors.ErrUserNotFound),
		errors.Is(err, domainErrors.ErrNotFound):
		return Problem{
			Type:   "/errors/not-found",
			Title:  "Not Found",
			Status: http.StatusNotFound,
			Detail: "The requested resource was not found.",
		}
	case errors.Is(err, domainErrors.ErrEmailAlreadyExists):
		return Problem{
			Type:   "/errors/email-already-exists",
			Title:  "Conflict",
			Status: http.StatusConflict,
			Detail: "This email is already registered.",
		}
	case errors.Is(err, domainErrors.ErrPasswordSameAsCurrent):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Conflict",
			Status: http.StatusConflict,
			Detail: "Passwords can't be same",
		}
	case errors.Is(err, domainErrors.ErrPermissionAlreadyExists), errors.Is(err, domainErrors.ErrConflict):
		return Problem{
			Type:   "/errors/conflict",
			Title:  "Conflict",
			Status: http.StatusConflict,
			Detail: "The request conflicts with current state.",
		}
	case errors.Is(err, domainErrors.ErrRateLimitExceeded):
		return Problem{
			Type:   "/errors/internal",
			Title:  "Too Many Requests",
			Status: http.StatusTooManyRequests,
			Detail: "An error occurred while processing your request.",
		}
	default:
		return Problem{
			Type:   "/errors/internal",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "An unexpected error occurred.",
		}
	}
}

func BadRequest(c *gin.Context, detail string) {
	WriteProblem(c, Problem{Type: "/errors/validation", Title: "Bad Request", Status: http.StatusBadRequest, Detail: detail})
}

func Unauthorized(c *gin.Context, detail string) {
	WriteProblem(c, Problem{Type: "/errors/unauthorized", Title: "Unauthorized", Status: http.StatusUnauthorized, Detail: detail})
}

func NotFound(c *gin.Context) {
	WriteProblem(c, MapError(domainErrors.ErrNotFound))
}

func Internal(c *gin.Context) {
	WriteProblem(c, MapError(domainErrors.ErrInternal))
}

func Forbidden(c *gin.Context, detail string) {
	WriteProblem(c, Problem{Type: "/errors/forbidden", Title: "Forbidden", Status: http.StatusForbidden, Detail: detail})
}

func TooManyRequests(c *gin.Context, detail string, metadata any) {
	WriteProblem(c, Problem{
		Type:     "/errors/rate-limit",
		Title:    "Too Many Requests",
		Status:   http.StatusTooManyRequests,
		Detail:   detail,
		Metadata: metadata,
	})
}
