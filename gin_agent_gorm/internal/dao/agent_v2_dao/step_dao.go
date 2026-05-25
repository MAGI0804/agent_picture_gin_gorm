package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateStep 创建 workflow 节点对应的 step 记录。
func (dao *AgentV2DAO) CreateStep(step *model.AgentStep) error {
	return database.DB.Create(step).Error
}

// UpdateStep 更新 step 的执行结果、耗时、错误或结构化输出。
func (dao *AgentV2DAO) UpdateStep(stepID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentStep{}).Where("id = ?", stepID).Updates(attrs).Error
}

// ListSteps 按 run 读取 step timeline，并通过 agent_runs 校验用户归属。
func (dao *AgentV2DAO) ListSteps(userID uint, runID uint) ([]model.AgentStep, error) {
	var steps []model.AgentStep
	err := database.DB.Model(&model.AgentStep{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_steps.agent_run_id").
		Where("agent_runs.user_id = ? AND agent_steps.agent_run_id = ?", userID, runID).
		Order("agent_steps.id asc").
		Find(&steps).Error
	return steps, err
}
