package jti

import (
	"context"
)

type JTIUseCase interface {
	StoreJTI(ctx context.Context, input StoreInput) error
	RevokeJTI(ctx context.Context, jti string) error
	IsJTIValid(ctx context.Context, jti string) (bool, error)
}
