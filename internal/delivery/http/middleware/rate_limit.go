package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/pkg"
)

type RateLimitConfig struct {
	Auth        RateLimit
	Public      RateLimit
	ProtectedIP RateLimit
	Private     RateLimit
}
type RateLimit struct {
	Limit  int
	Window time.Duration
}

type RateLimitMiddleware struct {
	limiter service.RateLimiter
	logger  pkg.Logger
}

func NewRateLimitMiddleware(limiter service.RateLimiter, logger pkg.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
		logger:  logger,
	}
}

// Handler is an outer admission boundary. It intentionally keys requests by
// the trusted-proxy-aware client IP so it can run before authentication and
// protect JWT, Redis session, PostgreSQL authorization, and password work.
func (m *RateLimitMiddleware) Handler(config RateLimit) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.allow(c, "ip", c.ClientIP(), config) {
			c.Next()
		}
	}
}

// Authenticated applies a per-user limit only after authentication has attached
// a verified principal. Client-supplied user IDs and fingerprint headers are
// deliberately excluded from the rate-limit key.
func (m *RateLimitMiddleware) Authenticated(config RateLimit) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok {
			response.Unauthorized(c, response.DetailAuthenticationRequired)
			return
		}
		if m.allow(c, "user", principal.UserID(), config) {
			c.Next()
		}
	}
}

func (m *RateLimitMiddleware) allow(
	c *gin.Context,
	actorType string,
	actorID string,
	config RateLimit,
) bool {
	allowed, retryAfter, currentCount, err := m.limiter.Allow(
		c.Request.Context(),
		actorType,
		actorID,
		c.FullPath(),
		config.Limit,
		config.Window,
	)

	if err != nil {
		m.logger.Error("rate-limit enforcement failed", "error", err)
		response.WriteProblem(c, response.MapError(err))
		return false
	}

	remaining := int64(config.Limit) - currentCount
	if remaining < 0 {
		remaining = 0
	}

	c.Header("X-RateLimit-Limit", strconv.Itoa(config.Limit))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

	resetTimestamp := time.Now().Add(retryAfter).Unix()
	c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTimestamp, 10))

	meta := gin.H{
		"limit":       config.Limit,
		"window":      config.Window.String(),
		"retry_after": retryAfter.Round(time.Second).String(),
	}

	if !allowed {
		c.Header("Retry-After", strconv.FormatInt(int64(retryAfter.Seconds()), 10))

		response.TooManyRequests(
			c,
			response.DetailRateLimitExceeded,
			response.TranslationParams{"RetryAfter": retryAfter.Round(time.Second).String()},
			meta,
		)
		return false
	}

	return true
}
