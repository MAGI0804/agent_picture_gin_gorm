package agent_v2_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// CreateTaskLedgerItem 创建一个运行任务台账项。
func (dao *AgentV2DAO) CreateTaskLedgerItem(item *model.TaskLedgerItem) error {
	return database.DB.Create(item).Error
}

// UpdateTaskLedgerItem 更新任务台账状态、输出、重试次数或错误信息。
func (dao *AgentV2DAO) UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error {
	return database.DB.Model(&model.TaskLedgerItem{}).Where("id = ?", itemID).Updates(attrs).Error
}

// ListTaskLedgerItems 按创建顺序列出运行的任务台账。
func (dao *AgentV2DAO) ListTaskLedgerItems(runID uint) ([]model.TaskLedgerItem, error) {
	var items []model.TaskLedgerItem
	err := database.DB.Where("agent_run_id = ?", runID).
		Order("id asc").
		Find(&items).Error
	return items, err
}
