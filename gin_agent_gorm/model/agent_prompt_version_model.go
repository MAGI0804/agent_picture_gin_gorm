package model

import "gorm.io/gorm"

// AgentPromptVersion 保存 Agent 使用的 prompt 模板版本，后续用于灰度、评测和回滚。
type AgentPromptVersion struct {
	BaseModel
	AgentName      string `gorm:"column:agent_name;size:128;not null" json:"agent_name"`
	Version        string `gorm:"column:version;size:64;not null" json:"version"`
	PromptTemplate string `gorm:"column:prompt_template;type:text;not null" json:"prompt_template"`
	Changelog      string `gorm:"column:changelog;type:text" json:"changelog"`
	Status         string `gorm:"column:status;size:32;not null;default:draft" json:"status"`
	Metrics        string `gorm:"column:metrics;type:text" json:"metrics"`
	CommonTimestampsField
}

// TableName 返回 Agent prompt 版本表名。
func (AgentPromptVersion) TableName() string {
	return "agent_prompt_versions"
}

// BeforeCreate 创建 prompt 版本前写入时间戳。
func (m *AgentPromptVersion) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新 prompt 版本前刷新时间戳。
func (m *AgentPromptVersion) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
