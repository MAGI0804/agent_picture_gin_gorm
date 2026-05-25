package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// AgentV2DAO 封装 Agent V2 需要的数据库访问。
type AgentV2DAO struct{}

// NewAgentV2DAO 创建 Agent V2 数据访问对象。
func NewAgentV2DAO() *AgentV2DAO {
	return &AgentV2DAO{}
}

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

// CreateArtifactVersion 写入产物版本记录，后续图片生成/编辑 Agent 会调用这里。
func (dao *AgentV2DAO) CreateArtifactVersion(version *model.ArtifactVersion) error {
	return database.DB.Create(version).Error
}

// ListArtifactVersions 读取某个产物的版本链。
func (dao *AgentV2DAO) ListArtifactVersions(artifactID uint) ([]model.ArtifactVersion, error) {
	var versions []model.ArtifactVersion
	err := database.DB.Where("artifact_id = ?", artifactID).
		Order("version_no asc, id asc").
		Find(&versions).Error
	return versions, err
}

// CreateArtifactFeedback 写入用户对产物的选择、下载、评分或差评。
func (dao *AgentV2DAO) CreateArtifactFeedback(feedback *model.ArtifactFeedback) error {
	return database.DB.Create(feedback).Error
}

// CreatePromptVersion 写入 Agent prompt 模板版本。
func (dao *AgentV2DAO) CreatePromptVersion(version *model.AgentPromptVersion) error {
	return database.DB.Create(version).Error
}

// CreateReflection 写入 Agent 反思记录。
func (dao *AgentV2DAO) CreateReflection(reflection *model.AgentReflection) error {
	return database.DB.Create(reflection).Error
}
