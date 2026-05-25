package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateToolInvocation 写入一条提供商/工具调用追踪记录。
func (dao *AgentV2DAO) CreateToolInvocation(invocation *model.ToolInvocation) error {
	return database.DB.Create(invocation).Error
}

// UpdateToolInvocation 更新调用状态、输出、成本、延迟或错误信息。
func (dao *AgentV2DAO) UpdateToolInvocation(invocationID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.ToolInvocation{}).Where("id = ?", invocationID).Updates(attrs).Error
}

// ListToolInvocationsByRun 用户权限验证后列出某次运行的工具调用。
func (dao *AgentV2DAO) ListToolInvocationsByRun(userID uint, runID uint) ([]model.ToolInvocation, error) {
	var invocations []model.ToolInvocation
	err := database.DB.Model(&model.ToolInvocation{}).
		Joins("JOIN agent_runs ON agent_runs.id = tool_invocations.agent_run_id").
		Where("agent_runs.user_id = ? AND tool_invocations.agent_run_id = ?", userID, runID).
		Order("tool_invocations.id asc").
		Find(&invocations).Error
	return invocations, err
}
