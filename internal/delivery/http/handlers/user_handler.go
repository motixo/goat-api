package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
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
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}
	input, err := request.toInput()
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	output, err := h.usecase.CreateUser(c, input)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
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
			response.Unauthorized(c, response.DetailAuthenticationContextMissing)
			return
		}
	}
	output, err := h.usecase.GetUser(c, targetUserID)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	response.OK(c, newUserResponse(output))
}

func (h *UserHandler) GetUserList(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input listUsersQuery
	if err := c.ShouldBindQuery(&input); err != nil {
		response.BadRequest(c, response.DetailInvalidPaginationParams)
		return
	}
	input.PaginationInput.Validate()

	actorID := c.GetString("user_id")
	if actorID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	output, total, err := h.usecase.GetUserslist(c, input.toInput(actorID))
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
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
		response.WriteProblem(c, response.MapError(err))
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
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}
	input, err := request.toInput(targetUserID)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	if err := h.usecase.UpdateUser(c, input); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, "user updated successfully")
}

func (h *UserHandler) ChangeEmail(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	var request updateUserEmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	if err := h.usecase.ChangeEmail(c, user.UpdateEmailInput{
		UserID: userID,
		Email:  request.Email,
	}); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, "user email updated successfully")
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	helper.LogRequest(h.logger, c)

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	var request updateUserPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	if err := h.usecase.ChangePassword(c, user.UpdatePassInput{
		UserID:      userID,
		OldPassword: request.CurrentPassword,
		NewPassword: request.NewPassword,
	}); err != nil {
		response.WriteProblem(c, response.MapError(err))
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
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}
	input, err := request.toInput(targetUserID)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	if err := h.usecase.ChangeRole(c, input); err != nil {
		response.WriteProblem(c, response.MapError(err))
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
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	actorID := c.GetString("user_id")
	input, err := request.toInput(targetUserID, actorID)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP())
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}
	if actorID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	if err := h.usecase.ChangeStatus(c, input); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, "status updated successfully")
}
