package entity

import "time"

type Permission struct {
	ID        string     `json:"id" db:"id"`
	RoleID    int8       `json:"role_id" db:"role_id"`
	Action    string     `json:"action" db:"action"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}
