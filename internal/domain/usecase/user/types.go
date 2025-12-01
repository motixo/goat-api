package user

import "time"

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	CreatedAt time.Time `json:"createdAt"`
}
