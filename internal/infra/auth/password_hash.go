package service

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	domainService "github.com/motixo/goat-api/internal/domain/service"
)

const (
	argonAlgorithm = "argon2id"
	argonVersion   = 19

	// Generated hashes are 118 bytes. The small headroom keeps parsing work
	// bounded without changing the one supported encoded-hash contract.
	maxEncodedPasswordHashLength = 128

	// This adapter intentionally accepts only the parameters it emits. There is
	// no legacy format or automatic cost migration in this base project.
	minimumArgonMemoryCost  = argonMemory
	maximumArgonMemoryCost  = argonMemory
	minimumArgonTimeCost    = argonTime
	maximumArgonTimeCost    = argonTime
	minimumArgonThreads     = argonThreads
	maximumArgonThreads     = argonThreads
	minimumSaltLength       = saltLen
	maximumSaltLength       = saltLen
	minimumDerivedKeyLength = int(argonKeyLen)
	maximumDerivedKeyLength = int(argonKeyLen)
)

type parsedArgon2idHash struct {
	memory     uint32
	time       uint32
	threads    uint8
	salt       []byte
	derivedKey []byte
}

func parseArgon2idHash(encoded string) (parsedArgon2idHash, error) {
	if len(encoded) == 0 {
		return parsedArgon2idHash{}, invalidStoredPasswordHash("encoded value is empty")
	}
	if len(encoded) > maxEncodedPasswordHashLength {
		return parsedArgon2idHash{}, invalidStoredPasswordHash("encoded value exceeds the supported length")
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return parsedArgon2idHash{}, invalidStoredPasswordHash("encoded structure is invalid")
	}
	if parts[1] != argonAlgorithm {
		return parsedArgon2idHash{}, invalidStoredPasswordHash("algorithm is unsupported")
	}

	version, err := parseNamedUint(parts[2], "v", 32)
	if err != nil || version != argonVersion {
		return parsedArgon2idHash{}, invalidStoredPasswordHash("version is unsupported")
	}

	memory, timeCost, threads, err := parseArgon2idParameters(parts[3])
	if err != nil {
		return parsedArgon2idHash{}, err
	}

	salt, err := decodeFixedRawBase64(parts[4], minimumSaltLength, maximumSaltLength, "salt")
	if err != nil {
		return parsedArgon2idHash{}, err
	}
	derivedKey, err := decodeFixedRawBase64(
		parts[5],
		minimumDerivedKeyLength,
		maximumDerivedKeyLength,
		"derived key",
	)
	if err != nil {
		return parsedArgon2idHash{}, err
	}

	return parsedArgon2idHash{
		memory:     memory,
		time:       timeCost,
		threads:    threads,
		salt:       salt,
		derivedKey: derivedKey,
	}, nil
}

func parseArgon2idParameters(encoded string) (uint32, uint32, uint8, error) {
	fields := strings.Split(encoded, ",")
	if len(fields) != 3 {
		return 0, 0, 0, invalidStoredPasswordHash("parameter set is incomplete")
	}

	var (
		memory      uint32
		timeCost    uint32
		threads     uint8
		seenMemory  bool
		seenTime    bool
		seenThreads bool
	)
	for _, field := range fields {
		name, _, ok := strings.Cut(field, "=")
		if !ok {
			return 0, 0, 0, invalidStoredPasswordHash("parameter is malformed")
		}

		switch name {
		case "m":
			if seenMemory {
				return 0, 0, 0, invalidStoredPasswordHash("memory parameter is duplicated")
			}
			value, parseErr := parseNamedUint(field, "m", 32)
			if parseErr != nil || value < uint64(minimumArgonMemoryCost) || value > uint64(maximumArgonMemoryCost) {
				return 0, 0, 0, invalidStoredPasswordHash("memory cost is unsupported")
			}
			memory = uint32(value)
			seenMemory = true
		case "t":
			if seenTime {
				return 0, 0, 0, invalidStoredPasswordHash("time parameter is duplicated")
			}
			value, parseErr := parseNamedUint(field, "t", 32)
			if parseErr != nil || value < uint64(minimumArgonTimeCost) || value > uint64(maximumArgonTimeCost) {
				return 0, 0, 0, invalidStoredPasswordHash("time cost is unsupported")
			}
			timeCost = uint32(value)
			seenTime = true
		case "p":
			if seenThreads {
				return 0, 0, 0, invalidStoredPasswordHash("parallelism parameter is duplicated")
			}
			value, parseErr := parseNamedUint(field, "p", 8)
			if parseErr != nil || value < uint64(minimumArgonThreads) || value > uint64(maximumArgonThreads) {
				return 0, 0, 0, invalidStoredPasswordHash("parallelism is unsupported")
			}
			threads = uint8(value)
			seenThreads = true
		default:
			return 0, 0, 0, invalidStoredPasswordHash("parameter is unknown")
		}
	}
	if !seenMemory || !seenTime || !seenThreads {
		return 0, 0, 0, invalidStoredPasswordHash("parameter set is incomplete")
	}

	return memory, timeCost, threads, nil
}

func parseNamedUint(field, name string, bitSize int) (uint64, error) {
	prefix := name + "="
	if !strings.HasPrefix(field, prefix) || len(field) == len(prefix) {
		return 0, invalidStoredPasswordHash("numeric field is malformed")
	}
	raw := field[len(prefix):]
	for index := range len(raw) {
		if raw[index] < '0' || raw[index] > '9' {
			return 0, invalidStoredPasswordHash("numeric field is malformed")
		}
	}
	value, err := strconv.ParseUint(raw, 10, bitSize)
	if err != nil {
		return 0, invalidStoredPasswordHash("numeric field is out of range")
	}
	return value, nil
}

func decodeFixedRawBase64(encoded string, minimumLength, maximumLength int, field string) ([]byte, error) {
	minimumEncodedLength := base64.RawStdEncoding.EncodedLen(minimumLength)
	maximumEncodedLength := base64.RawStdEncoding.EncodedLen(maximumLength)
	if len(encoded) < minimumEncodedLength || len(encoded) > maximumEncodedLength {
		return nil, invalidStoredPasswordHash(field + " length is unsupported")
	}
	for index := range len(encoded) {
		character := encoded[index]
		if !isRawBase64Character(character) {
			return nil, invalidStoredPasswordHash(field + " encoding is invalid")
		}
	}

	decoded, err := base64.RawStdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, invalidStoredPasswordHash(field + " encoding is invalid")
	}
	if len(decoded) < minimumLength || len(decoded) > maximumLength {
		return nil, invalidStoredPasswordHash(field + " length is unsupported")
	}
	return decoded, nil
}

func isRawBase64Character(character byte) bool {
	return character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9' ||
		character == '+' || character == '/'
}

func invalidStoredPasswordHash(reason string) error {
	return fmt.Errorf("%w: %s", domainService.ErrInvalidStoredPasswordHash, reason)
}
