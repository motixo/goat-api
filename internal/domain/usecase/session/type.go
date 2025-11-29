package session

import "time"

type SessionResponse struct {
	ID        string    `json:"id"`
	Device    string    `json:"device,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Current   bool      `json:"current"`
}
