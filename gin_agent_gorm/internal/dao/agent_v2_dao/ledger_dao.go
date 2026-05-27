package agent_v2_dao

import (
	"errors"

	"gorm.io/gorm"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateTaskLedgerItem creates a run-level workflow ledger entry.
func (dao *AgentV2DAO) CreateTaskLedgerItem(item *model.TaskLedgerItem) error {
	return database.DB.Create(item).Error
}

// FindTaskLedgerItem returns one ledger item by run and task key.
func (dao *AgentV2DAO) FindTaskLedgerItem(runID uint, taskKey string) (model.TaskLedgerItem, bool, error) {
	var item model.TaskLedgerItem
	err := database.DB.Where("agent_run_id = ? AND task_key = ?", runID, taskKey).
		Order("id desc").
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	return item, true, nil
}

// UpdateTaskLedgerItem updates ledger status, output refs, retry count, or error message.
func (dao *AgentV2DAO) UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.TaskLedgerItem{}).Where("id = ?", itemID).Updates(attrs).Error
}

// ListTaskLedgerItems lists run ledger entries in creation order.
func (dao *AgentV2DAO) ListTaskLedgerItems(runID uint) ([]model.TaskLedgerItem, error) {
	var items []model.TaskLedgerItem
	err := database.DB.Where("agent_run_id = ?", runID).
		Order("id asc").
		Find(&items).Error
	return items, err
}
