package entity

import "time"

type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Device     string    `json:"device,omitempty"`
	IP         string    `json:"ip,omitempty"`
	CurrentJTI string    `json:"current_jti,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	UpdateAt   time.Time `json:"updated_at"`
}
