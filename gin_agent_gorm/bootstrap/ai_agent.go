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
	)
	logger.LogErrorIf(err)
}
