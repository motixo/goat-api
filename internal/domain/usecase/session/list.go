package session

import (
	"context"
	"sort"
)

func (s *SessionUseCase) GetSessionsByUser(ctx context.Context, userID, sessionID string) ([]*SessionResponse, error) {
	sessions, err := s.sessionRepo.ListByUser(ctx, userID)
	if err != nil {
		return []*SessionResponse{}, err
	}

	response := make([]*SessionResponse, 0, len(sessions))
	for _, se := range sessions {
		r := &SessionResponse{
			ID:        se.ID,
			Device:    se.Device,
			IP:        se.IP,
			Current:   se.ID == sessionID,
			CreatedAt: se.CreatedAt,
			UpdatedAt: se.UpdatedAt,
		}

		response = append(response, r)
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].UpdatedAt.After(response[j].UpdatedAt)
	})

	return response, nil
}
