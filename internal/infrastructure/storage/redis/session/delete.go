package session

import (
	"context"
)

func (r *Repository) Delete(ctx context.Context, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	sessionKeys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionKeys = append(sessionKeys, r.key("session", sessionID))
	}

	script := getScript("delete_session")
	_, err := script.Run(ctx, r.client, sessionKeys).Result()
	return err
}
