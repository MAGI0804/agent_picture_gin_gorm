package model

import "gorm.io/gorm"

// UserModelPermission 表示用户对模型的使用权限。
type UserModelPermission struct {
	BaseModel
	UserID         uint `gorm:"column:user_id;index;not null" json:"user_id"`                 // 用户 ID
	ModelConfigID  uint `gorm:"column:model_config_id;index;not null" json:"model_config_id"` // 模型配置 ID
	CanUse         bool `gorm:"column:can_use;not null;default:true" json:"can_use"`          // 是否可以使用该模型
	CommonTimestampsField
}

// TableName 返回用户模型权限表名。
func (UserModelPermission) TableName() string {
	return "user_model_permissions"
}

// BeforeCreate 在创建权限记录前写入时间戳。
func (m *UserModelPermission) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新权限记录前刷新更新时间。
func (m *UserModelPermission) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}