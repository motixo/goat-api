package session

import (
	"time"
)

type SessionResponse struct {
	ID        string    `json:"id"`
	Device    string    `json:"device,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Current   bool      `json:"current"`
}

type CreateInput struct {
	ID         string
	UserID     string
	Device     string
	IP         string
	CurrentJTI string
	SessionTTL time.Duration
	JTITTL     time.Duration
}

type DeleteSessionsInput struct {
	UserID         string
	CurrentSession string
	TargetSessions []string `json:"session_ids"`
	RemoveOthers   bool     `json:"others"`
}

type RotateInput struct {
	OldJTI     string
	CurrentJTI string
	Device     string
	IP         string
	SessionTTL time.Duration
	JTITTL     time.Duration
}
