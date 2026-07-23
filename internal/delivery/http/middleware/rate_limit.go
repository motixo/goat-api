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
	Auth    RateLimit
	Public  RateLimit
	Private RateLimit
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

func (m *RateLimitMiddleware) Handler(config RateLimit) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorType := "ip"
		actorID := c.ClientIP()

		if userID := c.GetString(UserIDKey); userID != "" {
			actorType = "user"
			actorID = userID
		}

		allowed, retryAfter, currentCount, err := m.limiter.Allow(
			c.Request.Context(),
			actorType,
			actorID,
			c.FullPath(),
			config.Limit,
			config.Window,
		)

		if err != nil {
			m.logger.Error("Rate limiter Redis error", "error", err)
			c.Next()
			return
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
			return
		}

		c.Next()
	}
}
