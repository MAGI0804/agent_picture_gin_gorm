package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	memorysvc "gin-biz-web-api/internal/service/agent_v2/memory"
	"gin-biz-web-api/internal/service/agent_v2/runtime"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

const maxIdempotencyKeyLength = 128

// Service 是 Agent V2 的应用服务层，负责把 HTTP 请求编排成一次 Agent Run。
type Service struct {
	dao       *agent_v2_dao.AgentV2DAO
	executor  *runtime.Executor
	artifacts *artifactsvc.Service
	memories  *memorysvc.Service
}

// CreateRunRequest 是创建 Agent Run 的请求体。
type CreateRunRequest struct {
	Content        string `json:"content" form:"content" binding:"required"`
	TaskType       string `json:"task_type" form:"task_type"`
	IdempotencyKey string `json:"idempotency_key" form:"idempotency_key"`
}

// MemorySearchRequest 是 V2 记忆查询请求。
type MemorySearchRequest struct {
	ConversationID uint   `json:"conversation_id" form:"conversation_id"`
	Namespace      string `json:"namespace" form:"namespace"`
	Scope          string `json:"scope" form:"scope"`
	Limit          int    `json:"limit" form:"limit"`
	MarkUsed       bool   `json:"mark_used" form:"mark_used"`
}

// SelectArtifactRequest 是选择候选产物请求。
type SelectArtifactRequest struct {
	ArtifactVersionID uint `json:"artifact_version_id" form:"artifact_version_id"`
}

// NewService 创建应用服务，并手动装配 DAO 与运行时执行器。
func NewService() *Service {
	dao := agent_v2_dao.NewAgentV2DAO()
	return &Service{
		dao:       dao,
		executor:  runtime.NewExecutor(dao),
		artifacts: artifactsvc.NewService(dao),
		memories:  memorysvc.NewService(dao),
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
	idempotencyKey := normalizeIdempotencyKey(request.IdempotencyKey)
	if idempotencyKey != "" {
		existingRun, err := svc.dao.FindRunByIdempotencyKey(userID, idempotencyKey)
		if err == nil {
			steps, stepErr := svc.dao.ListSteps(userID, existingRun.ID)
			if stepErr != nil {
				return nil, stepErr
			}
			return map[string]interface{}{
				"conversation": conversation,
				"agent_run":    existingRun,
				"steps":        steps,
				"idempotent":   true,
			}, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
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
		IdempotencyKey:   idempotencyKey,
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

// SearchMemories 查询 V2 记忆。
func (svc *Service) SearchMemories(userID uint, request MemorySearchRequest) ([]model.ContextMemory, error) {
	limit := request.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return svc.memories.Search(memorysvc.SearchRequest{
		UserID:         userID,
		ConversationID: request.ConversationID,
		Namespace:      request.Namespace,
		Scope:          request.Scope,
		Limit:          limit,
		MarkUsed:       request.MarkUsed,
	})
}

// DeleteMemory 删除当前用户的一条 V2 记忆。
func (svc *Service) DeleteMemory(userID uint, memoryID uint) error {
	return svc.memories.Delete(userID, memoryID)
}

// SelectArtifact 选择当前用户有权访问的候选产物。
func (svc *Service) SelectArtifact(userID uint, artifactID uint, request SelectArtifactRequest) error {
	return svc.artifacts.SelectArtifact(artifactsvc.SelectArtifactInput{
		UserID:            userID,
		ArtifactID:        artifactID,
		ArtifactVersionID: request.ArtifactVersionID,
	})
}

// coalesce 在请求未指定值时返回默认值。
func coalesce(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func normalizeIdempotencyKey(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxIdempotencyKeyLength {
		return value
	}
	return value[:maxIdempotencyKeyLength]
}

// mustJSON 将对象序列化为 JSON，序列化失败时返回空对象字符串。
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
