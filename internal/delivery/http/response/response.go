// internal/delivery/http/response/response.go
package response

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
)

const (
	problemContentType   = "application/problem+json"
	acceptLanguageHeader = "Accept-Language"
)

type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Metadata any    `json:"metadata,omitempty"`
}

type ProblemDescriptor struct {
	Type      string
	Status    int
	TitleKey  TranslationKey
	DetailKey TranslationKey
	Params    TranslationParams
	Metadata  any
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

func WriteProblem(c *gin.Context, descriptor ProblemDescriptor) {
	requestLocale := resolveLocale(c.GetHeader(acceptLanguageHeader))
	setProblemHeaders(c.Writer.Header(), requestLocale)
	problem := Problem{
		Type:     descriptor.Type,
		Title:    problemLocalizer.translate(requestLocale, descriptor.TitleKey, descriptor.Params),
		Status:   descriptor.Status,
		Detail:   problemLocalizer.translate(requestLocale, descriptor.DetailKey, descriptor.Params),
		Instance: c.Request.URL.Path,
		Metadata: descriptor.Metadata,
	}
	c.AbortWithStatusJSON(problem.Status, problem)
}

func setProblemHeaders(header http.Header, responseLocale locale) {
	header.Set("Content-Type", problemContentType)
	header.Set("Content-Language", string(responseLocale))
	mergeVary(header, acceptLanguageHeader)
}

func mergeVary(header http.Header, requiredValue string) {
	requiredKey := strings.ToLower(requiredValue)
	seen := make(map[string]struct{})
	values := make([]string, 0, len(header.Values("Vary"))+1)

	for _, line := range header.Values("Vary") {
		for _, rawValue := range strings.Split(line, ",") {
			value := strings.TrimSpace(rawValue)
			if value == "" {
				continue
			}
			if value == "*" {
				header.Set("Vary", "*")
				return
			}

			key := strings.ToLower(value)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if key == requiredKey {
				value = requiredValue
			}
			values = append(values, value)
		}
	}

	if _, exists := seen[requiredKey]; !exists {
		values = append(values, requiredValue)
	}
	header.Set("Vary", strings.Join(values, ", "))
}

func BadRequest(c *gin.Context, detailKey TranslationKey) {
	WriteProblem(c, ProblemDescriptor{
		Type:      "/errors/validation",
		Status:    http.StatusBadRequest,
		TitleKey:  titleBadRequest,
		DetailKey: detailKey,
	})
}

func BadRequestWithParams(c *gin.Context, detailKey TranslationKey, params TranslationParams) {
	WriteProblem(c, ProblemDescriptor{
		Type:      "/errors/validation",
		Status:    http.StatusBadRequest,
		TitleKey:  titleBadRequest,
		DetailKey: detailKey,
		Params:    params,
	})
}

func Unauthorized(c *gin.Context, detailKey TranslationKey) {
	WriteProblem(c, ProblemDescriptor{
		Type:      "/errors/unauthorized",
		Status:    http.StatusUnauthorized,
		TitleKey:  titleUnauthorized,
		DetailKey: detailKey,
	})
}

func NotFound(c *gin.Context) {
	WriteProblem(c, MapError(domainErrors.ErrNotFound))
}

func Internal(c *gin.Context) {
	WriteProblem(c, MapError(domainErrors.ErrInternal))
}

func Forbidden(c *gin.Context, detailKey TranslationKey) {
	WriteProblem(c, ProblemDescriptor{
		Type:      "/errors/forbidden",
		Status:    http.StatusForbidden,
		TitleKey:  titleForbidden,
		DetailKey: detailKey,
	})
}

func TooManyRequests(c *gin.Context, detailKey TranslationKey, params TranslationParams, metadata any) {
	WriteProblem(c, ProblemDescriptor{
		Type:      "/errors/rate-limit",
		Status:    http.StatusTooManyRequests,
		TitleKey:  titleTooManyRequests,
		DetailKey: detailKey,
		Params:    params,
		Metadata:  metadata,
	})
}
