package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

// ListPermittedModelConfigs returns global model configs the user can use.
func (dao *AgentV2DAO) ListPermittedModelConfigs(
	userID uint,
	isTextModel bool,
	isImageModel bool,
) ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	query := database.DB.Model(&model.ModelConfig{})
	if isTextModel {
		query = query.Where("model_configs.is_text_model = ?", true)
	}
	if isImageModel {
		query = query.Where("model_configs.is_image_model = ?", true)
	}

	hasPermissions, err := dao.userHasModelPermissions(userID)
	if err != nil {
		return nil, err
	}
	if hasPermissions {
		query = query.
			Joins("LEFT JOIN user_model_permissions ON user_model_permissions.model_config_id = model_configs.id AND user_model_permissions.user_id = ?", userID).
			Where("user_model_permissions.id IS NULL OR user_model_permissions.can_use = ?", true)
	}

	err = query.Order("model_configs.id desc").Find(&configs).Error
	return configs, err
}

// FindPermittedModelConfig returns one model config if permissions allow it.
func (dao *AgentV2DAO) FindPermittedModelConfig(userID uint, modelConfigID uint) (model.ModelConfig, error) {
	var config model.ModelConfig
	if err := database.DB.Where("id = ?", modelConfigID).First(&config).Error; err != nil {
		return config, err
	}
	hasPermissions, err := dao.userHasModelPermissions(userID)
	if err != nil {
		return config, err
	}
	if !hasPermissions {
		return config, nil
	}
	var permission model.UserModelPermission
	err = database.DB.
		Where("user_id = ? AND model_config_id = ?", userID, modelConfigID).
		First(&permission).Error
	if err == nil {
		if permission.CanUse {
			return config, nil
		}
		return model.ModelConfig{}, gorm.ErrRecordNotFound
	}
	if err == gorm.ErrRecordNotFound {
		return config, nil
	}
	return model.ModelConfig{}, err
}

func (dao *AgentV2DAO) userHasModelPermissions(userID uint) (bool, error) {
	err := database.DB.
		Where("user_id = ?", userID).
		First(&model.UserModelPermission{}).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}
