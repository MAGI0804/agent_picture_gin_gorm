package bootstrap

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"
)

// setupAIAgent 初始化 AI Agent 相关数据表。
func setupAIAgent() {
	if !config.GetBool("cfg.ai_agent.auto_migrate", true) {
		return
	}

	err := database.DB.AutoMigrate(aiAgentAutoMigrateModels()...)
	logger.LogErrorIf(err)
	ensureAIAgentIndexes()
}

// aiAgentAutoMigrateModels 返回 AI Agent 相关需要自动迁移的数据模型列表。
func aiAgentAutoMigrateModels() []interface{} {
	return []interface{}{
		&model.User{},
		&model.Conversation{},
		&model.Message{},
		&model.FollowUpQuestion{},
		&model.AgentRun{},
		&model.AgentStep{},
		&model.ContextMemory{},
		&model.Artifact{},
		&model.ArtifactVersion{},
		&model.ArtifactFeedback{},
		&model.TaskLedgerItem{},
		&model.ToolInvocation{},
		&model.MemoryEvent{},
		&model.AgentPromptVersion{},
		&model.AgentReflection{},
		&model.ModelConfig{},
		&model.UserModelConfig{},
		&model.UserModelPermission{},
	}
}

// ensureAIAgentIndexes 确保 AI Agent 相关的数据库索引存在。
func ensureAIAgentIndexes() {
	indexes := []struct {
		value interface{}
		name  string
	}{
		{value: &model.AgentRun{}, name: "idx_agent_runs_user_idempotency_unique"},
		{value: &model.UserModelConfig{}, name: "idx_user_model_configs_user_id"},
		{value: &model.UserModelPermission{}, name: "idx_user_model_permissions_user_id"},
		{value: &model.UserModelPermission{}, name: "idx_user_model_permissions_model_id"},
		{value: &model.UserModelPermission{}, name: "idx_user_model_permissions_user_model"},
		{value: &model.ModelConfig{}, name: "idx_model_configs_text"},
		{value: &model.ModelConfig{}, name: "idx_model_configs_image"},
	}
	for _, index := range indexes {
		if database.DB.Migrator().HasIndex(index.value, index.name) {
			continue
		}
		logger.LogErrorIf(database.DB.Migrator().CreateIndex(index.value, index.name))
	}
}
