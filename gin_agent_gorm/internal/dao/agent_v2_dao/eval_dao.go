package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreatePromptVersion 写入 Agent prompt 模板版本。
func (dao *AgentV2DAO) CreatePromptVersion(version *model.AgentPromptVersion) error {
	return database.DB.Create(version).Error
}

// CreateReflection 写入 Agent 反思记录。
func (dao *AgentV2DAO) CreateReflection(reflection *model.AgentReflection) error {
	return database.DB.Create(reflection).Error
}
