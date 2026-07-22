package handlers

import (
	"time"

	"github.com/motixo/goat-api/internal/usecase/session"
)

type deleteSessionsRequest struct {
	SessionIDs []string `json:"session_ids"`
	Others     bool     `json:"others"`
}

type sessionResponse struct {
	ID        string    `json:"id"`
	Device    string    `json:"device,omitempty"`
	IP        string    `json:"ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Current   bool      `json:"current"`
}

func newSessionResponses(output []session.SessionOutput) []sessionResponse {
	responses := make([]sessionResponse, 0, len(output))
	for _, item := range output {
		responses = append(responses, sessionResponse{
			ID:        item.ID,
			Device:    item.Device,
			IP:        item.IP,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Current:   item.Current,
		})
	}
	return responses
}
