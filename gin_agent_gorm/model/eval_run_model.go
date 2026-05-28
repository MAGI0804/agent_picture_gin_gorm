package model

import "gorm.io/gorm"

// EvalRun stores one evaluation execution result for a prompt version.
type EvalRun struct {
	BaseModel
	EvalCaseID      uint    `gorm:"column:eval_case_id;index;not null" json:"eval_case_id"`
	PromptVersionID uint    `gorm:"column:prompt_version_id;index;not null;default:0" json:"prompt_version_id"`
	AgentName       string  `gorm:"column:agent_name;size:128;index;not null" json:"agent_name"`
	Status          string  `gorm:"column:status;size:32;index;not null;default:pending" json:"status"`
	Score           float64 `gorm:"column:score;type:decimal(8,4);not null;default:0" json:"score"`
	MetricsJSON     string  `gorm:"column:metrics_json;type:text" json:"metrics_json"`
	ErrorMessage    string  `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt       int     `gorm:"column:started_at;index;not null;default:0" json:"started_at"`
	CompletedAt     int     `gorm:"column:completed_at;index;not null;default:0" json:"completed_at"`
	CommonTimestampsField
}

func (EvalRun) TableName() string {
	return "eval_runs"
}

func (m *EvalRun) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

func (m *EvalRun) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
