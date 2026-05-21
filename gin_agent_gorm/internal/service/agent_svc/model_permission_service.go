package agent_svc

import (
	"github.com/pkg/errors"

	"gin-biz-web-api/internal/dao/agent_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// ModelPermissionService 处理用户模型权限业务。
type ModelPermissionService struct {
	permissionDAO *agent_dao.UserModelPermissionDAO
}

// NewModelPermissionService 创建 ModelPermissionService 实例。
func NewModelPermissionService() *ModelPermissionService {
	return &ModelPermissionService{
		permissionDAO: agent_dao.NewUserModelPermissionDAO(),
	}
}

// GetUserModelPermissions 获取用户对所有模型的权限列表。
func (svc *ModelPermissionService) GetUserModelPermissions(userID uint) (map[string]interface{}, error) {
	permissions, err := svc.permissionDAO.GetUserPermissions(userID)
	if err != nil {
		return nil, err
	}

	var textModels []model.ModelConfig
	err = database.DB.Where("is_text_model = ?", true).Order("id desc").Find(&textModels).Error
	if err != nil {
		return nil, err
	}

	var imageModels []model.ModelConfig
	err = database.DB.Where("is_image_model = ?", true).Order("id desc").Find(&imageModels).Error
	if err != nil {
		return nil, err
	}

	permissionMap := make(map[uint]bool)
	for _, p := range permissions {
		permissionMap[p.ModelConfigID] = p.CanUse
	}

	resultTextModels := make([]map[string]interface{}, 0, len(textModels))
	for _, model := range textModels {
		canUse, ok := permissionMap[model.ID]
		if !ok {
			canUse = true
		}
		resultTextModels = append(resultTextModels, map[string]interface{}{
			"id":            model.ID,
			"model_name":    model.ModelName,
			"request_url":   model.RequestURL,
			"can_use":       canUse,
			"is_text_model": true,
		})
	}

	resultImageModels := make([]map[string]interface{}, 0, len(imageModels))
	for _, model := range imageModels {
		canUse, ok := permissionMap[model.ID]
		if !ok {
			canUse = true
		}
		resultImageModels = append(resultImageModels, map[string]interface{}{
			"id":             model.ID,
			"model_name":     model.ModelName,
			"request_url":    model.RequestURL,
			"can_use":        canUse,
			"is_image_model": true,
		})
	}

	return map[string]interface{}{
		"text_models":  resultTextModels,
		"image_models": resultImageModels,
	}, nil
}

// SetUserModelPermission 设置用户对指定模型的权限。
func (svc *ModelPermissionService) SetUserModelPermission(userID, modelConfigID uint, canUse bool) error {
	var modelConfig model.ModelConfig
	err := database.DB.Where("id = ?", modelConfigID).First(&modelConfig).Error
	if err != nil {
		return errors.Wrap(err, "model config not found")
	}
	if modelConfig.ID == 0 {
		return errors.New("model config not found")
	}
	return svc.permissionDAO.SetUserModelPermission(userID, modelConfigID, canUse)
}

// BatchSetUserModelPermissions 批量设置用户对多个模型的权限。
func (svc *ModelPermissionService) BatchSetUserModelPermissions(userID uint, permissions []struct {
	ModelConfigID uint `json:"model_config_id"`
	CanUse        bool `json:"can_use"`
}) error {
	permissionModels := make([]model.UserModelPermission, 0, len(permissions))
	for _, p := range permissions {
		var modelConfig model.ModelConfig
		err := database.DB.Where("id = ?", p.ModelConfigID).First(&modelConfig).Error
		if err != nil || modelConfig.ID == 0 {
			return errors.Errorf("model config %d not found", p.ModelConfigID)
		}
		permissionModels = append(permissionModels, model.UserModelPermission{
			ModelConfigID: p.ModelConfigID,
			CanUse:        p.CanUse,
		})
	}
	return svc.permissionDAO.BatchSetUserPermissions(userID, permissionModels)
}

// CheckUserModelPermission 检查用户是否有权限使用指定模型。
func (svc *ModelPermissionService) CheckUserModelPermission(userID, modelConfigID uint) (bool, error) {
	return svc.permissionDAO.CheckUserModelPermission(userID, modelConfigID)
}

// GetUserAvailableModels 获取用户可用的模型列表。
func (svc *ModelPermissionService) GetUserAvailableModels(userID uint) (map[string]interface{}, error) {
	result, err := svc.GetUserModelPermissions(userID)
	if err != nil {
		return nil, err
	}

	textModels := result["text_models"].([]map[string]interface{})
	imageModels := result["image_models"].([]map[string]interface{})

	availableTextModels := make([]map[string]interface{}, 0)
	for _, m := range textModels {
		if m["can_use"].(bool) {
			availableTextModels = append(availableTextModels, m)
		}
	}

	availableImageModels := make([]map[string]interface{}, 0)
	for _, m := range imageModels {
		if m["can_use"].(bool) {
			availableImageModels = append(availableImageModels, m)
		}
	}

	return map[string]interface{}{
		"text_models":  availableTextModels,
		"image_models": availableImageModels,
	}, nil
}