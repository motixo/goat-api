package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
)

func TestParseMigrationCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		want        migrationCommand
		wantErrText string
	}{
		{name: "apply", args: []string{"up"}, want: migrationCommandUp},
		{name: "validate", args: []string{"validate"}, want: migrationCommandValidate},
		{name: "missing", wantErrText: "usage: migrate <up|validate>"},
		{name: "extra", args: []string{"up", "extra"}, wantErrText: "usage: migrate <up|validate>"},
		{name: "unknown", args: []string{"down"}, wantErrText: "unsupported command"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseMigrationCommand(test.args)
			if test.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErrText) {
					t.Fatalf("parseMigrationCommand() error = %v, want text %q", err, test.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMigrationCommand() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("parseMigrationCommand() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNewMigrationPostgresConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		DBHost:                  "postgres.internal",
		DBPort:                  5544,
		DBUser:                  "migration_user",
		DBPassword:              "migration-secret",
		DBName:                  "goat_service",
		DBConnectionTimeout:     7 * time.Second,
		DBInitializationTimeout: 4 * time.Minute,
		Seed:                    true,
		AdminEmail:              "must-not-be-forwarded@example.com",
		AdminPassword:           "must-not-be-forwarded",
	}
	want := postgres.ClientConfig{
		Host:                  cfg.DBHost,
		Port:                  cfg.DBPort,
		User:                  cfg.DBUser,
		Password:              cfg.DBPassword,
		Database:              cfg.DBName,
		SSLMode:               postgres.SSLModeDisable,
		ConnectionTimeout:     cfg.DBConnectionTimeout,
		InitializationTimeout: cfg.DBInitializationTimeout,
	}

	got := newMigrationPostgresConfig(cfg)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("newMigrationPostgresConfig() = %#v, want %#v", got, want)
	}
	for _, forbidden := range []string{cfg.DBPassword, cfg.AdminEmail, cfg.AdminPassword} {
		if strings.Contains(got.String(), forbidden) {
			t.Fatalf("migration PostgreSQL config formatting exposed a secret or seed value: %s", got.String())
		}
	}
}
