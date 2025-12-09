package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type UserHandler struct {
	usecase user.UseCase
	logger  logger.Logger
}

func NewUserHandler(usecase user.UseCase, logger logger.Logger) *UserHandler {
	return &UserHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *UserHandler) GetUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")

	if targetUserID == "" {
		targetUserID = c.GetString("user_id")
		if targetUserID == "" {
			response.Unauthorized(c, "authentication context missing")
			return
		}
	}
	output, err := h.usecase.GetUser(c, targetUserID)
	if err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, output)
}

func (h *UserHandler) GetUserList(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input helper.PaginationInput
	if err := c.ShouldBindQuery(&input); err != nil {
		response.BadRequest(c, "invalid pagination params")
		return
	}
	input.Validate()
	output, total, err := h.usecase.GetUserslist(c, input.Offset(), input.Limit)
	if err != nil {
		response.Internal(c)
		return
	}
	meta := helper.NewPaginationMeta(total, input)
	response.OK(c, gin.H{"data": output, "meta": meta})
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")

	if targetUserID == "" {
		targetUserID = c.GetString("user_id")
		if targetUserID == "" {
			response.Internal(c)
			return
		}
	}

	if err := h.usecase.DeleteUser(c, targetUserID); err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, "Deleted")
}

func (h *UserHandler) UpdatePassword(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	var input user.UpdatePassInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	err := h.usecase.ChangePassword(c, user.UpdatePassInput{
		UserID:      userID,
		NewPassword: input.NewPassword,
		OldPassword: input.OldPassword,
	})

	if err != nil {
		if err == errors.ErrInvalidPassword {
			response.BadRequest(c, "current password is incorrect")
			return
		}
		if err == errors.ErrPasswordSameAsCurrent {
			response.DomainError(c, err)
			return
		}
		response.Internal(c)
		return
	}

	response.OK(c, "Password updated successfully")
}

func (h *UserHandler) UpdateRole(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	var input user.UpdateRoleInput

	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	role := valueobject.UserRole(input.Role)
	err := h.usecase.UpdateUser(c, user.UserUpdateInput{
		UserID: input.UserID,
		Role:   &role,
	})

	if err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "Role updated successfully")
}

func (h *UserHandler) UpdateStatus(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	var input user.UpdateStatusInput

	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	status := valueobject.UserStatus(input.Status)
	err := h.usecase.UpdateUser(c, user.UserUpdateInput{
		UserID: input.UserID,
		Status: &status,
	})

	if err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "Status updated successfully")
}
