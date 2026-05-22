package agent_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentDAO 封装 AI Agent 模块的数据库访问。
type AgentDAO struct {
}

// NewAgentDAO 创建 AI Agent DAO。
func NewAgentDAO() *AgentDAO {
	return &AgentDAO{}
}

// ListConversations 查询用户会话列表。
func (dao *AgentDAO) ListConversations(userID uint) ([]model.Conversation, error) {
	var conversations []model.Conversation
	err := database.DB.Where("user_id = ?", userID).
		Order("updated_at desc, id desc").
		Find(&conversations).Error
	return conversations, err
}

// CreateConversation 创建会话。
func (dao *AgentDAO) CreateConversation(conversation *model.Conversation) error {
	return database.DB.Create(conversation).Error
}

// FindConversation 根据用户和会话 ID 查询会话，用于权限隔离。
func (dao *AgentDAO) FindConversation(userID uint, conversationID uint) (model.Conversation, error) {
	var conversation model.Conversation
	err := database.DB.Where("user_id = ? AND id = ?", userID, conversationID).First(&conversation).Error
	return conversation, err
}

// CountMessages 统计会话中已有消息数，用于首条消息生成标题。
func (dao *AgentDAO) CountMessages(userID uint, conversationID uint) (int64, error) {
	var count int64
	err := database.DB.Model(&model.Message{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Count(&count).Error
	return count, err
}

// UpdateConversationTitle 更新会话标题，并校验当前用户归属。
func (dao *AgentDAO) UpdateConversationTitle(userID uint, conversationID uint, title string) error {
	return database.DB.Model(&model.Conversation{}).
		Where("user_id = ? AND id = ?", userID, conversationID).
		Update("title", title).Error
}

// ListMessages 查询会话消息。
func (dao *AgentDAO) ListMessages(userID uint, conversationID uint) ([]model.Message, error) {
	var messages []model.Message
	err := database.DB.Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("id asc").
		Find(&messages).Error
	return messages, err
}

// CreateMessage 创建一条会话消息。
func (dao *AgentDAO) CreateMessage(message *model.Message) error {
	return database.DB.Create(message).Error
}

// UpdateMessageOptimization 更新消息对应的提示词优化信息。
func (dao *AgentDAO) UpdateMessageOptimization(messageID uint, isOptimized bool, optimizedPrompt string) error {
	return database.DB.Model(&model.Message{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"is_optimized":     isOptimized,
			"optimized_prompt": optimizedPrompt,
		}).Error
}

// UpdateMessageAgentRunID 将消息和 Agent Run 关联起来。
func (dao *AgentDAO) UpdateMessageAgentRunID(messageID uint, agentRunID uint) error {
	return database.DB.Model(&model.Message{}).Where("id = ?", messageID).Update("agent_run_id", agentRunID).Error
}

// CreateFollowUpQuestions 批量创建补充问题。
func (dao *AgentDAO) CreateFollowUpQuestions(questions []model.FollowUpQuestion) error {
	if len(questions) == 0 {
		return nil
	}
	return database.DB.Create(&questions).Error
}

// AnswerFollowUpQuestions 写入用户对补充问题的回答。
func (dao *AgentDAO) AnswerFollowUpQuestions(userID uint, questionIDs []uint, answer string) error {
	if len(questionIDs) == 0 {
		return nil
	}
	return database.DB.Model(&model.FollowUpQuestion{}).
		Where("user_id = ? AND id IN ?", userID, questionIDs).
		Updates(map[string]interface{}{"answer": answer, "status": "answered"}).Error
}

// ListPendingQuestions 查询尚未回答的补充问题。
func (dao *AgentDAO) ListPendingQuestions(userID uint, conversationID uint) ([]model.FollowUpQuestion, error) {
	var questions []model.FollowUpQuestion
	err := database.DB.Where("user_id = ? AND conversation_id = ? AND status = ?", userID, conversationID, "pending").
		Order("id asc").
		Find(&questions).Error
	return questions, err
}

// ListConversationQuestions queries recent follow-up questions for context assembly.
func (dao *AgentDAO) ListConversationQuestions(
	userID uint,
	conversationID uint,
	limit int,
) ([]model.FollowUpQuestion, error) {
	var questions []model.FollowUpQuestion
	err := database.DB.Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("id desc").
		Limit(limit).
		Find(&questions).Error
	return questions, err
}

// CreateAgentRun 创建一次多 Agent 总任务。
func (dao *AgentDAO) CreateAgentRun(run *model.AgentRun) error {
	return database.DB.Create(run).Error
}

// UpdateAgentRun 更新 Agent Run 状态或错误信息。
func (dao *AgentDAO) UpdateAgentRun(runID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentRun{}).Where("id = ?", runID).Updates(attrs).Error
}

// ListAgentRunsByIDs returns run metadata for message display.
func (dao *AgentDAO) ListAgentRunsByIDs(userID uint, runIDs []uint) ([]model.AgentRun, error) {
	if len(runIDs) == 0 {
		return []model.AgentRun{}, nil
	}
	var runs []model.AgentRun
	err := database.DB.Where("user_id = ? AND id IN ?", userID, runIDs).
		Order("id asc").
		Find(&runs).Error
	return runs, err
}

// CreateAgentStep 创建一个 Agent 子步骤记录。
func (dao *AgentDAO) CreateAgentStep(step *model.AgentStep) error {
	return database.DB.Create(step).Error
}

// CreateAgentSteps batch-inserts Agent step records to reduce DB round-trips.
func (dao *AgentDAO) CreateAgentSteps(steps []model.AgentStep) error {
	if len(steps) == 0 {
		return nil
	}
	return database.DB.Create(&steps).Error
}

// ListAgentSteps 查询指定 Agent Run 的步骤列表，并校验用户归属。
func (dao *AgentDAO) ListAgentSteps(userID uint, runID uint) ([]model.AgentStep, error) {
	var steps []model.AgentStep
	err := database.DB.Model(&model.AgentStep{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_steps.agent_run_id").
		Where("agent_runs.user_id = ? AND agent_steps.agent_run_id = ?", userID, runID).
		Order("agent_steps.id asc").
		Find(&steps).Error
	return steps, err
}

// CreateContextMemory 保存一条会话上下文记忆。
func (dao *AgentDAO) CreateContextMemory(memory *model.ContextMemory) error {
	return database.DB.Create(memory).Error
}

// CreateContextMemories batch-inserts context memories.
func (dao *AgentDAO) CreateContextMemories(memories []model.ContextMemory) error {
	if len(memories) == 0 {
		return nil
	}
	return database.DB.Create(&memories).Error
}

// ListContextMemories 查询会话上下文记忆。
func (dao *AgentDAO) ListContextMemories(userID uint, conversationID uint, limit int) ([]model.ContextMemory, error) {
	var memories []model.ContextMemory
	err := database.DB.Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("score desc, id desc").
		Limit(limit).
		Find(&memories).Error
	return memories, err
}

// CreateArtifact 创建产物元数据。
func (dao *AgentDAO) CreateArtifact(artifact *model.Artifact) error {
	return database.DB.Create(artifact).Error
}

// CreateArtifacts batch-inserts artifact metadata to reduce DB round-trips.
func (dao *AgentDAO) CreateArtifacts(artifacts []model.Artifact) ([]model.Artifact, error) {
	if len(artifacts) == 0 {
		return artifacts, nil
	}
	err := database.DB.Create(&artifacts).Error
	return artifacts, err
}

// ListArtifacts 查询会话产物列表。
func (dao *AgentDAO) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	var artifacts []model.Artifact
	err := database.DB.Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("id desc").
		Find(&artifacts).Error
	return artifacts, err
}

// FindArtifact 查询用户有权访问的产物。
func (dao *AgentDAO) FindArtifact(userID uint, artifactID uint) (model.Artifact, error) {
	var artifact model.Artifact
	err := database.DB.Where("user_id = ? AND id = ?", userID, artifactID).First(&artifact).Error
	return artifact, err
}

// FindModelConfig 查询用户绑定的模型配置。
func (dao *AgentDAO) FindModelConfig() (model.ModelConfig, error) {
	var config model.ModelConfig
	err := database.DB.Order("id asc").First(&config).Error
	return config, err
}

// SaveModelConfig 创建或更新全局模型配置。
func (dao *AgentDAO) SaveModelConfig(config *model.ModelConfig) error {
	if config.ID > 0 {
		return database.DB.Model(&model.ModelConfig{}).
			Where("id = ?", config.ID).
			Updates(map[string]interface{}{
				"model_name":       config.ModelName,
				"request_url":      config.RequestURL,
				"is_text_model":    config.IsTextModel,
				"is_image_model":   config.IsImageModel,
				"support_thinking": config.SupportThinking,
				"config_info":      config.ConfigInfo,
			}).Error
	}

	var exists model.ModelConfig
	err := database.DB.Order("id asc").First(&exists).Error
	if err != nil {
		return database.DB.Create(config).Error
	}

	config.ID = exists.ID
	return dao.SaveModelConfig(config)
}

// FindUserModelConfig 查询用户绑定的模型配置。
func (dao *AgentDAO) FindUserModelConfig(userID uint) (model.UserModelConfig, error) {
	var config model.UserModelConfig
	err := database.DB.Where("user_id = ?", userID).First(&config).Error
	return config, err
}

// SaveUserModelConfig 创建或更新用户绑定的模型配置。
func (dao *AgentDAO) SaveUserModelConfig(config *model.UserModelConfig) error {
	var exists model.UserModelConfig
	err := database.DB.Where("user_id = ?", config.UserID).First(&exists).Error
	if err == nil {
		config.ID = exists.ID
		return database.DB.Save(config).Error
	}
	return database.DB.Create(config).Error
}

// SaveUserModelSelection creates or updates the user's selected global models.
func (dao *AgentDAO) SaveUserModelSelection(
	userID uint,
	textModelConfigID uint,
	imageModelConfigID uint,
) error {
	config := model.UserModelConfig{
		UserID:                     userID,
		SelectedTextModelConfigID:  textModelConfigID,
		SelectedImageModelConfigID: imageModelConfigID,
		Provider:                   "",
		ChatModel:                  "",
		ImageModel:                 "",
		BaseURL:                    "",
		APIKey:                     "",
		Temperature:                "0.7",
	}
	return database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"selected_text_model_config_id",
			"selected_image_model_config_id",
			"updated_at",
		}),
	}).Create(&config).Error
}

