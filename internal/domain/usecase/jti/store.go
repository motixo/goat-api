package jti

import (
	"context"
	"time"
)

type StoreInput struct {
	UserID string
	JTI    string
	Exp    time.Duration
}

func (j *JTIUsecase) StoreJTI(ctx context.Context, input StoreInput) error {
	ttlSeconds := int(input.Exp.Seconds())
	return j.jtiRepo.SaveJTI(ctx, input.UserID, input.JTI, ttlSeconds)
}
