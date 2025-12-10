package helper

import "strconv"

func ParseInt64(s string, fallback int64) int64 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return int64(v)
}

func ParseInt8(s string, fallback int8) int8 {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 8)
	if err != nil || v <= 0 {
		return fallback
	}
	return int8(v)
}
