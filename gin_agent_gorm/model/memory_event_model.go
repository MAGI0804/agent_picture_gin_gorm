package model

import "gorm.io/gorm"

// MemoryEvent 记录 V2 记忆的写入、更新、删除和使用更新事件。
type MemoryEvent struct {
	BaseModel
	MemoryID       uint   `gorm:"column:memory_id;index;not null;default:0" json:"memory_id"`
	UserID         uint   `gorm:"column:user_id;index;not null" json:"user_id"`
	ConversationID uint   `gorm:"column:conversation_id;index;not null;default:0" json:"conversation_id"`
	AgentRunID     uint   `gorm:"column:agent_run_id;index;not null;default:0" json:"agent_run_id"`
	EventType      string `gorm:"column:event_type;size:64;index;not null" json:"event_type"`
	SourceType     string `gorm:"column:source_type;size:64;index" json:"source_type"`
	SourceID       uint   `gorm:"column:source_id;index;not null;default:0" json:"source_id"`
	BeforeJSON     string `gorm:"column:before_json;type:text" json:"before_json"`
	AfterJSON      string `gorm:"column:after_json;type:text" json:"after_json"`
	Reason         string `gorm:"column:reason;type:text" json:"reason"`
	CommonTimestampsField
}

// TableName 返回记忆事件表名。
func (MemoryEvent) TableName() string {
	return "memory_events"
}

// BeforeCreate 写入记忆事件前写入时间戳。
func (m *MemoryEvent) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新记忆事件前刷新时间戳。
func (m *MemoryEvent) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
