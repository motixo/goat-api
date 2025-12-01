package valueobject_test

import (
	"testing"

	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

func TestUserStatus_String(t *testing.T) {
	tests := []struct {
		status   valueobject.UserStatus
		expected string
	}{
		{valueobject.StatusInactive, "inactive"},
		{valueobject.StatusActive, "active"},
		{valueobject.StatusSuspended, "suspended"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.status.String())
		}
	}
}

func TestUserStatus_IsValid(t *testing.T) {
	validStatuses := []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected status %d to be valid", s)
		}
	}

	invalidStatus := valueobject.UserStatus(100)
	if invalidStatus.IsValid() {
		t.Errorf("expected status %d to be invalid", invalidStatus)
	}
}
