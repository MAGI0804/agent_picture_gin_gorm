package agent_v2_dao

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// FindConversation validates that a conversation belongs to the current user.
func (dao *AgentV2DAO) FindConversation(userID uint, conversationID uint) (model.Conversation, error) {
	var conversation model.Conversation
	err := database.DB.Where("user_id = ? AND id = ?", userID, conversationID).First(&conversation).Error
	return conversation, err
}

// CreateMessage writes one message row.
func (dao *AgentV2DAO) CreateMessage(message *model.Message) error {
	return database.DB.Create(message).Error
}

// UpdateMessageAgentRunID links a message to its agent run.
func (dao *AgentV2DAO) UpdateMessageAgentRunID(messageID uint, agentRunID uint) error {
	return database.DB.Model(&model.Message{}).Where("id = ?", messageID).Update("agent_run_id", agentRunID).Error
}

// CreateRun creates one Agent V2 run.
func (dao *AgentV2DAO) CreateRun(run *model.AgentRun) error {
	return database.DB.Create(run).Error
}

// CreateMessageAndRun atomically creates the trigger message and run, then links them.
func (dao *AgentV2DAO) CreateMessageAndRun(message *model.Message, run *model.AgentRun) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		run.TriggerMessageID = message.ID
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		message.AgentRunID = run.ID
		return tx.Model(&model.Message{}).
			Where("id = ?", message.ID).
			Update("agent_run_id", run.ID).Error
	})
}

// UpdateRun updates Agent Run state, workflow metadata, budget, or error fields.
func (dao *AgentV2DAO) UpdateRun(runID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.AgentRun{}).Where("id = ?", runID).Updates(attrs).Error
}

// FindRun reads one run after validating user ownership.
func (dao *AgentV2DAO) FindRun(userID uint, runID uint) (model.AgentRun, error) {
	var run model.AgentRun
	err := database.DB.Where("user_id = ? AND id = ?", userID, runID).First(&run).Error
	return run, err
}

// FindRunStatus reads only the current run status for executor-side cancellation checks.
func (dao *AgentV2DAO) FindRunStatus(runID uint) (string, error) {
	var run model.AgentRun
	err := database.DB.Select("status").Where("id = ?", runID).First(&run).Error
	return run.Status, err
}

// ClaimRunStatus atomically moves a run from one status to another.
func (dao *AgentV2DAO) ClaimRunStatus(userID uint, runID uint, fromStatus string, toStatus string) (bool, error) {
	attrs := map[string]interface{}{
		"status": toStatus,
	}
	if toStatus == domain.RunStatusRunning {
		attrs["started_at"] = int(time.Now().Unix())
		attrs["error_message"] = ""
	}
	result := database.DB.Model(&model.AgentRun{}).
		Where("user_id = ? AND id = ? AND status = ?", userID, runID, fromStatus).
		Updates(attrs)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// FindRunByIdempotencyKey returns an existing run for a user-supplied idempotency key.
func (dao *AgentV2DAO) FindRunByIdempotencyKey(userID uint, idempotencyKey string) (model.AgentRun, error) {
	var run model.AgentRun
	err := database.DB.Where(
		"user_id = ? AND (idempotency_key = ? OR idempotency_key_unique = ?)",
		userID,
		idempotencyKey,
		idempotencyKey,
	).
		Order("id desc").
		First(&run).Error
	return run, err
}

// MarkTimedOutRunningRuns fails stale running runs older than the supplied cutoff.
func (dao *AgentV2DAO) MarkTimedOutRunningRuns(cutoffUnix int, reason string) (int64, error) {
	if cutoffUnix <= 0 {
		return 0, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "agent v2 run timed out"
	}
	result := database.DB.Model(&model.AgentRun{}).
		Where("status = ? AND started_at > 0 AND started_at < ?", domain.RunStatusRunning, cutoffUnix).
		Updates(map[string]interface{}{
			"status":        domain.RunStatusFailed,
			"error_message": reason,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
