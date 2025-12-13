package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type PermissionHandler struct {
	usecase permission.UseCase
	logger  service.Logger
}

func NewPermissionHandler(usecase permission.UseCase, logger service.Logger) *PermissionHandler {
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
	output, total, err := h.usecase.GetPermissions(c, input.Offset(), input.Limit)
	if err != nil {
		response.Internal(c)
		return
	}
	meta := helper.NewPaginationMeta(total, input)
	response.OK(c, gin.H{"data": output, "meta": meta})
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
		response.Internal(c)
		return
	}
	response.OK(c, output)
}

func (h *PermissionHandler) CreatePermissin(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input permission.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}
	output, err := h.usecase.Create(c, input)
	if err != nil {
		response.Internal(c)
		return
	}
	response.Created(c, output)
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
		response.Internal(c)
		return
	}
	response.OK(c, "Deleted")
}
