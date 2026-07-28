package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type migrationCommand string

const (
	migrationCommandUp       migrationCommand = "up"
	migrationCommandValidate migrationCommand = "validate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("database migration command failed: %v", err)
		os.Exit(1)
	}
}

func run(args []string) (err error) {
	command, err := parseMigrationCommand(args)
	if err != nil {
		return err
	}

	ctx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	appLogger, err := logger.NewZapLogger()
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer func() {
		if syncErr := appLogger.Sync(); syncErr != nil {
			err = errors.Join(err, fmt.Errorf("sync migration logger: %w", syncErr))
		}
	}()

	postgresConfig := newMigrationPostgresConfig(cfg)
	var result postgres.MigrationResult
	switch command {
	case migrationCommandUp:
		result, err = postgres.Migrate(ctx, postgresConfig, appLogger)
	case migrationCommandValidate:
		result, err = postgres.ValidateMigrations(ctx, postgresConfig, appLogger)
	default:
		return fmt.Errorf("unsupported migration command %q", command)
	}
	if err != nil {
		return fmt.Errorf("%s PostgreSQL migrations: %w", command, err)
	}

	appLogger.Info(
		"PostgreSQL migration command completed",
		"command", command,
		"applied", result.Applied,
		"current_version", result.CurrentVersion,
	)
	return nil
}

func parseMigrationCommand(args []string) (migrationCommand, error) {
	if len(args) != 1 {
		return "", errors.New("usage: migrate <up|validate>")
	}
	command := migrationCommand(args[0])
	if command != migrationCommandUp && command != migrationCommandValidate {
		return "", fmt.Errorf("usage: migrate <up|validate>: unsupported command %q", args[0])
	}
	return command, nil
}

func newMigrationPostgresConfig(cfg *config.Config) postgres.ClientConfig {
	return postgres.ClientConfig{
		Host:                  cfg.DBHost,
		Port:                  cfg.DBPort,
		User:                  cfg.DBUser,
		Password:              cfg.DBPassword,
		Database:              cfg.DBName,
		SSLMode:               postgres.SSLModeDisable,
		ConnectionTimeout:     cfg.DBConnectionTimeout,
		InitializationTimeout: cfg.DBInitializationTimeout,
	}
}
