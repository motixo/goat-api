// internal/config/config.go
package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/redis/go-redis/v9"
)

// Config holds all application configuration
type Config struct {
	Env            string        `envconfig:"ENV" default:"development"`
	ServerPort     string        `envconfig:"SERVER_PORT" default:"8080"`
	DBHost         string        `envconfig:"DB_HOST" required:"true"`
	DBPort         string        `envconfig:"DB_PORT" default:"5432"`
	DBUser         string        `envconfig:"DB_USER" required:"true"`
	DBPassword     string        `envconfig:"DB_PASSWORD" required:"true"`
	DBName         string        `envconfig:"DB_NAME" required:"true"`
	JWTSecret      string        `envconfig:"JWT_SECRET" required:"true"`
	PasswordPepper string        `envconfig:"PASSWORD_PEPPER" required:"true"`
	RedisAddr      string        `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword  string        `envconfig:"REDIS_PASSWORD"`
	RedisDB        int           `envconfig:"REDIS_DB" default:"0"`
	JWTExpiration  time.Duration `envconfig:"JWT_EXPIRATION" default:"24h"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config

	// Load .env file if it exists (optional in production)
	_ = godotenv.Load()

	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Add port prefix
	cfg.ServerPort = ":" + cfg.ServerPort

	return &cfg, cfg.validate()
}

// Validate ensures required fields are set
func (c *Config) validate() error {
	if c.Env != "development" && c.Env != "production" {
		return fmt.Errorf("invalid ENV: must be 'development' or 'production'")
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
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	}
}
