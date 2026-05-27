package model

import "gorm.io/gorm"

// AgentRun represents one workflow triggered by a user message.
type AgentRun struct {
	BaseModel
	ConversationID       uint    `gorm:"column:conversation_id;index;not null" json:"conversation_id"`
	UserID               uint    `gorm:"column:user_id;index;not null;uniqueIndex:idx_agent_runs_user_idempotency_unique,priority:1" json:"user_id"`
	TriggerMessageID     uint    `gorm:"column:trigger_message_id;index;not null" json:"trigger_message_id"`
	Status               string  `gorm:"column:status;size:32;not null;default:running" json:"status"`
	Intent               string  `gorm:"column:intent;size:64" json:"intent"`
	TaskType             string  `gorm:"column:task_type;size:64" json:"task_type"`
	WorkflowName         string  `gorm:"column:workflow_name;size:128" json:"workflow_name"`
	WorkflowVersion      string  `gorm:"column:workflow_version;size:64" json:"workflow_version"`
	StateJSON            string  `gorm:"column:state_json;type:text" json:"state_json"`
	BudgetJSON           string  `gorm:"column:budget_json;type:text" json:"budget_json"`
	IdempotencyKey       string  `gorm:"column:idempotency_key;size:128;index" json:"idempotency_key"`
	IdempotencyKeyUnique *string `gorm:"column:idempotency_key_unique;size:191;uniqueIndex:idx_agent_runs_user_idempotency_unique,priority:2" json:"-"`
	LockKey              string  `gorm:"column:lock_key;size:128;index" json:"lock_key"`
	TextModelName        string  `gorm:"column:text_model_name;size:128" json:"text_model_name"`
	ImageModelName       string  `gorm:"column:image_model_name;size:128" json:"image_model_name"`
	IsOptimized          bool    `gorm:"column:is_optimized;not null;default:false" json:"is_optimized"`
	OptimizedPrompt      string  `gorm:"column:optimized_prompt;type:text" json:"optimized_prompt"`
	ErrorMessage         string  `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt            int     `gorm:"column:started_at;not null;default:0" json:"started_at"`
	CompletedAt          int     `gorm:"column:completed_at;not null;default:0" json:"completed_at"`
	CancelledAt          int     `gorm:"column:cancelled_at;not null;default:0" json:"cancelled_at"`
	CommonTimestampsField
}

// TableName returns the agent run table name.
func (AgentRun) TableName() string {
	return "agent_runs"
}

// BeforeCreate writes timestamps before inserting an AgentRun.
func (m *AgentRun) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate refreshes timestamps before updating an AgentRun.
func (m *AgentRun) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
