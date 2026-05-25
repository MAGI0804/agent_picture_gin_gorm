package agent_v2_dao

import (
	"time"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

// MemoryFilter 描述 V2 记忆查询的权限和命名空间筛选条件。
type MemoryFilter struct {
	UserID         uint
	ConversationID uint
	Namespace      string
	Scope          string
	Limit          int
}

// CreateMemory 保存一条 V2 记忆。
func (dao *AgentV2DAO) CreateMemory(memory *model.ContextMemory) error {
	return database.DB.Create(memory).Error
}

// ListMemories 查询当前用户有权访问的未删除记忆。
func (dao *AgentV2DAO) ListMemories(filter MemoryFilter) ([]model.ContextMemory, error) {
	var memories []model.ContextMemory
	query := database.DB.Where("user_id = ? AND deleted_at = ?", filter.UserID, 0)

	if filter.ConversationID > 0 {
		query = query.Where("conversation_id = ?", filter.ConversationID)
	}
	if filter.Namespace != "" {
		query = query.Where("namespace = ?", filter.Namespace)
	}
	if filter.Scope != "" {
		query = query.Where("scope = ?", filter.Scope)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	err := query.
		Order("confidence desc, score desc, last_used_at desc, id desc").
		Find(&memories).Error
	return memories, err
}

// UpdateMemoryUsage 记录某条记忆被工作流使用。
func (dao *AgentV2DAO) UpdateMemoryUsage(memoryID uint) error {
	return database.DB.Model(&model.ContextMemory{}).
		Where("id = ? AND deleted_at = ?", memoryID, 0).
		Updates(map[string]interface{}{
			"use_count":    gorm.Expr("use_count + ?", 1),
			"last_used_at": int(time.Now().Unix()),
		}).Error
}

// SoftDeleteMemory 将标记记为已删除，同时保留可审计性。
func (dao *AgentV2DAO) SoftDeleteMemory(userID uint, memoryID uint) error {
	return database.DB.Model(&model.ContextMemory{}).
		Where("user_id = ? AND id = ? AND deleted_at = ?", userID, memoryID, 0).
		Update("deleted_at", int(time.Now().Unix())).Error
}

// CreateMemoryEvent 记录一条记忆审计事件。
func (dao *AgentV2DAO) CreateMemoryEvent(event *model.MemoryEvent) error {
	return database.DB.Create(event).Error
}
