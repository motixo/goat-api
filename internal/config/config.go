package config

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
)

const (
	minimumAccessTokenLifetime     = time.Minute
	maximumAccessTokenLifetime     = 15 * time.Minute
	maximumDBConnectionTimeout     = time.Minute
	maximumDBInitializationTimeout = 30 * time.Minute
	maximumRedisConnectionTimeout  = time.Minute
)

// Config holds all application configuration
type Config struct {
	Env                     string        `envconfig:"ENV" default:"development"`
	ServerPort              string        `envconfig:"SERVER_PORT" default:"8080"`
	DBHost                  string        `envconfig:"DB_HOST" required:"true"`
	DBPort                  string        `envconfig:"DB_PORT" default:"5432"`
	DBUser                  string        `envconfig:"DB_USER" required:"true"`
	DBPassword              string        `envconfig:"DB_PASSWORD" required:"true"`
	DBName                  string        `envconfig:"DB_NAME" required:"true"`
	DBConnectionTimeout     time.Duration `envconfig:"DB_CONNECTION_TIMEOUT" default:"5s"`
	DBInitializationTimeout time.Duration `envconfig:"DB_INITIALIZATION_TIMEOUT" default:"2m"`
	JWTSecret               string        `envconfig:"JWT_SECRET" required:"true"`
	PasswordPepper          string        `envconfig:"PASSWORD_PEPPER" required:"true"`
	RedisHost               string        `envconfig:"REDIS_HOST" default:"localhost"`
	RedisPort               string        `envconfig:"REDIS_PORT" default:"6379"`
	RedisPassword           string        `envconfig:"REDIS_PASSWORD"`
	RedisDB                 int           `envconfig:"REDIS_DB" default:"0"`
	RedisConnectionTimeout  time.Duration `envconfig:"REDIS_CONNECTION_TIMEOUT" default:"5s"`
	JWTExpiration           time.Duration `envconfig:"JWT_EXPIRATION" default:"5m"`
	RefreshTokenExpiration  time.Duration `envconfig:"REFRESH_TOKEN_EXPIRATION" default:"168h"`
	SessionExpiration       time.Duration `envconfig:"SESSION_EXPIRATION" default:"720h"`
	GinMode                 string        `envconfig:"GIN_MODE" default:"debug"`
	Seed                    int           `envconfig:"SEED" default:"1"`
	AdminEmail              string        `envconfig:"ADMIN_EMAIL" default:"admin@goat.api"`
	AdminPassword           string        `envconfig:"ADMIN_PASSWORD" default:"Qwerty@123"`
	RateLimitAuthLimit      int           `envconfig:"RATE_LIMIT_AUTH_LIMIT" default:"5"`
	RateLimitAuthWindow     time.Duration `envconfig:"RATE_LIMIT_AUTH_WINDOW" default:"1m"`
	RateLimitPublicLimit    int           `envconfig:"RATE_LIMIT_PUBLIC_LIMIT" default:"100"`
	RateLimitPublicWindow   time.Duration `envconfig:"RATE_LIMIT_PUBLIC_WINDOW" default:"1m"`
	RateLimitPrivateLimit   int           `envconfig:"RATE_LIMIT_PRIVATE_LIMIT" default:"60"`
	RateLimitPrivateWindow  time.Duration `envconfig:"RATE_LIMIT_PRIVATE_WINDOW" default:"1m"`
}

// Load reads configuration from environment variables and .env file
func Load() (*Config, error) {
	// Load .env if exists
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate ENV
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Prefix server port
	cfg.ServerPort = ":" + cfg.ServerPort

	// Set gin mode automatically
	switch cfg.GinMode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "debug":
		gin.SetMode(gin.DebugMode)
	default:
		return nil, fmt.Errorf("invalid GIN_MODE: must be 'debug' or 'release'")
	}

	return &cfg, nil
}

// validate ensures required fields are set
func (c *Config) validate() error {
	if c.Env != "development" && c.Env != "production" {
		return fmt.Errorf("invalid ENV: must be 'development' or 'production'")
	}
	if c.JWTExpiration < minimumAccessTokenLifetime ||
		c.JWTExpiration > maximumAccessTokenLifetime {
		return fmt.Errorf(
			"invalid JWT_EXPIRATION: must be between %s and %s",
			minimumAccessTokenLifetime,
			maximumAccessTokenLifetime,
		)
	}
	if c.DBConnectionTimeout <= 0 ||
		c.DBConnectionTimeout > maximumDBConnectionTimeout {
		return fmt.Errorf(
			"invalid DB_CONNECTION_TIMEOUT: must be positive and no greater than %s",
			maximumDBConnectionTimeout,
		)
	}
	if c.DBInitializationTimeout <= 0 ||
		c.DBInitializationTimeout > maximumDBInitializationTimeout {
		return fmt.Errorf(
			"invalid DB_INITIALIZATION_TIMEOUT: must be positive and no greater than %s",
			maximumDBInitializationTimeout,
		)
	}
	if c.RedisConnectionTimeout <= 0 ||
		c.RedisConnectionTimeout > maximumRedisConnectionTimeout {
		return fmt.Errorf(
			"invalid REDIS_CONNECTION_TIMEOUT: must be positive and no greater than %s",
			maximumRedisConnectionTimeout,
		)
	}
	return nil
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

// RedisOptions returns redis.Options
func (c *Config) RedisOptions() *redis.Options {
	return &redis.Options{
		Addr:                  c.RedisHost + ":" + c.RedisPort,
		Password:              c.RedisPassword,
		DB:                    c.RedisDB,
		ContextTimeoutEnabled: true,
	}
}

// IsProduction helper
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
