package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// AgentV2DAO Agent V2 数据访问对象
type AgentV2DAO struct{}

// NewAgentV2DAO 创建 AgentV2DAO 实例
func NewAgentV2DAO() *AgentV2DAO {
	return &AgentV2DAO{}
}

// FindConversation 根据用户ID和会话ID查找会话
func (dao *AgentV2DAO) FindConversation(userID uint, conversationID uint) (model.Conversation, error) {
	var conversation model.Conversation
	err := database.DB.Where("user_id = ? AND id = ?", userID, conversationID).First(&conversation).Error
	return conversation, err
}

// CreateMessage 创建一条消息
func (dao *AgentV2DAO) CreateMessage(message *model.Message) error {
	return database.DB.Create(message).Error
}

// UpdateMessageAgentRunID 更新消息关联的 Agent 运行ID
func (dao *AgentV2DAO) UpdateMessageAgentRunID(messageID uint, agentRunID uint) error {
	return database.DB.Model(&model.Message{}).Where("id = ?", messageID).Update("agent_run_id", agentRunID).Error
}

// CreateRun 创建一个 Agent 运行记录
func (dao *AgentV2DAO) CreateRun(run *model.AgentRun) error {
	return database.DB.Create(run).Error
}

// UpdateRun 更新 Agent 运行记录的属性
func (dao *AgentV2DAO) UpdateRun(runID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentRun{}).Where("id = ?", runID).Updates(attrs).Error
}

// FindRun 根据用户ID和运行ID查找 Agent 运行记录
func (dao *AgentV2DAO) FindRun(userID uint, runID uint) (model.AgentRun, error) {
	var run model.AgentRun
	err := database.DB.Where("user_id = ? AND id = ?", userID, runID).First(&run).Error
	return run, err
}

// CreateStep 创建一个 Agent 步骤记录
func (dao *AgentV2DAO) CreateStep(step *model.AgentStep) error {
	return database.DB.Create(step).Error
}

// UpdateStep 更新 Agent 步骤记录的属性
func (dao *AgentV2DAO) UpdateStep(stepID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentStep{}).Where("id = ?", stepID).Updates(attrs).Error
}

// ListSteps 列出指定 Agent 运行的所有步骤
func (dao *AgentV2DAO) ListSteps(userID uint, runID uint) ([]model.AgentStep, error) {
	var steps []model.AgentStep
	err := database.DB.Model(&model.AgentStep{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_steps.agent_run_id").
		Where("agent_runs.user_id = ? AND agent_steps.agent_run_id = ?", userID, runID).
		Order("agent_steps.id asc").
		Find(&steps).Error
	return steps, err
}
