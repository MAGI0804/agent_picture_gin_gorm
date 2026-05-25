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

// Service 是 Agent V2 的应用服务层，负责把 HTTP 请求编排成一次 Agent Run。
type Service struct {
	dao      *agent_v2_dao.AgentV2DAO
	executor *runtime.Executor
}

// CreateRunRequest 是创建 Agent Run 的请求体。
type CreateRunRequest struct {
	Content  string `json:"content" form:"content" binding:"required"`
	TaskType string `json:"task_type" form:"task_type"`
}

// NewService 创建应用服务，并手动装配 DAO 与运行时执行器。
func NewService() *Service {
	dao := agent_v2_dao.NewAgentV2DAO()
	return &Service{
		dao:      dao,
		executor: runtime.NewExecutor(dao),
	}
}

// CreateRun 创建用户消息、Agent Run，并立即执行第一版 mock workflow。
func (svc *Service) CreateRun(
	ctx context.Context,
	userID uint,
	conversationID uint,
	request CreateRunRequest,
) (map[string]interface{}, error) {
	// 第一步：校验会话归属，v2 接口不能跨用户创建 run。
	conversation, err := svc.dao.FindConversation(userID, conversationID)
	if err != nil {
		return nil, err
	}

	// 第二步：保存触发本次 Agent Run 的用户消息。
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

	// 第三步：创建结构化 RunState，后续所有 Agent 节点都读写这份状态。
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

	// 第四步：先落库 Agent Run，再把 run_id 回写到 RunState。
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

	// 第五步：执行固定 mock workflow，验证 runtime/step/timeline 的基础链路。
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

	// 第六步：重新读取 run 和 steps，返回给前端作为第一版 timeline。
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

// GetRun 读取 Agent Run 和已保存的 step timeline。
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

// ListRunEvents 读取 SSE 事件源，目前直接复用 step timeline。
func (svc *Service) ListRunEvents(userID uint, runID uint) ([]model.AgentStep, error) {
	if _, err := svc.dao.FindRun(userID, runID); err != nil {
		return nil, err
	}
	return svc.dao.ListSteps(userID, runID)
}

// coalesce 在请求未指定值时返回默认值。
func coalesce(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// mustJSON 将对象序列化为 JSON，序列化失败时返回空对象字符串。
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
