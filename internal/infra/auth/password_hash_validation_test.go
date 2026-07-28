package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	domainService "github.com/motixo/goat-api/internal/domain/service"
)

func TestParseArgon2idHashAcceptsGeneratedContract(t *testing.T) {
	encoded := encodedPassword(fixedDerivedKey(argonKeyLen)).Encoded()
	parsed, err := parseArgon2idHash(encoded)
	if err != nil {
		t.Fatalf("parseArgon2idHash() error = %v", err)
	}
	if parsed.memory != argonMemory || parsed.time != argonTime || parsed.threads != argonThreads {
		t.Fatalf(
			"parsed costs = memory:%d time:%d threads:%d, want %d/%d/%d",
			parsed.memory,
			parsed.time,
			parsed.threads,
			argonMemory,
			argonTime,
			argonThreads,
		)
	}
	if len(parsed.salt) != saltLen {
		t.Fatalf("parsed salt length = %d, want %d", len(parsed.salt), saltLen)
	}
	if len(parsed.derivedKey) != int(argonKeyLen) {
		t.Fatalf("parsed derived-key length = %d, want %d", len(parsed.derivedKey), argonKeyLen)
	}
	if !bytes.Equal(parsed.derivedKey, fixedDerivedKey(argonKeyLen)) {
		t.Fatal("parsed derived key changed")
	}
	if got := len(encoded); got != 118 {
		t.Fatalf("generated encoded-hash length = %d, want 118", got)
	}
}

func TestPasswordServiceRejectsInvalidEncodedHashesBeforeAdmission(t *testing.T) {
	validEncodedHash := encodedPassword(fixedDerivedKey(argonKeyLen)).Encoded()
	validParameters := fmt.Sprintf("m=%d,t=%d,p=%d", argonMemory, argonTime, argonThreads)
	tests := []struct {
		name    string
		encoded string
	}{
		{name: "empty", encoded: ""},
		{
			name:    "unsupported algorithm",
			encoded: strings.Replace(validEncodedHash, "$argon2id$", "$argon2i$", 1),
		},
		{
			name:    "unsupported version",
			encoded: strings.Replace(validEncodedHash, "$v=19$", "$v=16$", 1),
		},
		{
			name:    "overflowing version",
			encoded: strings.Replace(validEncodedHash, "$v=19$", "$v=4294967296$", 1),
		},
		{
			name:    "missing parameter",
			encoded: replaceEncodedHashField(validEncodedHash, 3, fmt.Sprintf("m=%d,t=%d", argonMemory, argonTime)),
		},
		{
			name:    "duplicate parameter",
			encoded: replaceEncodedHashField(validEncodedHash, 3, fmt.Sprintf("m=%d,m=%d,p=%d", argonMemory, argonMemory, argonThreads)),
		},
		{
			name:    "unknown parameter",
			encoded: replaceEncodedHashField(validEncodedHash, 3, fmt.Sprintf("m=%d,t=%d,x=%d", argonMemory, argonTime, argonThreads)),
		},
		{
			name:    "zero memory",
			encoded: replaceEncodedHashField(validEncodedHash, 3, strings.Replace(validParameters, "m=65536", "m=0", 1)),
		},
		{
			name:    "zero time",
			encoded: replaceEncodedHashField(validEncodedHash, 3, strings.Replace(validParameters, "t=3", "t=0", 1)),
		},
		{
			name:    "zero parallelism",
			encoded: replaceEncodedHashField(validEncodedHash, 3, strings.Replace(validParameters, "p=4", "p=0", 1)),
		},
		{
			name:    "excessive memory",
			encoded: strings.Replace(validEncodedHash, "m=65536", "m=65537", 1),
		},
		{
			name:    "excessive time",
			encoded: strings.Replace(validEncodedHash, "t=3", "t=4", 1),
		},
		{
			name:    "excessive parallelism",
			encoded: strings.Replace(validEncodedHash, "p=4", "p=5", 1),
		},
		{
			name:    "integer overflow",
			encoded: strings.Replace(validEncodedHash, "m=65536", "m=4294967296", 1),
		},
		{
			name:    "negative integer",
			encoded: strings.Replace(validEncodedHash, "m=65536", "m=-1", 1),
		},
		{name: "invalid salt base64", encoded: replaceEncodedHashField(validEncodedHash, 4, strings.Repeat("!", 43))},
		{name: "empty salt", encoded: replaceEncodedHashField(validEncodedHash, 4, "")},
		{name: "short salt", encoded: replaceEncodedHashField(validEncodedHash, 4, encodedBytes(saltLen-1))},
		{name: "long salt", encoded: replaceEncodedHashField(validEncodedHash, 4, encodedBytes(saltLen+1))},
		{name: "invalid derived-key base64", encoded: replaceEncodedHashField(validEncodedHash, 5, strings.Repeat("!", 43))},
		{name: "empty derived key", encoded: replaceEncodedHashField(validEncodedHash, 5, "")},
		{name: "short derived key", encoded: replaceEncodedHashField(validEncodedHash, 5, encodedBytes(int(argonKeyLen)-1))},
		{name: "long derived key", encoded: replaceEncodedHashField(validEncodedHash, 5, encodedBytes(int(argonKeyLen)+1))},
		{name: "truncated hash", encoded: validEncodedHash[:len(validEncodedHash)-1]},
		{name: "trailing field", encoded: validEncodedHash + "$trailing"},
		{name: "missing leading delimiter", encoded: strings.TrimPrefix(validEncodedHash, "$")},
		{name: "excessive encoded input", encoded: strings.Repeat("x", maxEncodedPasswordHashLength+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var deriveCalls atomic.Int32
			ctx := newAdmissionObservedContext(context.Background())
			passwordService := mustNewControlledPasswordService(
				t,
				1,
				nil,
				func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
					deriveCalls.Add(1)
					return fixedDerivedKey(argonKeyLen)
				},
			)

			verified, err := passwordService.Verify(
				ctx,
				testPlainPassword("Password1!"),
				testPasswordDigest(test.encoded),
			)
			if !errors.Is(err, domainService.ErrInvalidStoredPasswordHash) {
				t.Fatalf("Verify() error = %v, want ErrInvalidStoredPasswordHash", err)
			}
			if verified {
				t.Fatal("Verify() accepted an unsafe persisted hash")
			}
			if got := deriveCalls.Load(); got != 0 {
				t.Fatalf("Argon2 derivations = %d, want 0", got)
			}
			if got := ctx.checks.Load(); got != 1 {
				t.Fatalf("caller-context checks = %d, want 1 before parser rejection", got)
			}
			if got := len(passwordService.capacity); got != 0 {
				t.Fatalf("occupied hashing slots = %d, want 0", got)
			}
		})
	}
}

