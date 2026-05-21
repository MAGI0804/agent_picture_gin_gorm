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
	)
	logger.LogErrorIf(err)
	seedDefaultModelConfigs()
}

func seedDefaultModelConfigs() {
	var count int64
	err := database.DB.Model(&model.ModelConfig{}).Count(&count).Error
	if err != nil || count > 0 {
		logger.LogErrorIf(err)
		return
	}

	defaults := []model.ModelConfig{
		{
			ModelName:       "mock-text",
			RequestURL:      "",
			IsTextModel:     true,
			IsImageModel:    false,
			SupportThinking: true,
			ConfigInfo: model.JSONMap{
				"provider":    "mock",
				"temperature": "0.7",
			},
		},
		{
			ModelName:       "mock-image",
			RequestURL:      "",
			IsTextModel:     false,
			IsImageModel:    true,
			SupportThinking: false,
			ConfigInfo: model.JSONMap{
				"provider": "mock",
			},
		},
	}
	logger.LogErrorIf(database.DB.Create(&defaults).Error)
}
