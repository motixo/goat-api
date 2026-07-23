package session

import (
	"time"
)

type SessionOutput struct {
	ID        string
	Device    string
	IP        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Current   bool
}

type CreateInput struct {
	ID                string
	UserID            string
	Device            string
	IP                string
	CurrentJTI        string
	CredentialVersion int64
	SessionTTL        time.Duration
	JTITTL            time.Duration
}

type DeleteSessionsInput struct {
	UserID         string
	CurrentSession string
	TargetSessions []string
	RemoveOthers   bool
}

type RotateInput struct {
	UserID     string
	OldJTI     string
	CurrentJTI string
	Device     string
	IP         string
	SessionTTL time.Duration
	JTITTL     time.Duration
}

type ValidateInput struct {
	UserID    string
	SessionID string
	JTI       string
}
