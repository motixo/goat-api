package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	stdErrors "errors"
	"fmt"
	"strings"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"golang.org/x/crypto/argon2"
)

var errPasswordHashingContextRequired = stdErrors.New("password hashing context is required")

const (
	argonTime                      uint32 = 3
	argonMemory                    uint32 = 64 * 1024
	argonThreads                   uint8  = 4
	argonKeyLen                    uint32 = 32
	saltLen                               = 32
	maximumPasswordHashConcurrency        = 4
)

type PasswordHasherConfig struct {
	Pepper         string
	MaxConcurrency int
}

func (c PasswordHasherConfig) String() string {
	return fmt.Sprintf(
		"PasswordHasherConfig{pepper:<redacted>,maxConcurrency:%d}",
		c.MaxConcurrency,
	)
}

func (c PasswordHasherConfig) GoString() string {
	return c.String()
}

type deriveKeyFunc func(
	password []byte,
	salt []byte,
	time uint32,
	memory uint32,
	threads uint8,
	keyLen uint32,
) []byte

type saltReaderFunc func([]byte) (int, error)

type PasswordService struct {
	pepper   string
	capacity chan struct{}
	readSalt saltReaderFunc
	derive   deriveKeyFunc
}

var _ service.PasswordHasher = (*PasswordService)(nil)

func NewPasswordService(cfg PasswordHasherConfig) (*PasswordService, error) {
	return newPasswordService(cfg, rand.Read, argon2.IDKey)
}

func newPasswordService(
	cfg PasswordHasherConfig,
	readSalt saltReaderFunc,
	derive deriveKeyFunc,
) (*PasswordService, error) {
	if strings.TrimSpace(cfg.Pepper) == "" {
		return nil, stdErrors.New("password hasher pepper is required")
	}
	if cfg.MaxConcurrency <= 0 {
		return nil, stdErrors.New("password hasher max concurrency must be positive")
	}
	if cfg.MaxConcurrency > maximumPasswordHashConcurrency {
		return nil, fmt.Errorf(
			"password hasher max concurrency must not exceed %d",
			maximumPasswordHashConcurrency,
		)
	}
	return &PasswordService{
		pepper:   cfg.Pepper,
		capacity: make(chan struct{}, cfg.MaxConcurrency),
		readSalt: readSalt,
		derive:   derive,
	}, nil
}

func (s *PasswordService) acquire(ctx context.Context) error {
	if err := callerContextError(ctx); err != nil {
		return err
	}
	select {
	case s.capacity <- struct{}{}:
		if err := callerContextError(ctx); err != nil {
			s.release()
			return err
		}
		return nil
	case <-ctx.Done():
		return callerContextError(ctx)
	}
}

func (s *PasswordService) release() {
	<-s.capacity
}

func callerContextError(ctx context.Context) error {
	if ctx == nil {
		return errPasswordHashingContextRequired
	}
	ctxErr := ctx.Err()
	if ctxErr == nil {
		return nil
	}
	cause := context.Cause(ctx)
	if cause == nil || stdErrors.Is(ctxErr, cause) {
		return ctxErr
	}
	return stdErrors.Join(ctxErr, cause)
}

func (*PasswordService) String() string {
	return "PasswordService{pepper:<redacted>}"
}

func (s *PasswordService) GoString() string {
	return s.String()
}

func (s *PasswordService) Hash(
	ctx context.Context,
	password valueobject.PlainPassword,
) (valueobject.PasswordDigest, error) {
	if err := callerContextError(ctx); err != nil {
		return valueobject.PasswordDigest{}, err
	}
	if err := password.Validate(); err != nil {
		return valueobject.PasswordDigest{}, err
	}
	if err := s.acquire(ctx); err != nil {
		return valueobject.PasswordDigest{}, err
	}
	defer s.release()

	salt := make([]byte, saltLen)
	if _, err := s.readSalt(salt); err != nil {
		return valueobject.PasswordDigest{}, domainErrors.ErrPasswordHashingFailed
	}
	if err := callerContextError(ctx); err != nil {
		return valueobject.PasswordDigest{}, err
	}

	input := append(password.Bytes(), []byte(s.pepper)...)
	hash := s.derive(input, salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonAlgorithm, argonVersion,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))

	digest, err := valueobject.NewPasswordDigest(encoded)
	if err != nil {
		return valueobject.PasswordDigest{}, fmt.Errorf("construct generated password digest: %w", err)
	}
	return digest, nil
}

func (s *PasswordService) Verify(
	ctx context.Context,
	password valueobject.PlainPassword,
	digest valueobject.PasswordDigest,
) (bool, error) {
	if err := callerContextError(ctx); err != nil {
		return false, err
	}
	if err := password.Validate(); err != nil {
		return false, err
	}
	parsed, err := parseArgon2idHash(digest.Encoded())
	if err != nil {
		return false, err
	}
	if err := s.acquire(ctx); err != nil {
		return false, err
	}
	defer s.release()
	if err := callerContextError(ctx); err != nil {
		return false, err
	}

	input := append(password.Bytes(), []byte(s.pepper)...)
	hash := s.derive(
		input,
		parsed.salt,
		parsed.time,
		parsed.memory,
		parsed.threads,
		uint32(len(parsed.derivedKey)),
	)

	return subtle.ConstantTimeCompare(hash, parsed.derivedKey) == 1, nil
}
