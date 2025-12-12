package valueobject

import (
	"encoding/json"
	"fmt"
)

type UserStatus uint8

const (
	StatusUnknown UserStatus = iota
	StatusInactive
	StatusActive
	StatusSuspended
)

var statusToString = map[UserStatus]string{
	StatusInactive:  "inactive",
	StatusActive:    "active",
	StatusSuspended: "suspended",
}

var stringToStatus = map[string]UserStatus{
	"inactive":  StatusInactive,
	"active":    StatusActive,
	"suspended": StatusSuspended,
}

func (r UserStatus) String() string {
	s, ok := statusToString[r]
	if !ok {
		return "unknown"
	}
	return s
}

func ParseUserStatus(s string) (UserStatus, error) {
	r, ok := stringToStatus[s]
	if !ok {
		return 0, fmt.Errorf("invalid user role: %s", s)
	}
	return r, nil
}

// UnmarshalJSON allows parsing string status from JSON
func (r *UserStatus) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("user role must be a string")
	}

	parsedStatus, err := ParseUserStatus(s)
	if err != nil {
		return err
	}

	*r = parsedStatus
	return nil
}
