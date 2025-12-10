package permission

import "time"

type CreateInput struct {
	RoleID int8   `json:"role_id"`
	Action string `json:"action"`
}

type PermissionResponse struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}
