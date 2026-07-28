package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	caarlosenv "github.com/caarlos0/env/v11"
	"github.com/motixo/goat-api/internal/domain/validation"
)

const (
	minimumAccessTokenLifetime     = time.Minute
	maximumAccessTokenLifetime     = 15 * time.Minute
	maximumDBConnectionTimeout     = time.Minute
	maximumDBInitializationTimeout = 30 * time.Minute
	maximumRedisConnectionTimeout  = time.Minute
	maximumPasswordHashConcurrency = 4
	defaultAdminEmail              = "admin@goat.api"
	defaultAdminPassword           = "Qwerty@123"
)

var nonEmptyEnvironmentVariables = []string{
	"ENV",
	"SERVER_PORT",
	"DB_HOST",
	"DB_PORT",
	"DB_USER",
	"DB_PASSWORD",
	"DB_NAME",
	"DB_CONNECTION_TIMEOUT",
	"DB_INITIALIZATION_TIMEOUT",
	"JWT_SECRET",
	"PASSWORD_PEPPER",
	"PASSWORD_HASH_MAX_CONCURRENCY",
	"REDIS_HOST",
	"REDIS_PORT",
	"REDIS_DB",
	"REDIS_CONNECTION_TIMEOUT",
	"JWT_EXPIRATION",
	"REFRESH_TOKEN_EXPIRATION",
	"SESSION_EXPIRATION",
	"GIN_MODE",
	"SEED",
	"ADMIN_EMAIL",
	"ADMIN_PASSWORD",
	"RATE_LIMIT_AUTH_LIMIT",
	"RATE_LIMIT_AUTH_WINDOW",
	"RATE_LIMIT_PUBLIC_LIMIT",
	"RATE_LIMIT_PUBLIC_WINDOW",
	"RATE_LIMIT_PRIVATE_LIMIT",
	"RATE_LIMIT_PRIVATE_WINDOW",
}

// Config holds all application configuration
type Config struct {
	Env                        string        `env:"ENV" envDefault:"development"`
	ServerPort                 uint16        `env:"SERVER_PORT" envDefault:"8080"`
	DBHost                     string        `env:"DB_HOST,required,notEmpty"`
	DBPort                     uint16        `env:"DB_PORT" envDefault:"5432"`
	DBUser                     string        `env:"DB_USER,required,notEmpty"`
	DBPassword                 string        `env:"DB_PASSWORD,required,notEmpty"`
	DBName                     string        `env:"DB_NAME,required,notEmpty"`
	DBConnectionTimeout        time.Duration `env:"DB_CONNECTION_TIMEOUT" envDefault:"5s"`
	DBInitializationTimeout    time.Duration `env:"DB_INITIALIZATION_TIMEOUT" envDefault:"2m"`
	JWTSecret                  string        `env:"JWT_SECRET,required,notEmpty"`
	PasswordPepper             string        `env:"PASSWORD_PEPPER,required,notEmpty"`
	PasswordHashMaxConcurrency int           `env:"PASSWORD_HASH_MAX_CONCURRENCY" envDefault:"2"`
	RedisHost                  string        `env:"REDIS_HOST,notEmpty" envDefault:"localhost"`
	RedisPort                  uint16        `env:"REDIS_PORT" envDefault:"6379"`
	RedisPassword              string        `env:"REDIS_PASSWORD"`
	RedisDB                    int           `env:"REDIS_DB" envDefault:"0"`
	RedisConnectionTimeout     time.Duration `env:"REDIS_CONNECTION_TIMEOUT" envDefault:"5s"`
	JWTExpiration              time.Duration `env:"JWT_EXPIRATION" envDefault:"5m"`
	RefreshTokenExpiration     time.Duration `env:"REFRESH_TOKEN_EXPIRATION" envDefault:"168h"`
	SessionExpiration          time.Duration `env:"SESSION_EXPIRATION" envDefault:"720h"`
	GinMode                    string        `env:"GIN_MODE" envDefault:"debug"`
	Seed                       bool          `env:"SEED" envDefault:"true"`
	AdminEmail                 string        `env:"ADMIN_EMAIL" envDefault:"admin@goat.api"`
	AdminPassword              string        `env:"ADMIN_PASSWORD" envDefault:"Qwerty@123"`
	RateLimitAuthLimit         int           `env:"RATE_LIMIT_AUTH_LIMIT" envDefault:"5"`
	RateLimitAuthWindow        time.Duration `env:"RATE_LIMIT_AUTH_WINDOW" envDefault:"1m"`
	RateLimitPublicLimit       int           `env:"RATE_LIMIT_PUBLIC_LIMIT" envDefault:"100"`
	RateLimitPublicWindow      time.Duration `env:"RATE_LIMIT_PUBLIC_WINDOW" envDefault:"1m"`
	RateLimitPrivateLimit      int           `env:"RATE_LIMIT_PRIVATE_LIMIT" envDefault:"60"`
	RateLimitPrivateWindow     time.Duration `env:"RATE_LIMIT_PRIVATE_WINDOW" envDefault:"1m"`
}

