package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// ListReflections returns recent reflection records for prompt evolution.
func (dao *AgentV2DAO) ListReflections(agentName string, limit int) ([]model.AgentReflection, error) {
	var reflections []model.AgentReflection
	query := database.DB.Model(&model.AgentReflection{})
	if agentName != "" {
		query = query.Where("agent_name = ?", agentName)
	}
	if limit <= 0 {
		limit = 50
	}
	err := query.Order("id desc").Limit(limit).Find(&reflections).Error
	return reflections, err
}

// CreatePromptVersion writes an Agent prompt template version.
func (dao *AgentV2DAO) CreatePromptVersion(version *model.AgentPromptVersion) error {
	return database.DB.Create(version).Error
}

// ListPromptVersions returns prompt versions for one agent or all agents.
func (dao *AgentV2DAO) ListPromptVersions(agentName string, limit int) ([]model.AgentPromptVersion, error) {
	var versions []model.AgentPromptVersion
	query := database.DB.Model(&model.AgentPromptVersion{})
	if agentName != "" {
		query = query.Where("agent_name = ?", agentName)
	}
	if limit <= 0 {
		limit = 50
	}
	err := query.Order("id desc").Limit(limit).Find(&versions).Error
	return versions, err
}

// FindPromptVersion returns one prompt version.
func (dao *AgentV2DAO) FindPromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	var version model.AgentPromptVersion
	err := database.DB.Where("id = ?", versionID).First(&version).Error
	return version, err
}

// UpdatePromptVersion updates status or metrics for one prompt version.
func (dao *AgentV2DAO) UpdatePromptVersion(versionID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentPromptVersion{}).Where("id = ?", versionID).Updates(attrs).Error
}

// ArchiveActivePromptVersions archives active versions for an agent before activation.
func (dao *AgentV2DAO) ArchiveActivePromptVersions(agentName string, exceptID uint) error {
	query := database.DB.Model(&model.AgentPromptVersion{}).
		Where("agent_name = ? AND status = ?", agentName, "active")
	if exceptID > 0 {
		query = query.Where("id <> ?", exceptID)
	}
	return query.Update("status", "archived").Error
}

// CreateReflection writes one Agent reflection record.
func (dao *AgentV2DAO) CreateReflection(reflection *model.AgentReflection) error {
	return database.DB.Create(reflection).Error
}

// CreateEvalCase writes a reusable eval case.
func (dao *AgentV2DAO) CreateEvalCase(evalCase *model.EvalCase) error {
	return database.DB.Create(evalCase).Error
}

// ListEvalCases returns eval cases.
func (dao *AgentV2DAO) ListEvalCases(agentName string, limit int) ([]model.EvalCase, error) {
	var cases []model.EvalCase
	query := database.DB.Model(&model.EvalCase{})
	if agentName != "" {
		query = query.Where("agent_name = ?", agentName)
	}
	if limit <= 0 {
		limit = 50
	}
	err := query.Order("id desc").Limit(limit).Find(&cases).Error
	return cases, err
}

// CreateEvalRun writes an eval run result.
func (dao *AgentV2DAO) CreateEvalRun(run *model.EvalRun) error {
	return database.DB.Create(run).Error
}

// ListEvalRuns returns recent eval runs.
func (dao *AgentV2DAO) ListEvalRuns(agentName string, limit int) ([]model.EvalRun, error) {
	var runs []model.EvalRun
	query := database.DB.Model(&model.EvalRun{})
	if agentName != "" {
		query = query.Where("agent_name = ?", agentName)
	}
	if limit <= 0 {
		limit = 50
	}
	err := query.Order("id desc").Limit(limit).Find(&runs).Error
	return runs, err
}
