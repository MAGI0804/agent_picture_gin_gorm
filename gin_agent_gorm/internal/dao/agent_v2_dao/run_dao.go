package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// FindConversation 校验会话归属，避免用户访问不属于自己的会话。
func (dao *AgentV2DAO) FindConversation(userID uint, conversationID uint) (model.Conversation, error) {
	var conversation model.Conversation
	err := database.DB.Where("user_id = ? AND id = ?", userID, conversationID).First(&conversation).Error
	return conversation, err
}

// CreateMessage 写入触发本次 run 的用户消息。
func (dao *AgentV2DAO) CreateMessage(message *model.Message) error {
	return database.DB.Create(message).Error
}

// UpdateMessageAgentRunID 将用户消息和 Agent Run 绑定起来，方便前端按消息恢复执行记录。
func (dao *AgentV2DAO) UpdateMessageAgentRunID(messageID uint, agentRunID uint) error {
	return database.DB.Model(&model.Message{}).Where("id = ?", messageID).Update("agent_run_id", agentRunID).Error
}

// CreateRun 创建 Agent V2 的一次运行记录。
func (dao *AgentV2DAO) CreateRun(run *model.AgentRun) error {
	return database.DB.Create(run).Error
}

// UpdateRun 更新 Agent Run 的状态、工作流信息或 RunState 快照。
func (dao *AgentV2DAO) UpdateRun(runID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentRun{}).Where("id = ?", runID).Updates(attrs).Error
}

// FindRun 按用户校验后读取指定 Agent Run。
func (dao *AgentV2DAO) FindRun(userID uint, runID uint) (model.AgentRun, error) {
	var run model.AgentRun
	err := database.DB.Where("user_id = ? AND id = ?", userID, runID).First(&run).Error
	return run, err
}

// FindRunByIdempotencyKey returns an existing run for a user-supplied idempotency key.
func (dao *AgentV2DAO) FindRunByIdempotencyKey(userID uint, idempotencyKey string) (model.AgentRun, error) {
	var run model.AgentRun
	err := database.DB.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).
		Order("id desc").
		First(&run).Error
	return run, err
}
