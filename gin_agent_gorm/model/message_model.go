package model

import "gorm.io/gorm"

// Message 表示会话中的一条消息。
type Message struct {
	BaseModel
	ConversationID uint   `gorm:"column:conversation_id;index;not null" json:"conversation_id"` // 所属会话 ID。
	UserID         uint   `gorm:"column:user_id;index;not null" json:"user_id"`                 // 所属用户 ID。
	Role           string `gorm:"column:role;size:32;not null" json:"role"`                     // 消息角色：user、assistant、system。
	InputType      string `gorm:"column:input_type;size:64" json:"input_type"`                  // 输入类型：normal、answer_to_questions 等。
	Content        string `gorm:"column:content;type:text;not null" json:"content"`             // 消息正文。
	AgentRunID     uint   `gorm:"column:agent_run_id;index" json:"agent_run_id"`                // 关联的 Agent Run ID。
	CommonTimestampsField
}

// TableName 返回消息表名。
func (Message) TableName() string {
	return "messages"
}

// BeforeCreate 在创建消息前写入时间戳。
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新消息前刷新更新时间。
func (m *Message) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
