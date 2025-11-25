package jti

import "context"

func (j *JTIUsecase) RevokeJTI(ctx context.Context, jti string) error {
	return j.jtiRepo.DeleteJTI(ctx, jti)
}
