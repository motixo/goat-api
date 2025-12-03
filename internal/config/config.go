package config

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
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
	GinMode        string        `envconfig:"GIN_MODE" default:"debug"`
	IsSeeded       int           `envconfig:"SEEDED" default:"1"`
	AdminEmail     string        `envconfig:"ADMIN_EMAIL" default:"admin@goth.api"`
	AdminPassword  string        `envconfig:"ADMIN_PASSWORD" default:"Qwerty@123"`
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

// IsProduction helper
func (c *Config) IsProduction() bool {
	return c.Env == "production"
}
