package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/user"
)

type UserHandler struct {
	usecase user.UseCase
	logger  pkg.Logger
}

func NewUserHandler(usecase user.UseCase, logger pkg.Logger) *UserHandler {
	return &UserHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request createUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.CreateUser(c, user.CreateInput{
		Email:    request.Email,
		Password: request.Password,
		Status:   request.Status,
		Role:     request.Role,
	})
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.Created(c, newUserResponse(output))
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
	response.OK(c, newUserResponse(output))
}

func (h *UserHandler) GetUserList(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input listUsersQuery
	if err := c.ShouldBindQuery(&input); err != nil {
		response.BadRequest(c, "invalid pagination params")
		return
	}
	input.PaginationInput.Validate()

	actorID := c.GetString("user_id")
	if actorID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	filter := user.ListFilter{
		Search: input.Filter.Search,
	}
	for _, r := range input.Filter.Roles {
		vr, _ := valueobject.ParseUserRole(r)
		filter.Roles = append(filter.Roles, vr)
	}

	for _, s := range input.Filter.Statuses {
		vs, _ := valueobject.ParseUserStatus(s)
		filter.Statuses = append(filter.Statuses, vs)
	}
	output, total, err := h.usecase.GetUserslist(c, user.GetListInput{
		ActorID: actorID,
		Filter:  filter,
		Offset:  input.Offset(),
		Limit:   input.Limit,
	})
	if err != nil {
		response.Internal(c)
		return
	}

	meta := helper.NewPaginationMeta(total, input.PaginationInput)
	response.OK(c, gin.H{"data": newUserResponses(output), "meta": meta})
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
		response.DomainError(c, err)
		return
	}
	response.OK(c, "Deleted")
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")
	var request updateUserRequest
	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&request)
	if err != nil || errId != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	if err := h.usecase.UpdateUser(c, user.UpdateInput{
		UserID:   targetUserID,
		Email:    request.Email,
		Password: request.Password,
		Status:   request.Status,
		Role:     request.Role,
	}); err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "user updated successfully")
}

func (h *UserHandler) ChangeEmail(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	var request updateUserEmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	if err := h.usecase.ChangeEmail(c, user.UpdateEmailInput{
		UserID: userID,
		Email:  request.Email,
	}); err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "user email updated successfully")
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	var request updateUserPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	if err := h.usecase.ChangePassword(c, user.UpdatePassInput{
		UserID:      userID,
		OldPassword: request.CurrentPassword,
		NewPassword: request.NewPassword,
	}); err != nil {
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

	response.OK(c, "password updated successfully")
}

func (h *UserHandler) ChangeRole(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")
	var request updateUserRoleRequest

	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&request)
	if err != nil || errId != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	if err := h.usecase.ChangeRole(c, user.UpdateRoleInput{
		UserID: targetUserID,
		Role:   request.Role,
	}); err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "role updated successfully")
}

func (h *UserHandler) ChangeStatus(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")
	var request updateUserStatusRequest

	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&request)
	if err != nil || errId != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	actorID := c.GetString("user_id")
	if actorID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	if err := h.usecase.ChangeStatus(c, user.UpdateStatusInput{
		UserID:  targetUserID,
		ActorID: actorID,
		Status:  request.Status,
	}); err != nil {
		response.DomainError(c, err)
		return
	}

	response.OK(c, "status updated successfully")
}
