package agent_v2_dao

import (
	"errors"

	"gorm.io/gorm"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateStep creates a persisted workflow step attempt.
func (dao *AgentV2DAO) CreateStep(step *model.AgentStep) error {
	return database.DB.Create(step).Error
}

// UpdateStep updates step result, retry status, duration, or structured output.
func (dao *AgentV2DAO) UpdateStep(stepID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentStep{}).Where("id = ?", stepID).Updates(attrs).Error
}

// FindReusableStep returns the latest completed step with the same stable input snapshot.
func (dao *AgentV2DAO) FindReusableStep(runID uint, stepKey string, inputHash string) (model.AgentStep, bool, error) {
	var step model.AgentStep
	err := database.DB.Where(
		"agent_run_id = ? AND step_key = ? AND input_hash = ? AND status = ?",
		runID,
		stepKey,
		inputHash,
		domain.StepStatusCompleted,
	).
		Order("attempt desc, id desc").
		First(&step).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return step, false, nil
	}
	if err != nil {
		return step, false, err
	}
	return step, true, nil
}

// MaxStepAttempt returns the highest persisted attempt for the same step input.
func (dao *AgentV2DAO) MaxStepAttempt(runID uint, stepKey string, inputHash string) (int, error) {
	var maxAttempt int
	err := database.DB.Model(&model.AgentStep{}).
		Where("agent_run_id = ? AND step_key = ? AND input_hash = ?", runID, stepKey, inputHash).
		Select("COALESCE(MAX(attempt), 0)").
		Scan(&maxAttempt).Error
	return maxAttempt, err
}

// CountStepAttempts counts all persisted step attempts for a run.
func (dao *AgentV2DAO) CountStepAttempts(runID uint) (int, error) {
	var count int64
	err := database.DB.Model(&model.AgentStep{}).
		Where("agent_run_id = ?", runID).
		Count(&count).Error
	return int(count), err
}

// CountStepAttemptsByKey counts all persisted attempts for one workflow step key.
func (dao *AgentV2DAO) CountStepAttemptsByKey(runID uint, stepKey string) (int, error) {
	var count int64
	err := database.DB.Model(&model.AgentStep{}).
		Where("agent_run_id = ? AND step_key = ?", runID, stepKey).
		Count(&count).Error
	return int(count), err
}

// ListSteps reads a run timeline after validating user ownership through agent_runs.
func (dao *AgentV2DAO) ListSteps(userID uint, runID uint) ([]model.AgentStep, error) {
	var steps []model.AgentStep
	err := database.DB.Model(&model.AgentStep{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_steps.agent_run_id").
		Where("agent_runs.user_id = ? AND agent_steps.agent_run_id = ?", userID, runID).
		Order("agent_steps.id asc").
		Find(&steps).Error
	return steps, err
}
