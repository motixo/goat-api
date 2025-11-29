package dto

import "time"

type Session struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	IP                string    `json:"ip"`
	Device            string    `json:"device"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	CurrentJTI        string    `json:"current_jti"`
	JTITTLSeconds     int       `json:"jti_ttl_seconds"`
	SessionTTLSeconds int       `json:"session_ttl_seconds"`
}
