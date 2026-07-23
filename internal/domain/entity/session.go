package entity

import "time"

type Session struct {
	ID                string
	UserID            string
	CredentialVersion int64
	Device            string
	IP                string
	CurrentJTI        string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	UpdatedAt         time.Time
	JTITTLSeconds     int64
	SessionTTLSeconds int64
}
