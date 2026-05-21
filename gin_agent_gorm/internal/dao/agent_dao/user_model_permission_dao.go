package agent_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserModelPermissionDAO 处理用户模型权限的数据访问。
type UserModelPermissionDAO struct{}

// NewUserModelPermissionDAO 创建 UserModelPermissionDAO 实例。
func NewUserModelPermissionDAO() *UserModelPermissionDAO {
	return &UserModelPermissionDAO{}
}

// GetUserPermissions 获取用户对所有模型的权限。
func (dao *UserModelPermissionDAO) GetUserPermissions(userID uint) ([]model.UserModelPermission, error) {
	var permissions []model.UserModelPermission
	err := database.DB.Where("user_id = ?", userID).Find(&permissions).Error
	return permissions, err
}

// GetUserModelPermission 获取用户对特定模型的权限。
func (dao *UserModelPermissionDAO) GetUserModelPermission(userID, modelConfigID uint) (model.UserModelPermission, error) {
	var permission model.UserModelPermission
	err := database.DB.Where("user_id = ? AND model_config_id = ?", userID, modelConfigID).First(&permission).Error
	return permission, err
}

// CheckUserModelPermission 检查用户是否有权限使用指定模型。
// 如果没有明确的权限记录，默认返回 true（允许使用）。
func (dao *UserModelPermissionDAO) CheckUserModelPermission(userID, modelConfigID uint) (bool, error) {
	var permission model.UserModelPermission
	err := database.DB.Where("user_id = ? AND model_config_id = ?", userID, modelConfigID).First(&permission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return true, nil
		}
		return false, err
	}
	return permission.CanUse, nil
}

// BatchSetUserPermissions 批量设置用户对多个模型的权限。
func (dao *UserModelPermissionDAO) BatchSetUserPermissions(userID uint, permissions []model.UserModelPermission) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, permission := range permissions {
		permission.UserID = userID
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "model_config_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"can_use",
				"updated_at",
			}),
		}).Create(&permission).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// SetUserModelPermission 设置用户对单个模型的权限。
func (dao *UserModelPermissionDAO) SetUserModelPermission(userID, modelConfigID uint, canUse bool) error {
	permission := model.UserModelPermission{
		UserID:        userID,
		ModelConfigID: modelConfigID,
		CanUse:        canUse,
	}
	return database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "model_config_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"can_use",
			"updated_at",
		}),
	}).Create(&permission).Error
}

// DeleteUserModelPermission 删除用户对特定模型的权限记录。
func (dao *UserModelPermissionDAO) DeleteUserModelPermission(userID, modelConfigID uint) error {
	return database.DB.Where("user_id = ? AND model_config_id = ?", userID, modelConfigID).Delete(&model.UserModelPermission{}).Error
}