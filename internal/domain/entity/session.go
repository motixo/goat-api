package entity

import "time"

type Session struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	RoleID            int8      `json:"role_id"`
	Device            string    `json:"device,omitempty"`
	IP                string    `json:"ip,omitempty"`
	CurrentJTI        string    `json:"current_jti,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	JTITTLSeconds     int64     `json:"jti_ttl_seconds"`
	SessionTTLSeconds int64     `json:"session_ttl_seconds"`
}