// ListPermittedModelConfigs returns global model configs the user is allowed to use.
// If the user has no permission records, all models are permitted by default.
func (dao *AgentDAO) ListPermittedModelConfigs(
	userID uint,
	isTextModel bool,
	isImageModel bool,
) ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	query := database.DB.Model(&model.ModelConfig{})

	if isTextModel {
		query = query.Where("model_configs.is_text_model = ?", true)
	}
	if isImageModel {
		query = query.Where("model_configs.is_image_model = ?", true)
	}

	var hasPermissions bool
	err := database.DB.Model(&model.UserModelPermission{}).
		Where("user_id = ?", userID).
		First(&model.UserModelPermission{}).Error
	if err == nil {
		hasPermissions = true
	} else if err == gorm.ErrRecordNotFound {
		hasPermissions = false
	} else {
		return nil, err
	}

	if hasPermissions {
		query = query.
			Joins("LEFT JOIN user_model_permissions ON user_model_permissions.model_config_id = model_configs.id AND user_model_permissions.user_id = ?", userID).
			Where("user_model_permissions.id IS NULL OR user_model_permissions.can_use = ?", true)
	}

	err = query.Order("model_configs.id desc").Find(&configs).Error
	return configs, err
}

// FindPermittedModelConfig returns one global model config if the user has permission.
func (dao *AgentDAO) FindPermittedModelConfig(
	userID uint,
	modelConfigID uint,
) (model.ModelConfig, error) {
	var config model.ModelConfig
	err := database.DB.Model(&model.ModelConfig{}).
		Select("model_configs.*").
		Joins("JOIN user_model_permissions ON user_model_permissions.model_config_id = model_configs.id").
		Where("user_model_permissions.user_id = ? AND user_model_permissions.can_use = ? AND model_configs.id = ?", userID, true, modelConfigID).
		First(&config).Error
	return config, err
}
