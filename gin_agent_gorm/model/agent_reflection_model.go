package model

import "gorm.io/gorm"

// AgentReflection 保存 Agent 对失败轨迹、低分产物和用户反馈的结构化反思。
type AgentReflection struct {
	BaseModel
	AgentRunID       uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`
	AgentName        string `gorm:"column:agent_name;size:128;not null" json:"agent_name"`
	FailureType      string `gorm:"column:failure_type;size:128" json:"failure_type"`
	Reflection       string `gorm:"column:reflection;type:text;not null" json:"reflection"`
	ActionItem       string `gorm:"column:action_item;type:text;not null" json:"action_item"`
	PromotedToMemory bool   `gorm:"column:promoted_to_memory;not null;default:false" json:"promoted_to_memory"`
	CommonTimestampsField
}

// TableName 返回 Agent 反思表名。
func (AgentReflection) TableName() string {
	return "agent_reflections"
}

// BeforeCreate 创建反思记录前写入时间戳。
func (m *AgentReflection) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新反思记录前刷新时间戳。
func (m *AgentReflection) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
