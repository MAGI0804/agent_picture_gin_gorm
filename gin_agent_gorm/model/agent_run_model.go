package model

import "gorm.io/gorm"

// AgentRun 表示一次用户输入触发的多 Agent 总任务。
type AgentRun struct {
	BaseModel
	ConversationID   uint   `gorm:"column:conversation_id;index;not null" json:"conversation_id"`       // 所属会话 ID。
	UserID           uint   `gorm:"column:user_id;index;not null" json:"user_id"`                       // 所属用户 ID。
	TriggerMessageID uint   `gorm:"column:trigger_message_id;index;not null" json:"trigger_message_id"` // 触发任务的用户消息 ID。
	Status           string `gorm:"column:status;size:32;not null;default:running" json:"status"`       // 任务状态。
	Intent           string `gorm:"column:intent;size:64" json:"intent"`                                // 识别出的任务意图。
	ErrorMessage     string `gorm:"column:error_message;type:text" json:"error_message"`                // 失败原因。
	CommonTimestampsField
}

// TableName 返回 Agent Run 表名。
func (AgentRun) TableName() string {
	return "agent_runs"
}

// BeforeCreate 在创建 Agent Run 前写入时间戳。
func (m *AgentRun) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新 Agent Run 前刷新更新时间。
func (m *AgentRun) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
