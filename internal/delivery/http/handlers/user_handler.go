package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserHandler struct {
	usecase user.UseCase
	logger  service.Logger
}

func NewUserHandler(usecase user.UseCase, logger service.Logger) *UserHandler {
	return &UserHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input user.CreateInput

	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.CreateUser(c, input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.Created(c, output)
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
	var input helper.UserListInput
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

	filter := entity.UserFilter{
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

func (h *UserHandler) UpdateUser(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")
	var input user.UpdateInput
	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&input)
	if err != nil || errId != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input.UserID = targetUserID
	if err := h.usecase.UpdateUser(c, input); err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "user updated successfully")
}

func (h *UserHandler) ChangeEmail(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	var input user.UpdateEmailInput

	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	input.UserID = userID
	if err := h.usecase.ChangeEmail(c, input); err != nil {
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

	var input user.UpdatePassInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input.UserID = userID
	if err := h.usecase.ChangePassword(c, input); err != nil {
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
	var input user.UpdateRoleInput

	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&input)
	if err != nil || errId != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input.UserID = targetUserID
	if err := h.usecase.ChangeRole(c, input); err != nil {
		response.Internal(c)
		return
	}

	response.OK(c, "role updated successfully")
}

func (h *UserHandler) ChangeStatus(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	targetUserID := c.Param("id")
	var input user.UpdateStatusInput

	_, errId := uuid.Parse(targetUserID)
	err := c.ShouldBindJSON(&input)
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

	input.ActorID = actorID
	input.UserID = targetUserID
	if err := h.usecase.ChangeStatus(c, input); err != nil {
		response.DomainError(c, err)
		return
	}

	response.OK(c, "status updated successfully")
}