func TestInvalidStoredPasswordHashIdentitySurvivesWrappingWithoutSecretLeakage(t *testing.T) {
	const (
		plaintext = "Password1!"
		pepper    = "parser-error-secret-pepper"
	)
	validEncodedHash := encodedPassword(fixedDerivedKey(argonKeyLen)).Encoded()
	invalidEncodedHash := replaceEncodedHashField(validEncodedHash, 4, strings.Repeat("!", 43))
	passwordService := mustNewPasswordService(t, pepper, 1)

	_, err := passwordService.Verify(
		context.Background(),
		testPlainPassword(plaintext),
		testPasswordDigest(invalidEncodedHash),
	)
	if !errors.Is(err, domainService.ErrInvalidStoredPasswordHash) {
		t.Fatalf("Verify() error = %v, want ErrInvalidStoredPasswordHash", err)
	}
	wrapped := fmt.Errorf("verify persisted credential: %w", err)
	if !errors.Is(wrapped, domainService.ErrInvalidStoredPasswordHash) {
		t.Fatalf("wrapped error = %v, want ErrInvalidStoredPasswordHash identity", wrapped)
	}
	for _, secret := range []string{
		plaintext,
		pepper,
		invalidEncodedHash,
		strings.Split(invalidEncodedHash, "$")[4],
		strings.Split(invalidEncodedHash, "$")[5],
	} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("Verify() error exposed password, pepper, or encoded credential material")
		}
	}
}

func FuzzParseArgon2idHash(f *testing.F) {
	validEncodedHash := encodedPassword(fixedDerivedKey(argonKeyLen)).Encoded()
	f.Add(validEncodedHash)
	f.Add("")
	f.Add("$argon2id$v=19$m=4294967296,t=3,p=4$$")
	f.Add(strings.Repeat("x", maxEncodedPasswordHashLength+1))

	f.Fuzz(func(t *testing.T, encoded string) {
		parsed, err := parseArgon2idHash(encoded)
		if err != nil {
			if !errors.Is(err, domainService.ErrInvalidStoredPasswordHash) {
				t.Fatalf("parser error does not preserve ErrInvalidStoredPasswordHash: %v", err)
			}
			return
		}
		if len(encoded) > maxEncodedPasswordHashLength ||
			parsed.memory < minimumArgonMemoryCost || parsed.memory > maximumArgonMemoryCost ||
			parsed.time < minimumArgonTimeCost || parsed.time > maximumArgonTimeCost ||
			parsed.threads < minimumArgonThreads || parsed.threads > maximumArgonThreads ||
			len(parsed.salt) < minimumSaltLength || len(parsed.salt) > maximumSaltLength ||
			len(parsed.derivedKey) < minimumDerivedKeyLength || len(parsed.derivedKey) > maximumDerivedKeyLength {
			t.Fatal("parser accepted a value outside the supported resource bounds")
		}
	})
}

func FuzzPasswordServiceInvalidHashesNeverReachDerivation(f *testing.F) {
	validEncodedHash := encodedPassword(fixedDerivedKey(argonKeyLen)).Encoded()
	f.Add(validEncodedHash)
	f.Add("invalid")
	f.Add(strings.Replace(validEncodedHash, "m=65536", "m=4294967296", 1))

	f.Fuzz(func(t *testing.T, encoded string) {
		_, parseErr := parseArgon2idHash(encoded)
		var deriveCalls atomic.Int32
		passwordService := mustNewControlledPasswordService(
			t,
			1,
			nil,
			func([]byte, []byte, uint32, uint32, uint8, uint32) []byte {
				deriveCalls.Add(1)
				return fixedDerivedKey(argonKeyLen)
			},
		)
		_, verifyErr := passwordService.Verify(
			context.Background(),
			testPlainPassword("Password1!"),
			testPasswordDigest(encoded),
		)
		if parseErr != nil {
			if !errors.Is(verifyErr, domainService.ErrInvalidStoredPasswordHash) {
				t.Fatalf("Verify() error = %v, want invalid stored-hash identity", verifyErr)
			}
			if got := deriveCalls.Load(); got != 0 {
				t.Fatalf("invalid persisted hash reached %d derivations", got)
			}
			return
		}
		if verifyErr != nil {
			t.Fatalf("Verify() rejected a parser-approved hash: %v", verifyErr)
		}
		if got := deriveCalls.Load(); got != 1 {
			t.Fatalf("valid persisted hash reached %d derivations, want 1", got)
		}
	})
}

func replaceEncodedHashField(encoded string, index int, replacement string) string {
	parts := strings.Split(encoded, "$")
	parts[index] = replacement
	return strings.Join(parts, "$")
}

func encodedBytes(length int) string {
	return base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, length))
}
