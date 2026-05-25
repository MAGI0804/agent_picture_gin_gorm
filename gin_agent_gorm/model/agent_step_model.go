package model

import "gorm.io/gorm"

// AgentStep represents one child step in an Agent Run.
type AgentStep struct {
	BaseModel
	AgentRunID       uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`
	Name             string `gorm:"column:name;size:64;not null" json:"name"`
	Status           string `gorm:"column:status;size:32;not null;default:pending" json:"status"`
	Input            string `gorm:"column:input;type:text" json:"input"`
	Output           string `gorm:"column:output;type:text" json:"output"`
	ErrorMessage     string `gorm:"column:error_message;type:text" json:"error_message"`
	ThinkContent     string `gorm:"column:think_content;type:text;comment:Agent thinking content" json:"think_content"`
	ReasoningContent string `gorm:"column:reasoning_content;type:text;comment:Raw model reasoning content" json:"reasoning_content"`
	StepKey          string `gorm:"column:step_key;size:128" json:"step_key"`
	Attempt          int    `gorm:"column:attempt;not null;default:1" json:"attempt"`
	ProviderName     string `gorm:"column:provider_name;size:128" json:"provider_name"`
	ModelName        string `gorm:"column:model_name;size:128" json:"model_name"`
	DurationMS       int64  `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	CostJSON         string `gorm:"column:cost_json;type:text" json:"cost_json"`
	InputJSON        string `gorm:"column:input_json;type:text" json:"input_json"`
	OutputJSON       string `gorm:"column:output_json;type:text" json:"output_json"`
	InputHash        string `gorm:"column:input_hash;size:128" json:"input_hash"`
	OutputHash       string `gorm:"column:output_hash;size:128" json:"output_hash"`
	CommonTimestampsField
}

// TableName returns the Agent Step table name.
func (AgentStep) TableName() string {
	return "agent_steps"
}

// BeforeCreate writes timestamps before inserting an AgentStep.
func (m *AgentStep) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate refreshes timestamps before updating an AgentStep.
func (m *AgentStep) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
