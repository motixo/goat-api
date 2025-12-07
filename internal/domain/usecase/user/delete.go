package user

import "context"

func (us *UserUseCase) DeleteUser(ctx context.Context, userID string) error {
	us.logger.Info("Attempting to delete user", "TargetUserID:", userID)
	if err := us.userRepo.Delete(ctx, userID); err != nil {
		us.logger.Error("Failed to delete user", "Error:", err)
		return err
	}

	sessions, err := us.sessionRepo.ListByUser(ctx, userID)
	if err != nil {
		us.logger.Error("field to fetch user sessions", "UserID:", userID)
		return nil
	}

	if len(sessions) == 0 {
		return nil
	}

	targets := make([]string, 0, len(sessions))
	for i := range sessions {
		targets[i] = sessions[i].ID
	}
	if err := us.sessionRepo.Delete(ctx, targets); err != nil {
		us.logger.Error("filed to delete user sessions", "userID:", userID)
		return nil
	}

	us.logger.Info("User deleted successfully", "TargetUserID:", userID)
	return nil
}
