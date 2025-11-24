package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/domain/errors"
)

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, gin.H{"data": data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Generic error responder
func Error(c *gin.Context, status int, title, detail string) {
	c.AbortWithStatusJSON(status, Problem{
		Type:     "/errors/" + http.StatusText(status),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Request.URL.Path,
	})
}

// Map domain errors to proper HTTP responses
func DomainError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	status := errors.HTTPStatus(err)
	title := http.StatusText(status)
	detail := err.Error()

	// Customize common cases if needed
	if status == http.StatusInternalServerError {
		title = "Internal Server Error"
		detail = "Something went wrong on the server."
	}

	Error(c, status, title, detail)
}

// Convenience helpers
func BadRequest(c *gin.Context, detail string) {
	Error(c, http.StatusBadRequest, "Bad Request", detail)
}

func Unauthorized(c *gin.Context, detail string) {
	Error(c, http.StatusUnauthorized, "Unauthorized", detail)
}

func Forbidden(c *gin.Context) {
	Error(c, http.StatusForbidden, "Forbidden", "You do not have permission to perform this action.")
}

func NotFound(c *gin.Context) {
	Error(c, http.StatusNotFound, "Not Found", "The requested resource was not found.")
}

func Conflict(c *gin.Context, detail string) {
	Error(c, http.StatusConflict, "Conflict", detail)
}

func Internal(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "Internal Server Error", "Something went wrong on the server.")
}
