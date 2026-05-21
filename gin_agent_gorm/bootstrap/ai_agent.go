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

	err := database.DB.AutoMigrate(
		&model.User{},
		&model.Conversation{},
		&model.Message{},
		&model.FollowUpQuestion{},
		&model.AgentRun{},
		&model.AgentStep{},
		&model.ContextMemory{},
		&model.Artifact{},
		&model.ModelConfig{},
		&model.UserModelConfig{},
		&model.UserModelPermission{},
	)
	logger.LogErrorIf(err)
	ensureAIAgentIndexes()
}

func ensureAIAgentIndexes() {
	indexes := []struct {
		value interface{}
		name  string
	}{
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
