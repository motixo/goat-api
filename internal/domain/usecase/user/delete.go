package user

import "context"

func (us *UserUseCase) DeleteUser(ctx context.Context, userID string) error {
	us.logger.Info("Attempting to delete user", "TargetUserID:", userID)
	if err := us.userRepo.Delete(ctx, userID); err != nil {
		us.logger.Error("Failed to delete user", "Error:", err)
		return err
	}
	us.logger.Info("User deleted successfully", "TargetUserID:", userID)
	return nil
}
