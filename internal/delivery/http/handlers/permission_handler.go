package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/permission"
)

type PermissionHandler struct {
	usecase permission.UseCase
	logger  pkg.Logger
}

func NewPermissionHandler(usecase permission.UseCase, logger pkg.Logger) *PermissionHandler {
	return &PermissionHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *PermissionHandler) GetPermissions(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input helper.PaginationInput
	if err := c.ShouldBindQuery(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}
	input.Validate()
	output, total, err := h.usecase.GetPermissions(c, input.Offset(), input.Limit)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	meta := helper.NewPaginationMeta(total, input)
	response.OK(c, gin.H{"data": newPermissionResponses(output), "meta": meta})
}

func (h *PermissionHandler) GetPermissionsByRole(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	roleInput := c.Param("role")
	if roleInput == "" {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	role, err := valueobject.ParseUserRole(roleInput)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, err.Error())
		return
	}
	output, err := h.usecase.GetPermissionsByRole(c, role)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	response.OK(c, newPermissionResponses(output))
}

func (h *PermissionHandler) CreatePermissin(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request createPermissionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	role, err := valueobject.ParseUserRole(request.Role)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	var action valueobject.Permission
	if request.Action != "" {
		action, err = valueobject.ParsePermission(request.Action)
		if err != nil {
			h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
			response.BadRequest(c, "Invalid request payload")
			return
		}
	}

	output, err := h.usecase.Create(c, permission.CreateInput{
		Role:   role,
		Action: action,
	})
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	response.Created(c, newPermissionResponse(output))
}

func (h *PermissionHandler) DeletePermissin(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	permissionID := c.Param("id")
	if permissionID == "" {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}
	if err := h.usecase.Delete(c, permissionID); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	response.OK(c, "Deleted")
}
