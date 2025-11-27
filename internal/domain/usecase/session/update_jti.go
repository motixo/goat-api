package session

import "context"

func (s *SessionUsecase) UpdateJTI(ctx context.Context, sessionID, jti string, ttlSeconds int) error {
	return s.sessionRepo.UpdateSessionJTI(ctx, sessionID, jti, ttlSeconds)
}
