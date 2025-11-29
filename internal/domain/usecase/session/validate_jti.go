package session

import "context"

func (s *SessionUseCase) IsJTIValid(ctx context.Context, jti string) (bool, error) {
	valid, err := s.sessionRepo.ExistsJTI(ctx, jti)
	if err != nil {
		return false, err
	}

	return valid, nil
}
