package session

import "time"

type SessionResponse struct {
	UserID    string    `json:"user_id"`
	Device    string    `json:"device,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	UpdateAt  time.Time `json:"updated_at"`
}
