package jti

import "context"

func (j *JTIUsecase) IsJTIValid(ctx context.Context, jti string) (bool, error) {
	valid, err := j.jtiRepo.Exists(ctx, jti)
	if err != nil {
		return false, err
	}

	return valid, nil
}
