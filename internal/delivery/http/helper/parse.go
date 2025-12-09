package helper

import "strconv"

func ParseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return int(v)
}
