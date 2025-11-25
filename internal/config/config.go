package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string `env:"ENV" envDefault:"development"`
	ServerPort     string `env:"SERVER_PORT" envDefault:"8080"`
	DBHost         string `env:"DB_HOST" envRequired:"true"`
	DBPort         string `env:"DB_PORT" envDefault:"5432"`
	DBUser         string `env:"DB_USER" envRequired:"true"`
	DBPassword     string `env:"DB_PASSWORD" envRequired:"true"`
	DBName         string `env:"DB_NAME" envRequired:"true"`
	JWTSecret      string `env:"JWT_SECRET" envRequired:"true"`
	PasswordPepper string `env:"PASSWORD_PEPPER" envRequired:"true"`
	RedisAddr      string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword  string `env:"REDIS_PASSWORD" envDefault:""`
	RedisDB        int    `env:"REDIS_DB"`
}

var (
	once     sync.Once
	instance *Config
)

func Get() *Config {
	once.Do(func() {
		_ = godotenv.Load()

		redisDB := mustInt(getEnv("REDIS_DB", "0"), "REDIS_DB")

		instance = &Config{
			Env:            getEnv("ENV", "development"),
			ServerPort:     ":" + getEnv("SERVER_PORT", "8080"),
			DBHost:         getEnv("DB_HOST", ""),
			DBPort:         getEnv("DB_PORT", "5432"),
			DBUser:         getEnv("DB_USER", ""),
			DBPassword:     getEnv("DB_PASSWORD", ""),
			DBName:         getEnv("DB_NAME", ""),
			JWTSecret:      getEnv("JWT_SECRET", ""),
			PasswordPepper: getEnv("PASSWORD_PEPPER", ""),
			RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
			RedisPassword:  getEnv("REDIS_PASSWORD", ""),
			RedisDB:        redisDB,
		}

		if err := instance.validate(); err != nil {
			panic(fmt.Sprintf("config validation failed: %v", err))
		}
	})
	return instance
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (c *Config) validate() error {
	required := []struct {
		name string
		val  string
	}{
		{"DB_HOST", c.DBHost},
		{"DB_USER", c.DBUser},
		{"DB_PASSWORD", c.DBPassword},
		{"DB_NAME", c.DBName},
		{"JWT_SECRET", c.JWTSecret},
		{"PASSWORD_PEPPER", c.PasswordPepper},
		{"REDIS_ADDR", c.RedisAddr},
	}

	for _, r := range required {
		if r.val == "" {
			return fmt.Errorf("%s is required", r.name)
		}
	}
	return nil
}

func (c *Config) DBConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

func mustInt(val string, name string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("invalid value for %s: %s", name, val))
	}
	return i
}
