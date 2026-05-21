package model

import "gorm.io/gorm"

// Conversation 表示一个用户的 AI Agent 会话。
type Conversation struct {
	BaseModel
	UserID uint   `gorm:"column:user_id;index;not null" json:"user_id"`                // 用户 ID，用于数据隔离。
	Title  string `gorm:"column:title;size:255;not null" json:"title"`                 // 会话标题。
	Status string `gorm:"column:status;size:32;not null;default:active" json:"status"` // 会话状态，默认 active。
	CommonTimestampsField
}

// TableName 返回会话表名。
func (Conversation) TableName() string {
	return "conversations"
}

// BeforeCreate 在创建会话前写入时间戳。
func (m *Conversation) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新会话前刷新更新时间。
func (m *Conversation) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
