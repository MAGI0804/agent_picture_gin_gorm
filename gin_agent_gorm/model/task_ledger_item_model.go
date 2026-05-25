package model

import "gorm.io/gorm"

// TaskLedgerItem 记录运行级别任务台账中的一个工作流任务。
type TaskLedgerItem struct {
	BaseModel
	AgentRunID     uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`
	TaskKey        string `gorm:"column:task_key;size:128;index;not null" json:"task_key"`
	OwnerAgent     string `gorm:"column:owner_agent;size:128;index;not null" json:"owner_agent"`
	Status         string `gorm:"column:status;size:32;index;not null;default:pending" json:"status"`
	DependsOnJSON  string `gorm:"column:depends_on_json;type:text" json:"depends_on_json"`
	InputRefsJSON  string `gorm:"column:input_refs_json;type:text" json:"input_refs_json"`
	OutputRefsJSON string `gorm:"column:output_refs_json;type:text" json:"output_refs_json"`
	RetryCount     int    `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	ErrorMessage   string `gorm:"column:error_message;type:text" json:"error_message"`
	CommonTimestampsField
}

// TableName 返回任务台账表名。
func (TaskLedgerItem) TableName() string {
	return "task_ledger_items"
}

// BeforeCreate 写入任务台账项前写入时间戳。
func (m *TaskLedgerItem) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 更新任务台账项前刷新时间戳。
func (m *TaskLedgerItem) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
