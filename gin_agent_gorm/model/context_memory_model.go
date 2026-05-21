package model

import "gorm.io/gorm"

// ContextMemory 表示会话长期上下文、偏好或历史摘要。
type ContextMemory struct {
	BaseModel
	ConversationID uint   `gorm:"column:conversation_id;index;not null" json:"conversation_id"` // 所属会话 ID。
	UserID         uint   `gorm:"column:user_id;index;not null" json:"user_id"`                 // 所属用户 ID。
	Kind           string `gorm:"column:kind;size:64;not null" json:"kind"`                     // 记忆类型。
	Content        string `gorm:"column:content;type:text;not null" json:"content"`             // 记忆内容。
	Score          int    `gorm:"column:score;not null;default:0" json:"score"`                 // 检索排序分数。
	CommonTimestampsField
}

// TableName 返回上下文记忆表名。
func (ContextMemory) TableName() string {
	return "context_memories"
}

// BeforeCreate 在创建上下文记忆前写入时间戳。
func (m *ContextMemory) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新上下文记忆前刷新更新时间。
func (m *ContextMemory) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
