package repository

import (
	"context"
)

type RoleRepository interface {
	//Create(ctx context.Context, userID string, roleId int8) error
	GetByUserID(ctx context.Context, userID string) (int8, error)
}
