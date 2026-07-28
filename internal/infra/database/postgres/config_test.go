package postgres

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewConnectionConfigMapsPostgreSQLSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  ClientConfig
	}{
		{
			name: "hostname",
			cfg:  validClientConfig(),
		},
		{
			name: "IPv4",
			cfg: func() ClientConfig {
				cfg := validClientConfig()
				cfg.Host = "192.0.2.10"
				cfg.Port = 5544
				return cfg
			}(),
		},
		{
			name: "special characters remain exact",
			cfg: func() ClientConfig {
				cfg := validClientConfig()
				cfg.User = `user name:@/\\'"`
				cfg.Password = `password :@/\\'"?#[]%`
				cfg.Database = `database name:@/\\'"?#[]%`
				return cfg
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := newConnectionConfig(test.cfg)
			if err != nil {
				t.Fatalf("newConnectionConfig() error = %v", err)
			}
			if got.Host != test.cfg.Host ||
				got.Port != test.cfg.Port ||
				got.User != test.cfg.User ||
				got.Database != test.cfg.Database {
				t.Fatalf(
					"connection identity = (%q, %d, %q, %q), want (%q, %d, %q, %q)",
					got.Host,
					got.Port,
					got.User,
					got.Database,
					test.cfg.Host,
					test.cfg.Port,
					test.cfg.User,
					test.cfg.Database,
				)
			}
			if got.Password != test.cfg.Password {
				t.Fatal("connection password was not preserved exactly")
			}
			if got.TLSConfig != nil || len(got.Fallbacks) != 0 {
				t.Fatalf("SSL mode %q configured TLS or fallbacks unexpectedly", test.cfg.SSLMode)
			}
		})
	}
}

func TestClientConfigRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ClientConfig)
	}{
		{name: "missing host", mutate: func(cfg *ClientConfig) { cfg.Host = " " }},
		{name: "missing port", mutate: func(cfg *ClientConfig) { cfg.Port = 0 }},
		{name: "missing user", mutate: func(cfg *ClientConfig) { cfg.User = " " }},
		{name: "missing password", mutate: func(cfg *ClientConfig) { cfg.Password = "" }},
		{name: "missing database", mutate: func(cfg *ClientConfig) { cfg.Database = " " }},
		{name: "unsupported SSL mode", mutate: func(cfg *ClientConfig) { cfg.SSLMode = SSLMode("prefer") }},
		{name: "zero connection timeout", mutate: func(cfg *ClientConfig) { cfg.ConnectionTimeout = 0 }},
		{name: "negative initialization timeout", mutate: func(cfg *ClientConfig) { cfg.InitializationTimeout = -time.Second }},
		{name: "missing seed email", mutate: func(cfg *ClientConfig) {
			cfg.Seed = true
			cfg.AdminEmail = " "
		}},
		{name: "missing seed password", mutate: func(cfg *ClientConfig) {
			cfg.Seed = true
			cfg.AdminPassword = ""
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := validClientConfig()
			test.mutate(&cfg)
			connectionConfig, err := newConnectionConfig(cfg)
			if err == nil {
				t.Fatal("newConnectionConfig() error = nil, want invalid configuration error")
			}
			if connectionConfig != nil {
				t.Fatal("newConnectionConfig() returned a connection config for invalid input")
			}
			for _, secret := range []string{"database-password", "administrator-password"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("newConnectionConfig() error exposed secret %q: %v", secret, err)
				}
			}
		})
	}
}

func TestClientConfigFormattingRedactsCredentials(t *testing.T) {
	t.Parallel()

	cfg := validClientConfig()
	for _, format := range []string{"%v", "%+v", "%#v"} {
		formatted := fmt.Sprintf(format, cfg)
		for _, forbidden := range []string{
			cfg.Password,
			cfg.AdminPassword,
			"postgres://",
			"password=",
		} {
			if strings.Contains(formatted, forbidden) {
				t.Fatalf("format %s exposed PostgreSQL credentials or a connection string: %s", format, formatted)
			}
		}
		if strings.Count(formatted, "<redacted>") != 2 {
			t.Fatalf("format %s = %q, want both passwords explicitly redacted", format, formatted)
		}
	}
}

func validClientConfig() ClientConfig {
	return ClientConfig{
		Host:                  "postgres.internal",
		Port:                  5432,
		User:                  "goat_user",
		Password:              "database-password",
		Database:              "goat_service",
		SSLMode:               SSLModeDisable,
		ConnectionTimeout:     5 * time.Second,
		InitializationTimeout: 2 * time.Minute,
		Seed:                  false,
		AdminEmail:            "admin@goat.api",
		AdminPassword:         "administrator-password",
	}
}
