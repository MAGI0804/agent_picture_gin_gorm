package app

import (
	"context"
	"encoding/json"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/runtime"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"
)

0// Service Agent V2 业务服务层
type Service struct {
	dao      *agent_v2_dao.AgentV2DAO
	executor *runtime.Executor
}

// CreateRunRequest 创建运行请求参数
type CreateRunRequest struct {
	Content  string `json:"content" form:"content" binding:"required"`
	TaskType string `json:"task_type" form:"task_type"`
}

// NewService 创建 Service 实例
func NewService() *Service {
	dao := agent_v2_dao.NewAgentV2DAO()
	return &Service{
		dao:      dao,
		executor: runtime.NewExecutor(dao),
	}
}

// CreateRun 创建并执行一个新的 Agent 运行
func (svc *Service) CreateRun(ctx context.Context, userID uint, conversationID uint, request CreateRunRequest) (map[string]interface{}, error) {
	conversation, err := svc.dao.FindConversation(userID, conversationID)
	if err != nil {
		return nil, err
	}

	message := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "user",
		InputType:      "normal",
		Content:        request.Content,
	}
	if err := svc.dao.CreateMessage(&message); err != nil {
		return nil, err
	}

	state := domain.RunState{
		UserID:         userID,
		ConversationID: conversation.ID,
		TaskType:       coalesce(request.TaskType, "image_generation"),
		UserRequest:    request.Content,
		Budget: domain.RunBudget{
			MaxSteps:            12,
			MaxImageGenerations: 1,
			TimeoutSeconds:      180,
		},
		Metadata: map[string]string{
			"runtime": "agent_v2_first_day",
		},
	}

	run := model.AgentRun{
		ConversationID:   conversation.ID,
		UserID:           userID,
		TriggerMessageID: message.ID,
		Status:           domain.RunStatusCreated,
		TaskType:         state.TaskType,
		WorkflowName:     "image_generation_v2",
		WorkflowVersion:  "0.1.0",
		StateJSON:        mustJSON(state),
		BudgetJSON:       mustJSON(state.Budget),
	}
	if err := svc.dao.CreateRun(&run); err != nil {
		return nil, err
	}
	_ = svc.dao.UpdateMessageAgentRunID(message.ID, run.ID)

	state.RunID = run.ID
	flow := workflow.MockImageGenerationWorkflow()
	finalState, err := svc.executor.Execute(ctx, state, flow)
	if err != nil {
		steps, _ := svc.dao.ListSteps(userID, run.ID)
		return map[string]interface{}{
			"agent_run": run,
			"steps":     steps,
			"state":     finalState,
		}, err
	}

	run, _ = svc.dao.FindRun(userID, run.ID)
	steps, err := svc.dao.ListSteps(userID, run.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"conversation": conversation,
		"user_message": message,
		"agent_run":    run,
		"steps":        steps,
		"state":        finalState,
	}, nil
}

// GetRun 获取 Agent 运行信息
func (svc *Service) GetRun(userID uint, runID uint) (map[string]interface{}, error) {
	run, err := svc.dao.FindRun(userID, runID)
	if err != nil {
		return nil, err
	}
	steps, err := svc.dao.ListSteps(userID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"agent_run": run,
		"steps":     steps,
	}, nil
}

// ListRunEvents 列出 Agent 运行的事件（步骤）
func (svc *Service) ListRunEvents(userID uint, runID uint) ([]model.AgentStep, error) {
	if _, err := svc.dao.FindRun(userID, runID); err != nil {
		return nil, err
	}
	return svc.dao.ListSteps(userID, runID)
}

// coalesce 如果值为空则返回默认值
func coalesce(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// mustJSON 将对象序列化为 JSON，出错时返回空对象
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
