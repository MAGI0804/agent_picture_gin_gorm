package model

import "gorm.io/gorm"

// ToolInvocation records every provider/tool call made through the V2 Tool Registry.
type ToolInvocation struct {
	BaseModel
	AgentRunID   uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`
	AgentStepID  uint   `gorm:"column:agent_step_id;index;not null;default:0" json:"agent_step_id"`
	UserID       uint   `gorm:"column:user_id;index;not null" json:"user_id"`
	ToolName     string `gorm:"column:tool_name;size:128;index;not null" json:"tool_name"`
	ToolKind     string `gorm:"column:tool_kind;size:64;index;not null" json:"tool_kind"`
	ProviderName string `gorm:"column:provider_name;size:128;index" json:"provider_name"`
	ModelName    string `gorm:"column:model_name;size:128" json:"model_name"`
	Status       string `gorm:"column:status;size:32;index;not null;default:pending" json:"status"`
	InputJSON    string `gorm:"column:input_json;type:text" json:"input_json"`
	OutputJSON   string `gorm:"column:output_json;type:text" json:"output_json"`
	CostJSON     string `gorm:"column:cost_json;type:text" json:"cost_json"`
	DurationMS   int64  `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	ErrorCode    string `gorm:"column:error_code;size:128" json:"error_code"`
	ErrorMessage string `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt    int    `gorm:"column:started_at;index;not null;default:0" json:"started_at"`
	CompletedAt  int    `gorm:"column:completed_at;index;not null;default:0" json:"completed_at"`
	CommonTimestampsField
}

// TableName returns the tool invocation table name.
func (ToolInvocation) TableName() string {
	return "tool_invocations"
}

// BeforeCreate writes timestamps before inserting a tool invocation.
func (m *ToolInvocation) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate refreshes timestamps before updating a tool invocation.
func (m *ToolInvocation) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
