package agent_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

// ModelPermissionController 处理模型权限相关 HTTP 请求。
type ModelPermissionController struct {
}

// GetUserModelPermissions 获取当前用户对所有模型的权限列表。
// GET /api/permissions/model-permissions
func (ctrl *ModelPermissionController) GetUserModelPermissions(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	result, err := agent_svc.NewModelPermissionService().GetUserModelPermissions(userID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(result)
}

// GetUserAvailableModels 获取当前用户可用的模型列表（过滤掉无权限的模型）。
// GET /api/permissions/available-models
func (ctrl *ModelPermissionController) GetUserAvailableModels(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	result, err := agent_svc.NewModelPermissionService().GetUserAvailableModels(userID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(result)
}

// SetUserModelPermission 设置指定用户对指定模型的权限（管理员接口）。
// POST /api/permissions/user-model-permission
func (ctrl *ModelPermissionController) SetUserModelPermission(c *gin.Context) {
	var request struct {
		UserID        uint `json:"user_id" binding:"required"`
		ModelConfigID uint `json:"model_config_id" binding:"required"`
		CanUse        bool `json:"can_use"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}

	if request.UserID == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "用户 ID 不能为空")
		return
	}
	if request.ModelConfigID == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "模型配置 ID 不能为空")
		return
	}

	err := agent_svc.NewModelPermissionService().SetUserModelPermission(request.UserID, request.ModelConfigID, request.CanUse)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}

	responses.New(c).ToResponse(gin.H{"message": "权限设置成功"})
}

// BatchSetUserModelPermissions 批量设置指定用户对多个模型的权限（管理员接口）。
// POST /api/permissions/user-model-permissions
func (ctrl *ModelPermissionController) BatchSetUserModelPermissions(c *gin.Context) {
	var request struct {
		UserID       uint `json:"user_id" binding:"required"`
		Permissions []struct {
			ModelConfigID uint `json:"model_config_id" binding:"required"`
			CanUse        bool `json:"can_use"`
		} `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}

	if request.UserID == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "用户 ID 不能为空")
		return
	}
	if len(request.Permissions) == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "权限列表不能为空")
		return
	}

	permissions := make([]struct {
		ModelConfigID uint `json:"model_config_id"`
		CanUse        bool `json:"can_use"`
	}, 0, len(request.Permissions))
	for _, p := range request.Permissions {
		permissions = append(permissions, struct {
			ModelConfigID uint `json:"model_config_id"`
			CanUse        bool `json:"can_use"`
		}{p.ModelConfigID, p.CanUse})
	}
	err := agent_svc.NewModelPermissionService().BatchSetUserModelPermissions(request.UserID, permissions)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}

	responses.New(c).ToResponse(gin.H{"message": "批量权限设置成功"})
}

// CheckModelPermission 检查当前用户是否有权限使用指定模型。
// GET /api/permissions/check/model/:model_id
func (ctrl *ModelPermissionController) CheckModelPermission(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	modelIDStr := c.Param("model_id")

	modelConfigID, err := strconv.ParseUint(modelIDStr, 10, 64)
	if err != nil || modelConfigID == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "模型配置 ID 参数错误")
		return
	}

	canUse, err := agent_svc.NewModelPermissionService().CheckUserModelPermission(userID, uint(modelConfigID))
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{
		"model_config_id": modelConfigID,
		"can_use":         canUse,
	})
}
