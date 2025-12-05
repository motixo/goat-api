// internal/delivery/http/response/response.go
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/errors"
)

type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
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

func respondWithProblem(c *gin.Context, status int, title, detail, typ string) {
	c.AbortWithStatusJSON(status, Problem{
		Type:     typ,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
	})
}

func problemType(err error) string {
	switch err {
	case errors.ErrPasswordTooShort,
		errors.ErrPasswordTooLong,
		errors.ErrPasswordPolicyViolation:
		return "/errors/invalid-password"

	case errors.ErrEmailAlreadyExists:
		return "/errors/email-already-exists"

	case errors.ErrUserNotFound, errors.ErrNotFound:
		return "/errors/not-found"

	case errors.ErrConflict:
		return "/errors/conflict"

	default:
		return "/errors/internal"
	}
}

func clientMessage(err error) string {
	switch err {
	case errors.ErrPasswordTooShort:
		return "Password must be at least 8 characters long."
	case errors.ErrPasswordTooLong:
		return "Password must not exceed 72 characters."
	case errors.ErrPasswordPolicyViolation:
		return "Password must contain uppercase, lowercase, number, and special character."
	case errors.ErrEmailAlreadyExists:
		return "This email is already registered."
	case errors.ErrUserNotFound, errors.ErrNotFound:
		return "The requested resource was not found."
	case errors.ErrConflict:
		return "The request conflicts with current state."
	default:
		return "An error occurred while processing your request."
	}
}

func DomainError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	status := errors.HTTPStatus(err)
	title := http.StatusText(status)

	if status == http.StatusInternalServerError {
		respondWithProblem(c, status, "Internal Server Error", "An unexpected error occurred.", "/errors/internal")
		return
	}

	respondWithProblem(c, status, title, clientMessage(err), problemType(err))
}

func BadRequest(c *gin.Context, detail string) {
	respondWithProblem(c, http.StatusBadRequest, "Bad Request", detail, "/errors/validation")
}

func Unauthorized(c *gin.Context, detail string) {
	respondWithProblem(c, http.StatusUnauthorized, "Unauthorized", detail, "/errors/unauthorized")
}

func NotFound(c *gin.Context) {
	respondWithProblem(c, http.StatusNotFound, "Not Found", "The requested resource was not found.", "/errors/not-found")
}

func Internal(c *gin.Context) {
	respondWithProblem(c, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred.", "/errors/internal")
}

func Forbidden(c *gin.Context, detail string) {
	respondWithProblem(c, http.StatusForbidden, "Forbidden", detail, "/errors/forbidden")
}
