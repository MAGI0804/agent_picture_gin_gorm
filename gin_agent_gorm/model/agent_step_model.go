package model

import "gorm.io/gorm"

// AgentStep 表示 Agent Run 中的一个子步骤。
type AgentStep struct {
	BaseModel
	AgentRunID       uint   `gorm:"column:agent_run_id;index;not null" json:"agent_run_id"`                        // 所属 Agent Run ID。
	Name             string `gorm:"column:name;size:64;not null" json:"name"`                                      // 子 Agent 或步骤名称。
	Status           string `gorm:"column:status;size:32;not null;default:pending" json:"status"`                  // 步骤状态。
	Input            string `gorm:"column:input;type:text" json:"input"`                                           // 步骤输入。
	Output           string `gorm:"column:output;type:text" json:"output"`                                         // 步骤输出。
	ErrorMessage     string `gorm:"column:error_message;type:text" json:"error_message"`                           // 步骤失败原因。
	ThinkContent     string `gorm:"column:think_content;type:text;comment:Agent思考过程" json:"think_content"`         // Agent 自主规划思考内容。
	ReasoningContent string `gorm:"column:reasoning_content;type:text;comment:大模型原生思考内容" json:"reasoning_content"` // 大模型返回的原生思考内容。
	CommonTimestampsField
}

// TableName 返回 Agent Step 表名。
func (AgentStep) TableName() string {
	return "agent_steps"
}

// BeforeCreate 在创建 Agent Step 前写入时间戳。
func (m *AgentStep) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新 Agent Step 前刷新更新时间。
func (m *AgentStep) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
