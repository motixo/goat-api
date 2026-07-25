package valueobject

import (
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

// IsKnown reports whether the status is one of the defined account states.
func (r UserStatus) IsKnown() bool {
	_, ok := statusToString[r]
	return ok
}

// CanTransitionTo defines the User account-status state machine. Reapplying a
// known status is accepted so callers can safely retry an interrupted
// cross-store status workflow.
func (r UserStatus) CanTransitionTo(next UserStatus) bool {
	if !r.IsKnown() || !next.IsKnown() {
		return false
	}
	if r == next {
		return true
	}

	switch r {
	case StatusInactive:
		return next == StatusActive
	case StatusActive:
		return next == StatusSuspended
	case StatusSuspended:
		return next == StatusActive
	default:
		return false
	}
}

func ParseUserStatus(s string) (UserStatus, error) {
	r, ok := stringToStatus[s]
	if !ok {
		return 0, fmt.Errorf("invalid user role: %s", s)
	}
	return r, nil
}
