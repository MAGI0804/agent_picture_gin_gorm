package model

import "gorm.io/gorm"

// FollowUpQuestion 表示 assistant 在上一轮输出后生成的补充问题。
type FollowUpQuestion struct {
	BaseModel
	ConversationID uint   `gorm:"column:conversation_id;index;not null" json:"conversation_id"` // 所属会话 ID。
	MessageID      uint   `gorm:"column:message_id;index;not null" json:"message_id"`           // 产生该问题的 assistant 消息 ID。
	UserID         uint   `gorm:"column:user_id;index;not null" json:"user_id"`                 // 所属用户 ID。
	Question       string `gorm:"column:question;type:text;not null" json:"question"`           // 问题内容。
	Answer         string `gorm:"column:answer;type:text" json:"answer"`                        // 用户回答内容。
	Status         string `gorm:"column:status;size:32;not null;default:pending" json:"status"` // 问题状态：pending、answered。
	CommonTimestampsField
}

// TableName 返回补充问题表名。
func (FollowUpQuestion) TableName() string {
	return "follow_up_questions"
}

// BeforeCreate 在创建补充问题前写入时间戳。
func (m *FollowUpQuestion) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新补充问题前刷新更新时间。
func (m *FollowUpQuestion) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
