package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateTaskLedgerItem creates one run task ledger item.
func (dao *AgentV2DAO) CreateTaskLedgerItem(item *model.TaskLedgerItem) error {
	return database.DB.Create(item).Error
}

// UpdateTaskLedgerItem updates task ledger status, outputs, retry count, or error.
func (dao *AgentV2DAO) UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.TaskLedgerItem{}).Where("id = ?", itemID).Updates(attrs).Error
}

// ListTaskLedgerItems lists a run's task ledger in creation order.
func (dao *AgentV2DAO) ListTaskLedgerItems(runID uint) ([]model.TaskLedgerItem, error) {
	var items []model.TaskLedgerItem
	err := database.DB.Where("agent_run_id = ?", runID).
		Order("id asc").
		Find(&items).Error
	return items, err
}