// Load reads configuration from the process environment.
func Load() (*Config, error) {
	environment := caarlosenv.ToMap(os.Environ())
	if err := validateExplicitEnvironmentValues(environment); err != nil {
		return nil, err
	}
	cfg, err := caarlosenv.ParseAsWithOptions[Config](caarlosenv.Options{
		Environment: environment,
	})
	if err != nil {
		return nil, fmt.Errorf("parse environment configuration: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateExplicitEnvironmentValues(environment map[string]string) error {
	for _, name := range nonEmptyEnvironmentVariables {
		if value, exists := environment[name]; exists && value == "" {
			return fmt.Errorf("invalid %s: must not be empty", name)
		}
	}
	return nil
}

// validate enforces cross-field and security-sensitive configuration rules.
func (c *Config) validate() error {
	if c.Env != "development" && c.Env != "production" {
		return fmt.Errorf("invalid ENV: must be 'development' or 'production'")
	}
	if c.ServerPort == 0 {
		return fmt.Errorf("invalid SERVER_PORT: must be between 1 and 65535")
	}
	if strings.TrimSpace(c.DBHost) == "" {
		return fmt.Errorf("invalid DB_HOST: must not be empty")
	}
	if c.DBPort == 0 {
		return fmt.Errorf("invalid DB_PORT: must be between 1 and 65535")
	}
	if strings.TrimSpace(c.DBUser) == "" {
		return fmt.Errorf("invalid DB_USER: must not be empty")
	}
	if c.DBPassword == "" {
		return fmt.Errorf("invalid DB_PASSWORD: must not be empty")
	}
	if strings.TrimSpace(c.DBName) == "" {
		return fmt.Errorf("invalid DB_NAME: must not be empty")
	}
	if strings.TrimSpace(c.JWTSecret) == "" {
		return fmt.Errorf("invalid JWT_SECRET: must not be empty")
	}
	if strings.TrimSpace(c.PasswordPepper) == "" {
		return fmt.Errorf("invalid PASSWORD_PEPPER: must not be empty")
	}
	if c.PasswordHashMaxConcurrency <= 0 ||
		c.PasswordHashMaxConcurrency > maximumPasswordHashConcurrency {
		return fmt.Errorf(
			"invalid PASSWORD_HASH_MAX_CONCURRENCY: must be between 1 and %d",
			maximumPasswordHashConcurrency,
		)
	}
	if strings.TrimSpace(c.RedisHost) == "" {
		return fmt.Errorf("invalid REDIS_HOST: must not be empty")
	}
	if c.RedisPort == 0 {
		return fmt.Errorf("invalid REDIS_PORT: must be between 1 and 65535")
	}
	if c.RedisDB < 0 {
		return fmt.Errorf("invalid REDIS_DB: must not be negative")
	}
	if c.GinMode != "debug" && c.GinMode != "release" {
		return fmt.Errorf("invalid GIN_MODE: must be 'debug' or 'release'")
	}
	if c.JWTExpiration < minimumAccessTokenLifetime ||
		c.JWTExpiration > maximumAccessTokenLifetime {
		return fmt.Errorf(
			"invalid JWT_EXPIRATION: must be between %s and %s",
			minimumAccessTokenLifetime,
			maximumAccessTokenLifetime,
		)
	}
	if c.RefreshTokenExpiration <= 0 {
		return fmt.Errorf("invalid REFRESH_TOKEN_EXPIRATION: must be positive")
	}
	if c.SessionExpiration <= 0 {
		return fmt.Errorf("invalid SESSION_EXPIRATION: must be positive")
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
	positiveIntegers := []struct {
		name  string
		value int
	}{
		{name: "RATE_LIMIT_AUTH_LIMIT", value: c.RateLimitAuthLimit},
		{name: "RATE_LIMIT_PUBLIC_LIMIT", value: c.RateLimitPublicLimit},
		{name: "RATE_LIMIT_PRIVATE_LIMIT", value: c.RateLimitPrivateLimit},
	}
	for _, field := range positiveIntegers {
		if field.value <= 0 {
			return fmt.Errorf("invalid %s: must be positive", field.name)
		}
	}
	positiveDurations := []struct {
		name  string
		value time.Duration
	}{
		{name: "RATE_LIMIT_AUTH_WINDOW", value: c.RateLimitAuthWindow},
		{name: "RATE_LIMIT_PUBLIC_WINDOW", value: c.RateLimitPublicWindow},
		{name: "RATE_LIMIT_PRIVATE_WINDOW", value: c.RateLimitPrivateWindow},
	}
	for _, field := range positiveDurations {
		if field.value <= 0 {
			return fmt.Errorf("invalid %s: must be positive", field.name)
		}
	}
	if !c.Seed {
		return nil
	}
	if strings.TrimSpace(c.AdminEmail) == "" {
		return fmt.Errorf("invalid ADMIN_EMAIL: must not be empty when seeding is enabled")
	}
	if c.AdminPassword == "" {
		return fmt.Errorf("invalid ADMIN_PASSWORD: must not be empty when seeding is enabled")
	}
	if err := validation.ValidatePasswordPolicy(c.AdminPassword); err != nil {
		return fmt.Errorf("invalid ADMIN_PASSWORD for administrator seeding: %w", err)
	}
	if c.IsProduction() &&
		(c.AdminEmail == defaultAdminEmail || c.AdminPassword == defaultAdminPassword) {
		return fmt.Errorf("invalid administrator seed configuration: default credentials are not allowed in production")
	}
	return nil
}

// ServerAddress returns the HTTP listen address derived from SERVER_PORT.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf(":%d", c.ServerPort)
}

// IsProduction helper
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
