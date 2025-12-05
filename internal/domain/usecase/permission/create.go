package permission

import (
	"context"
)

type CreateInput struct {
	RoleID string
	Action string
}

func (us *PermissionUseCase) Create(ctx context.Context, input CreateInput) error {
	return nil
}
