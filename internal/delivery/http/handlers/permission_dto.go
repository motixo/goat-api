package handlers

import (
	"time"

	"github.com/motixo/goat-api/internal/usecase/permission"
)

type createPermissionRequest struct {
	Role   string `json:"role"`
	Action string `json:"action"`
}

type permissionResponse struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func newPermissionResponse(output permission.PermissionOutput) permissionResponse {
	return permissionResponse{
		ID:        output.ID,
		Role:      output.Role,
		Action:    output.Action,
		CreatedAt: output.CreatedAt,
	}
}

func newPermissionResponses(outputs []permission.PermissionOutput) []permissionResponse {
	responses := make([]permissionResponse, 0, len(outputs))
	for _, output := range outputs {
		responses = append(responses, newPermissionResponse(output))
	}
	return responses
}
