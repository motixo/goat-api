package postgres

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SSLMode identifies the PostgreSQL transport-security mode supported by the adapter.
type SSLMode string

const (
	// SSLModeDisable preserves the existing unencrypted PostgreSQL connection behavior.
	SSLModeDisable SSLMode = "disable"
)

// ClientConfig contains only the settings required to construct and initialize PostgreSQL.
type ClientConfig struct {
	Host                  string
	Port                  uint16
	User                  string
	Password              string
	Database              string
	SSLMode               SSLMode
	ConnectionTimeout     time.Duration
	InitializationTimeout time.Duration
	Seed                  bool
	AdminEmail            string
	AdminPassword         string
}

// String formats the PostgreSQL client configuration without exposing credentials.
func (c ClientConfig) String() string {
	return fmt.Sprintf(
		"{Host:%q Port:%d User:%q Password:<redacted> Database:%q SSLMode:%q ConnectionTimeout:%s InitializationTimeout:%s Seed:%t AdminEmail:%q AdminPassword:<redacted>}",
		c.Host,
		c.Port,
		c.User,
		c.Database,
		c.SSLMode,
		c.ConnectionTimeout,
		c.InitializationTimeout,
		c.Seed,
		c.AdminEmail,
	)
}

// GoString formats the PostgreSQL client configuration without exposing credentials.
func (c ClientConfig) GoString() string {
	return c.String()
}

func (c ClientConfig) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("PostgreSQL host is required")
	}
	if c.Port == 0 {
		return errors.New("PostgreSQL port must be positive")
	}
	if strings.TrimSpace(c.User) == "" {
		return errors.New("PostgreSQL user is required")
	}
	if c.Password == "" {
		return errors.New("PostgreSQL password is required")
	}
	if strings.TrimSpace(c.Database) == "" {
		return errors.New("PostgreSQL database is required")
	}
	if c.SSLMode != SSLModeDisable {
		return fmt.Errorf("unsupported PostgreSQL SSL mode %q", c.SSLMode)
	}
	if c.ConnectionTimeout <= 0 {
		return errors.New("PostgreSQL connection timeout must be positive")
	}
	if c.InitializationTimeout <= 0 {
		return errors.New("PostgreSQL initialization timeout must be positive")
	}
	if !c.Seed {
		return nil
	}
	if strings.TrimSpace(c.AdminEmail) == "" {
		return errors.New("PostgreSQL administrator seed email is required")
	}
	if c.AdminPassword == "" {
		return errors.New("PostgreSQL administrator seed password is required")
	}
	return nil
}

func newConnectionConfig(cfg ClientConfig) (*pgx.ConnConfig, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Parse only non-sensitive adapter-owned options, then assign credentials
	// structurally so no credential-bearing DSN needs to be constructed.
	connectionConfig, err := pgx.ParseConfig("sslmode=" + string(cfg.SSLMode))
	if err != nil {
		return nil, fmt.Errorf("construct PostgreSQL connection configuration: %w", err)
	}
	connectionConfig.Host = cfg.Host
	connectionConfig.Port = cfg.Port
	connectionConfig.User = cfg.User
	connectionConfig.Password = cfg.Password
	connectionConfig.Database = cfg.Database
	return connectionConfig, nil
}
