package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	"gin-biz-web-api/internal/service/agent_svc"
	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	evalsvc "gin-biz-web-api/internal/service/agent_v2/eval"
	memorysvc "gin-biz-web-api/internal/service/agent_v2/memory"
	"gin-biz-web-api/internal/service/agent_v2/runtime"
	agentsecurity "gin-biz-web-api/internal/service/agent_v2/security"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

const (
	maxIdempotencyKeyLength = 128
	minReviewMemoryScore    = 0.70
	defaultRunMaxToolCalls  = 16

	// MaxImageUploadBytes bounds V2 reference/edit image uploads before decoding.
	MaxImageUploadBytes  = int64(10 << 20)
	maxImageUploadPixels = 36_000_000
)

// Service 是 Agent V2 的应用服务层，负责把 HTTP 请求编排成一次 Agent Run。
type Service struct {
	dao       *agent_v2_dao.AgentV2DAO
	executor  *runtime.Executor
	artifacts *artifactsvc.Service
	memories  *memorysvc.Service
	evolution *evalsvc.EvolutionService
	runQueue  RunQueue
}

// CreateRunRequest 是创建 Agent Run 的请求体。
type CreateRunRequest struct {
	Content              string `json:"content" form:"content"`
	TaskType             string `json:"task_type" form:"task_type"`
	IdempotencyKey       string `json:"idempotency_key" form:"idempotency_key"`
	TextModelConfigID    uint   `json:"text_model_config_id" form:"text_model_config_id"`
	ImageModelConfigID   uint   `json:"image_model_config_id" form:"image_model_config_id"`
	CandidateCount       int    `json:"candidate_count" form:"candidate_count"`
	DisableClarification bool   `json:"disable_clarification" form:"disable_clarification"`
	ArtifactIDs          []uint `json:"artifact_ids" form:"artifact_ids"`
}

// ResumeRunRequest carries the user's clarification answer for a waiting run.
type ResumeRunRequest struct {
	Content string `json:"content" form:"content" binding:"required"`
}

// MemorySearchRequest 是 V2 记忆查询请求。
type MemorySearchRequest struct {
	ConversationID uint   `json:"conversation_id" form:"conversation_id"`
	Namespace      string `json:"namespace" form:"namespace"`
	Scope          string `json:"scope" form:"scope"`
	Kind           string `json:"kind" form:"kind"`
	Limit          int    `json:"limit" form:"limit"`
	MarkUsed       bool   `json:"mark_used" form:"mark_used"`
}

// PromoteMemoryRequest confirms a draft memory proposal.
type PromoteMemoryRequest struct {
	Confidence float64 `json:"confidence" form:"confidence"`
}

// UpdateMemoryRequest edits or disables one memory.
type UpdateMemoryRequest struct {
	Content    string   `json:"content" form:"content"`
	Confidence *float64 `json:"confidence" form:"confidence"`
	Disabled   bool     `json:"disabled" form:"disabled"`
}

// SelectArtifactRequest 是选择候选产物请求。
type SelectArtifactRequest struct {
	ArtifactVersionID uint `json:"artifact_version_id" form:"artifact_version_id"`
}

// ArtifactFeedbackRequest records explicit user feedback for a V2 artifact.
type ArtifactFeedbackRequest struct {
	ArtifactVersionID uint   `json:"artifact_version_id" form:"artifact_version_id"`
	FeedbackType      string `json:"feedback_type" form:"feedback_type" binding:"required"`
	Rating            int    `json:"rating" form:"rating"`
	Comment           string `json:"comment" form:"comment"`
}

// UploadArtifactInput carries a user uploaded reference or edit source image.
type UploadArtifactInput struct {
	ConversationID uint
	FileName       string
	ContentType    string
	Content        []byte
}

// EditArtifactRequest appends an edited image version to an existing artifact.
type EditArtifactRequest struct {
	ArtifactVersionID  uint   `json:"artifact_version_id" form:"artifact_version_id"`
	Prompt             string `json:"prompt" form:"prompt" binding:"required"`
	ImageModelConfigID uint   `json:"image_model_config_id" form:"image_model_config_id"`
}

// RenderTextRequest creates a text-layer SVG artifact from an existing image artifact.
type RenderTextRequest struct {
	ArtifactVersionID uint   `json:"artifact_version_id" form:"artifact_version_id"`
	Title             string `json:"title" form:"title" binding:"required"`
	Subtitle          string `json:"subtitle" form:"subtitle"`
	Brand             string `json:"brand" form:"brand"`
}

type EvolutionQueryRequest struct {
	AgentName string `json:"agent_name" form:"agent_name"`
	Limit     int    `json:"limit" form:"limit"`
}

type DraftPromptVersionRequest struct {
	AgentName string `json:"agent_name" form:"agent_name" binding:"required"`
	Limit     int    `json:"limit" form:"limit"`
}

type EvalCaseRequest struct {
	AgentName    string  `json:"agent_name" form:"agent_name" binding:"required"`
	Name         string  `json:"name" form:"name" binding:"required"`
	InputJSON    string  `json:"input_json" form:"input_json" binding:"required"`
	ExpectedJSON string  `json:"expected_json" form:"expected_json"`
	TagsJSON     string  `json:"tags_json" form:"tags_json"`
	Weight       float64 `json:"weight" form:"weight"`
}

type EvalRunRequest struct {
	EvalCaseID      uint    `json:"eval_case_id" form:"eval_case_id" binding:"required"`
	PromptVersionID uint    `json:"prompt_version_id" form:"prompt_version_id"`
	AgentName       string  `json:"agent_name" form:"agent_name" binding:"required"`
	Status          string  `json:"status" form:"status"`
	Score           float64 `json:"score" form:"score"`
	MetricsJSON     string  `json:"metrics_json" form:"metrics_json"`
	ErrorMessage    string  `json:"error_message" form:"error_message"`
}

// RunEvent is a normalized polling/SSE event assembled from steps, ledger, and tool invocations.
type RunEvent struct {
	Cursor         int                   `json:"cursor"`
	Type           string                `json:"type"`
	ID             uint                  `json:"id"`
	CreatedAt      int                   `json:"created_at"`
	Step           *model.AgentStep      `json:"step,omitempty"`
	LedgerItem     *model.TaskLedgerItem `json:"ledger_item,omitempty"`
	ToolInvocation *model.ToolInvocation `json:"tool_invocation,omitempty"`
}

// RunEventsResponse is the stable polling response for run events.
type RunEventsResponse struct {
	Events []RunEvent `json:"events"`
	Cursor int        `json:"cursor"`
}

// NewService 创建应用服务，并手动装配 DAO 与运行时执行器。
func NewService() *Service {
	return newService(NewDefaultRunQueue())
}

func NewServiceWithRunQueue(runQueue RunQueue) *Service {
	return newService(runQueue)
}

func newService(runQueue RunQueue) *Service {
	dao := agent_v2_dao.NewAgentV2DAO()
	return &Service{
		dao:       dao,
		executor:  runtime.NewExecutor(dao),
		artifacts: artifactsvc.NewService(dao),
		memories:  memorysvc.NewService(dao),
		evolution: evalsvc.NewEvolutionService(dao),
		runQueue:  runQueue,
	}
}

// CreateRun 创建用户消息、Agent Run，并立即执行第一版 mock workflow。
func (svc *Service) CreateRun(
	ctx context.Context,
	userID uint,
	conversationID uint,
	request CreateRunRequest,
) (map[string]interface{}, error) {
	return svc.createRun(ctx, userID, conversationID, request, false)
}

// CreateRunAsync creates a run, marks it queued, and executes the workflow in the background.
func (svc *Service) CreateRunAsync(
	ctx context.Context,
	userID uint,
	conversationID uint,
	request CreateRunRequest,
) (map[string]interface{}, error) {
	return svc.createRun(ctx, userID, conversationID, request, true)
}

func (svc *Service) createRun(
	ctx context.Context,
	userID uint,
	conversationID uint,
	request CreateRunRequest,
	async bool,
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
			return svc.idempotentRunResponse(conversation, userID, existingRun)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	imageConfig, err := svc.resolveRuntimeModelConfig(userID, "image", request.ImageModelConfigID)
	if err != nil {
		return nil, err
	}
	textConfig, textConfigErr := svc.resolveRuntimeModelConfig(userID, "text", request.TextModelConfigID)
	visionConfig, visionConfigErr := svc.resolveVisionRuntimeModelConfig(userID)

	registry := tools.NewRegistry()
	if err := registerSafetyTool(registry, svc.dao); err != nil {
		return nil, err
	}
	imageAdapter := tools.NewLegacyProviderAdapter(imageConfig.Config)
	if err := registry.Register(tools.InstrumentTool(tools.Tool{
		Name:          runtimeImageModelName(imageConfig.Config),
		Kind:          tools.KindImageGeneration,
		Provider:      imageConfig.Config.Provider,
		Model:         runtimeImageModelName(imageConfig.Config),
		ModelConfigID: imageConfig.GlobalID,
		Capability: tools.Capability{
			MaxPromptChars:  8000,
			SupportedRatios: []string{"1:1", "3:4", "4:3", "16:9", "9:16"},
			MaxCandidates:   3,
			CostPolicy:      "real_provider",
		},
		ImageGenerationProvider: imageAdapter,
	}, svc.dao)); err != nil {
		return nil, err
	}
	if err := registry.Register(tools.InstrumentTool(tools.Tool{
		Name:          runtimeImageModelName(imageConfig.Config) + "-edit",
		Kind:          tools.KindImageEdit,
		Provider:      imageConfig.Config.Provider,
		Model:         runtimeImageModelName(imageConfig.Config),
		ModelConfigID: imageConfig.GlobalID,
		Capability: tools.Capability{
			MaxPromptChars:     8000,
			SupportsImageInput: true,
			MaxCandidates:      1,
			CostPolicy:         "real_provider",
		},
		ImageEditProvider: imageAdapter,
	}, svc.dao)); err != nil {
		return nil, err
	}
	if textConfigErr == nil {
		textAdapter := tools.NewLegacyProviderAdapter(textConfig.Config)
		_ = registry.Register(tools.InstrumentTool(tools.Tool{
			Name:          runtimeTextModelName(textConfig.Config),
			Kind:          tools.KindText,
			Provider:      textConfig.Config.Provider,
			Model:         runtimeTextModelName(textConfig.Config),
			ModelConfigID: textConfig.GlobalID,
			Capability: tools.Capability{
				MaxPromptChars: 8000,
				CostPolicy:     "real_provider",
			},
			TextProvider: textAdapter,
		}, svc.dao))
	}
	if visionConfigErr == nil {
		visionProvider := tools.NewGoogleVisionProvider(visionConfig.Config)
		_ = registry.Register(tools.InstrumentTool(tools.Tool{
			Name:          runtimeTextModelName(visionConfig.Config),
			Kind:          tools.KindVision,
			Provider:      visionConfig.Config.Provider,
			Model:         runtimeTextModelName(visionConfig.Config),
			ModelConfigID: visionConfig.GlobalID,
			Capability: tools.Capability{
				SupportsImageInput: true,
				CostPolicy:         "real_provider",
			},
			VisionProvider: visionProvider,
		}, svc.dao))
		_ = registry.Register(tools.InstrumentTool(tools.Tool{
			Name:          runtimeTextModelName(visionConfig.Config) + "-ocr",
			Kind:          tools.KindOCR,
			Provider:      visionConfig.Config.Provider,
			Model:         runtimeTextModelName(visionConfig.Config),
			ModelConfigID: visionConfig.GlobalID,
			Capability: tools.Capability{
				SupportsImageInput: true,
				CostPolicy:         "real_provider",
			},
			OCRProvider: visionProvider,
		}, svc.dao))
	}

	request.Content = strings.TrimSpace(request.Content)
	if request.Content == "" {
		request.Content = "compose final image from uploaded assets"
	}

	// 第二步：保存触发本次 Agent Run 的用户消息。
	message := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "user",
		InputType:      "normal",
		Content:        request.Content,
	}
	// 第三步：创建结构化 RunState，后续所有 Agent 节点都读写这份状态。
	state := domain.RunState{
		UserID:         userID,
		ConversationID: conversation.ID,
		TaskType:       coalesce(request.TaskType, "image_generation"),
		UserRequest:    request.Content,
		Budget: domain.RunBudget{
			MaxSteps:            12,
			MaxImageGenerations: normalizeCandidateCount(request.CandidateCount),
			MaxToolCalls:        defaultRunMaxToolCalls,
			TimeoutSeconds:      180,
			MaxAutoRefines:      1,
		},
		Metadata: map[string]string{
			"runtime":               "agent_v2_real_image_generation",
			"image_model_config_id": uintToString(imageConfig.GlobalID),
			"text_model_config_id":  uintToString(textConfig.GlobalID),
			"image_model_provider":  imageConfig.Config.Provider,
			"image_model_name":      runtimeImageModelName(imageConfig.Config),
		},
	}
	state.Metadata["disable_clarification"] = "true"
	if len(request.ArtifactIDs) > 0 {
		state.Metadata["input_artifact_ids"] = mustJSON(request.ArtifactIDs)
	}
	if visionConfigErr == nil {
		state.Metadata["vision_model_config_id"] = uintToString(visionConfig.GlobalID)
		state.Metadata["vision_model_provider"] = visionConfig.Config.Provider
		state.Metadata["vision_model_name"] = runtimeTextModelName(visionConfig.Config)
	}
	promptMemories, err := svc.memories.PromptContext(memorysvc.PromptContextRequest{
		UserID:        userID,
		Limit:         8,
		MinConfidence: 0.70,
	})
	if err != nil {
		return nil, err
	}
	state.MemoryContext = memoryItemsFromModel(promptMemories)

	// 第四步：先落库 Agent Run，再把 run_id 回写到 RunState。
	run := model.AgentRun{
		ConversationID:       conversation.ID,
		UserID:               userID,
		Status:               domain.RunStatusCreated,
		TaskType:             state.TaskType,
		WorkflowName:         "image_generation_v2",
		WorkflowVersion:      "0.7.0",
		StateJSON:            mustJSON(state),
		BudgetJSON:           mustJSON(state.Budget),
		IdempotencyKey:       idempotencyKey,
		IdempotencyKeyUnique: idempotencyKeyUniqueValue(idempotencyKey),
		ImageModelName:       runtimeImageModelName(imageConfig.Config),
	}
	if textConfigErr == nil {
		run.TextModelName = runtimeTextModelName(textConfig.Config)
	}
	if err := svc.dao.CreateMessageAndRun(&message, &run); err != nil {
		if idempotencyKey != "" && isUniqueConstraintError(err) {
			existingRun, findErr := svc.dao.FindRunByIdempotencyKey(userID, idempotencyKey)
			if findErr == nil {
				return svc.idempotentRunResponse(conversation, userID, existingRun)
			}
		}
		return nil, err
	}

	// 第五步：执行固定真实图片 workflow，调用 V2 tool registry 并写入 artifact/version。
	state.RunID = run.ID
	flow := workflow.ImageGenerationWorkflow(workflow.ImageGenerationWorkflowOptions{
		Registry:            registry,
		ArtifactWriter:      svc.artifacts,
		TextModelConfigID:   textConfig.GlobalID,
		ImageModelConfigID:  imageConfig.GlobalID,
		VisionModelConfigID: visionConfig.GlobalID,
		OCRModelConfigID:    visionConfig.GlobalID,
		CandidateCount:      normalizeCandidateCount(request.CandidateCount),
		ModelProvider:       imageConfig.Config.Provider,
		ModelName:           runtimeImageModelName(imageConfig.Config),
	})
	if async {
		if svc.runQueue == nil {
			err := errors.New("agent v2 run queue is not configured")
			_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
				"status":        domain.RunStatusFailed,
				"error_message": err.Error(),
			})
			return nil, err
		}
		state.RunID = run.ID
		if err := svc.dao.UpdateRun(run.ID, map[string]interface{}{
			"status":     domain.RunStatusQueued,
			"state_json": mustJSON(state),
		}); err != nil {
			return nil, err
		}
		run.Status = domain.RunStatusQueued
		run.StateJSON = mustJSON(state)
		if err := svc.runQueue.EnqueueAgentRun(ctx, AgentRunQueuePayload{
			RunID:          run.ID,
			UserID:         userID,
			ConversationID: conversation.ID,
		}); err != nil {
			_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
				"status":        domain.RunStatusFailed,
				"error_message": err.Error(),
			})
			return nil, err
		}
		return map[string]interface{}{
			"conversation": conversation,
			"user_message": message,
			"agent_run":    run,
			"queued":       true,
		}, nil
	}

	finalState, assistantMessage, err := svc.executePreparedRun(ctx, userID, conversation, run, state, flow)
	if err != nil {
		steps, _ := svc.dao.ListSteps(userID, run.ID)
		ledgerItems, _ := svc.dao.ListTaskLedgerItems(run.ID)
		toolInvocations, _ := svc.dao.ListToolInvocationsByRun(userID, run.ID)
		if errors.Is(err, runtime.ErrRunWaitingForUser) {
			run, _ = svc.dao.FindRun(userID, run.ID)
			return map[string]interface{}{
				"agent_run":         run,
				"steps":             steps,
				"task_ledger_items": ledgerItems,
				"tool_invocations":  toolInvocations,
				"state":             finalState,
			}, nil
		}
		return map[string]interface{}{
			"agent_run":         run,
			"steps":             steps,
			"task_ledger_items": ledgerItems,
			"tool_invocations":  toolInvocations,
			"state":             finalState,
		}, err
	}

	// 第六步：重新读取 run 和 steps，返回给前端作为第一版 timeline。
	run, _ = svc.dao.FindRun(userID, run.ID)
	steps, err := svc.dao.ListSteps(userID, run.ID)
	if err != nil {
		return nil, err
	}
	ledgerItems, err := svc.dao.ListTaskLedgerItems(run.ID)
	if err != nil {
		return nil, err
	}
	toolInvocations, err := svc.dao.ListToolInvocationsByRun(userID, run.ID)
	if err != nil {
		return nil, err
	}
	artifacts, err := svc.artifacts.ListArtifacts(userID, conversation.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"conversation":      conversation,
		"user_message":      message,
		"assistant_message": assistantMessage,
		"agent_run":         run,
		"steps":             steps,
		"task_ledger_items": ledgerItems,
		"tool_invocations":  toolInvocations,
		"artifacts":         publicArtifacts(artifacts),
		"state":             finalState,
	}, nil
}

func (svc *Service) executePreparedRun(
	ctx context.Context,
	userID uint,
	conversation model.Conversation,
	run model.AgentRun,
	state domain.RunState,
	flow workflow.Workflow,
) (domain.RunState, model.Message, error) {
	finalState, err := svc.executor.Execute(ctx, state, flow)
	if err != nil {
		return finalState, model.Message{}, err
	}
	if err := svc.recordRunReviewScores(userID, finalState); err != nil {
		return finalState, model.Message{}, err
	}

	assistantMessage := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "assistant",
		InputType:      "agent_result",
		Content:        "Agent V2 已完成图片生成，并写入产物版本。",
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&assistantMessage); err != nil {
		return finalState, model.Message{}, err
	}
	return finalState, assistantMessage, nil
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
	ledgerItems, err := svc.dao.ListTaskLedgerItems(runID)
	if err != nil {
		return nil, err
	}
	toolInvocations, err := svc.dao.ListToolInvocationsByRun(userID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"agent_run":         run,
		"steps":             steps,
		"task_ledger_items": ledgerItems,
		"tool_invocations":  toolInvocations,
	}, nil
}

// CancelRun marks a queued or running V2 run as cancelled.
func (svc *Service) CancelRun(userID uint, runID uint) (map[string]interface{}, error) {
	run, err := svc.dao.FindRun(userID, runID)
	if err != nil {
		return nil, err
	}
	if !isCancellableRunStatus(run.Status) {
		return map[string]interface{}{
			"agent_run": run,
			"cancelled": run.Status == domain.RunStatusCancelled,
		}, nil
	}
	if err := svc.dao.UpdateRun(run.ID, map[string]interface{}{
		"status":        domain.RunStatusCancelled,
		"cancelled_at":  int(time.Now().Unix()),
		"error_message": "user cancelled run",
	}); err != nil {
		return nil, err
	}
	run, err = svc.dao.FindRun(userID, runID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"agent_run": run,
		"cancelled": true,
	}, nil
}

// ResumeRun records a clarification answer and requeues the same waiting run.
func (svc *Service) ResumeRun(
	ctx context.Context,
	userID uint,
	runID uint,
	request ResumeRunRequest,
) (map[string]interface{}, error) {
	answer := strings.TrimSpace(request.Content)
	if answer == "" {
		return nil, errors.New("clarification answer is required")
	}
	run, err := svc.dao.FindRun(userID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != domain.RunStatusWaiting {
		return nil, fmt.Errorf("run %d is not waiting for user input", run.ID)
	}
	state := mergeClarificationAnswer(queuedRunState(run), answer, int(time.Now().Unix()))
	message := model.Message{
		ConversationID: run.ConversationID,
		UserID:         userID,
		Role:           "user",
		InputType:      "answer_to_questions",
		Content:        answer,
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&message); err != nil {
		return nil, err
	}
	if err := svc.dao.UpdateRun(run.ID, map[string]interface{}{
		"status":        domain.RunStatusQueued,
		"state_json":    mustJSON(state),
		"error_message": "",
		"completed_at":  0,
		"cancelled_at":  0,
	}); err != nil {
		return nil, err
	}
	if svc.runQueue == nil {
		err := errors.New("agent v2 run queue is not configured")
		_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
			"status":        domain.RunStatusFailed,
			"error_message": err.Error(),
		})
		return nil, err
	}
	if err := svc.runQueue.EnqueueAgentRun(ctx, AgentRunQueuePayload{
		RunID:          run.ID,
		UserID:         userID,
		ConversationID: run.ConversationID,
	}); err != nil {
		_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
			"status":        domain.RunStatusFailed,
			"error_message": err.Error(),
		})
		return nil, err
	}
	run, _ = svc.dao.FindRun(userID, run.ID)
	steps, _ := svc.dao.ListSteps(userID, run.ID)
	ledgerItems, _ := svc.dao.ListTaskLedgerItems(run.ID)
	toolInvocations, _ := svc.dao.ListToolInvocationsByRun(userID, run.ID)
	return map[string]interface{}{
		"agent_run":         run,
		"user_message":      message,
		"steps":             steps,
		"task_ledger_items": ledgerItems,
		"tool_invocations":  toolInvocations,
		"state":             state,
		"queued":            true,
	}, nil
}

// ListRunEvents assembles a stable cursor-based event list from steps, ledger, and tool invocations.
func (svc *Service) ListRunEvents(userID uint, runID uint, cursor int) (RunEventsResponse, error) {
	if _, err := svc.dao.FindRun(userID, runID); err != nil {
		return RunEventsResponse{}, err
	}
	steps, err := svc.dao.ListSteps(userID, runID)
	if err != nil {
		return RunEventsResponse{}, err
	}
	ledgerItems, err := svc.dao.ListTaskLedgerItems(runID)
	if err != nil {
		return RunEventsResponse{}, err
	}
	toolInvocations, err := svc.dao.ListToolInvocationsByRun(userID, runID)
	if err != nil {
		return RunEventsResponse{}, err
	}
	events := buildRunEvents(steps, ledgerItems, toolInvocations)
	filtered := make([]RunEvent, 0, len(events))
	for _, event := range events {
		if event.Cursor > cursor {
			filtered = append(filtered, event)
		}
	}
	nextCursor := cursor
	if len(events) > 0 {
		nextCursor = events[len(events)-1].Cursor
	}
	return RunEventsResponse{Events: filtered, Cursor: nextCursor}, nil
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
		Kind:           request.Kind,
		Limit:          limit,
		MarkUsed:       request.MarkUsed,
	})
}

// UpdateMemory edits or disables a V2 memory owned by the user.
func (svc *Service) UpdateMemory(userID uint, memoryID uint, request UpdateMemoryRequest) (model.ContextMemory, error) {
	return svc.memories.Update(memorysvc.UpdateMemoryInput{
		UserID:     userID,
		MemoryID:   memoryID,
		Content:    request.Content,
		Confidence: request.Confidence,
		Disabled:   request.Disabled,
	})
}

// DeleteMemory 删除当前用户的一条 V2 记忆。
func (svc *Service) DeleteMemory(userID uint, memoryID uint) error {
	return svc.memories.Delete(userID, memoryID)
}

// PromoteMemoryProposal confirms a draft memory proposal as stable memory.
func (svc *Service) PromoteMemoryProposal(
	userID uint,
	memoryID uint,
	request PromoteMemoryRequest,
) (model.ContextMemory, bool, error) {
	return svc.memories.PromoteProposal(memorysvc.PromoteProposalInput{
		UserID:     userID,
		MemoryID:   memoryID,
		Confidence: request.Confidence,
	})
}

// SelectArtifact 选择当前用户有权访问的候选产物。
func (svc *Service) EvolutionSummary(request EvolutionQueryRequest) ([]evalsvc.FailureSummary, error) {
	return svc.evolution.FailureSummary(request.AgentName, request.Limit)
}

func (svc *Service) DraftPromptVersion(request DraftPromptVersionRequest) (model.AgentPromptVersion, error) {
	return svc.evolution.DraftPromptVersion(evalsvc.DraftPromptInput{AgentName: request.AgentName, Limit: request.Limit})
}

func (svc *Service) ListPromptVersions(request EvolutionQueryRequest) ([]model.AgentPromptVersion, error) {
	return svc.evolution.ListPromptVersions(request.AgentName, request.Limit)
}

func (svc *Service) MovePromptVersionToReview(versionID uint) (model.AgentPromptVersion, error) {
	return svc.evolution.MovePromptVersionToReview(versionID)
}

func (svc *Service) ActivatePromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	return svc.evolution.ActivatePromptVersion(versionID)
}

func (svc *Service) ArchivePromptVersion(versionID uint) (model.AgentPromptVersion, error) {
	return svc.evolution.ArchivePromptVersion(versionID)
}

func (svc *Service) CreateEvalCase(request EvalCaseRequest) (model.EvalCase, error) {
	return svc.evolution.CreateEvalCase(evalsvc.EvalCaseInput{
		AgentName:    request.AgentName,
		Name:         request.Name,
		InputJSON:    request.InputJSON,
		ExpectedJSON: request.ExpectedJSON,
		TagsJSON:     request.TagsJSON,
		Weight:       request.Weight,
	})
}

func (svc *Service) ListEvalCases(request EvolutionQueryRequest) ([]model.EvalCase, error) {
	return svc.evolution.ListEvalCases(request.AgentName, request.Limit)
}

func (svc *Service) CreateEvalRun(request EvalRunRequest) (model.EvalRun, error) {
	return svc.evolution.CreateEvalRun(evalsvc.EvalRunInput{
		EvalCaseID:      request.EvalCaseID,
		PromptVersionID: request.PromptVersionID,
		AgentName:       request.AgentName,
		Status:          request.Status,
		Score:           request.Score,
		MetricsJSON:     request.MetricsJSON,
		ErrorMessage:    request.ErrorMessage,
	})
}

func (svc *Service) ListEvalRuns(request EvolutionQueryRequest) ([]model.EvalRun, error) {
	return svc.evolution.ListEvalRuns(request.AgentName, request.Limit)
}

func (svc *Service) SelectArtifact(userID uint, artifactID uint, request SelectArtifactRequest) error {
	artifact, err := svc.artifacts.AuthorizeDownload(userID, artifactID)
	if err != nil {
		return err
	}
	if err := svc.artifacts.SelectArtifact(artifactsvc.SelectArtifactInput{
		UserID:            userID,
		ArtifactID:        artifactID,
		ArtifactVersionID: request.ArtifactVersionID,
	}); err != nil {
		return err
	}
	_, _, err = svc.memories.ProposeFromArtifactFeedback(memorysvc.ArtifactFeedbackProposalInput{
		UserID:            userID,
		ConversationID:    artifact.ConversationID,
		AgentRunID:        artifact.AgentRunID,
		ArtifactID:        artifactID,
		ArtifactVersionID: request.ArtifactVersionID,
		FeedbackType:      artifactsvc.FeedbackTypeSelected,
	})
	return err
}

// ListArtifacts returns V2 artifacts for a conversation after ownership validation.
func (svc *Service) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	if _, err := svc.dao.FindConversation(userID, conversationID); err != nil {
		return nil, err
	}
	artifacts, err := svc.artifacts.ListArtifacts(userID, conversationID)
	if err != nil {
		return nil, err
	}
	return publicArtifacts(artifacts), nil
}

// UploadArtifact validates and stores a user uploaded image as artifact version v1.
func (svc *Service) UploadArtifact(ctx context.Context, userID uint, input UploadArtifactInput) (model.Artifact, model.ArtifactVersion, error) {
	if userID == 0 {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("upload user_id is required")
	}
	conversation, err := svc.dao.FindConversation(userID, input.ConversationID)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	metadata, err := validateImageUpload(input.Content, input.FileName, input.ContentType)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	name := safeUploadName(input.FileName, metadata.MimeType)
	objectKey := uploadObjectKey(userID, conversation.ID, name)
	stored, err := agent_svc.NewObjectStore().Save(objectKey, input.Content)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	if err := svc.checkImageSafety(ctx, userID, stored.ObjectKey); err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	params, _ := json.Marshal(map[string]interface{}{
		"source":       "upload",
		"width":        metadata.Width,
		"height":       metadata.Height,
		"content_type": metadata.MimeType,
	})
	return svc.artifacts.CreateArtifactWithVersion(artifactsvc.CreateArtifactWithVersionInput{
		Artifact: model.Artifact{
			UserID:         userID,
			ConversationID: conversation.ID,
			Name:           name,
			Kind:           "image",
			MimeType:       metadata.MimeType,
			ObjectKey:      stored.ObjectKey,
			PreviewURL:     stored.PreviewURL,
			SizeBytes:      stored.SizeBytes,
			Hash:           stored.Hash,
			Visibility:     "private",
			StoragePolicy:  "local_private",
		},
		Version: model.ArtifactVersion{
			VersionNo:        1,
			Operation:        "upload",
			Prompt:           "user uploaded image",
			GenerationParams: string(params),
			ObjectKey:        stored.ObjectKey,
			PreviewURL:       stored.PreviewURL,
			Hash:             stored.Hash,
		},
	})
}

// ListArtifactVersions returns all versions of an artifact owned by the user.
func (svc *Service) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	versions, err := svc.artifacts.ListVersions(userID, artifactID)
	if err != nil {
		return nil, err
	}
	return publicArtifactVersions(versions), nil
}

// EditArtifact runs the image edit provider and appends the result to the artifact version chain.
func (svc *Service) EditArtifact(ctx context.Context, userID uint, artifactID uint, request EditArtifactRequest) (model.ArtifactVersion, error) {
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return model.ArtifactVersion{}, errors.New("edit prompt is required")
	}
	artifact, err := svc.artifacts.AuthorizeDownload(userID, artifactID)
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	if err := svc.checkTextSafety(ctx, userID, prompt); err != nil {
		return model.ArtifactVersion{}, err
	}
	versions, err := svc.artifacts.ListVersions(userID, artifactID)
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	parent, err := selectParentVersion(versions, request.ArtifactVersionID)
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	imageConfig, err := svc.resolveRuntimeModelConfig(userID, "image", request.ImageModelConfigID)
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	registry := tools.NewRegistry()
	adapter := tools.NewLegacyProviderAdapter(imageConfig.Config)
	if err := registry.Register(tools.InstrumentTool(tools.Tool{
		Name:          runtimeImageModelName(imageConfig.Config) + "-edit",
		Kind:          tools.KindImageEdit,
		Provider:      imageConfig.Config.Provider,
		Model:         runtimeImageModelName(imageConfig.Config),
		ModelConfigID: imageConfig.GlobalID,
		Capability: tools.Capability{
			MaxPromptChars:     8000,
			SupportsImageInput: true,
			SupportsMask:       false,
			MaxCandidates:      1,
			CostPolicy:         "real_provider",
		},
		ImageEditProvider: adapter,
	}, svc.dao)); err != nil {
		return model.ArtifactVersion{}, err
	}
	tool, err := registry.FindTool(tools.FindToolRequest{Kind: tools.KindImageEdit, UserID: userID, ModelConfigID: imageConfig.GlobalID})
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	result, err := tool.ImageEditProvider.EditImage(ctx, tools.ImageEditRequest{
		UserID:         userID,
		ConversationID: artifact.ConversationID,
		TaskType:       "image_edit",
		Prompt:         prompt,
		ImageRefs:      []string{parent.ObjectKey},
		CandidateCount: 1,
	})
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	if len(result.Images) == 0 {
		return model.ArtifactVersion{}, errors.New("image edit provider returned no images")
	}
	image := result.Images[0]
	if err := svc.checkImageSafety(ctx, userID, image.ObjectKey); err != nil {
		return model.ArtifactVersion{}, err
	}
	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"artifact_id":       artifact.ID,
		"parent_version_id": parent.ID,
		"image_refs":        []string{parent.ObjectKey},
	})
	version, err := svc.artifacts.CreateRefinedVersion(artifactsvc.CreateRefinedVersionInput{
		UserID:          userID,
		ArtifactID:      artifactID,
		ParentVersionID: parent.ID,
		Image: model.ArtifactVersion{
			Operation:     "edit",
			Prompt:        prompt,
			ModelProvider: imageConfig.Config.Provider,
			ModelName:     runtimeImageModelName(imageConfig.Config),
			SourceRefs:    string(sourceRefs),
			ObjectKey:     image.ObjectKey,
			PreviewURL:    image.PreviewURL,
			Hash:          image.Hash,
		},
	})
	if err != nil {
		return model.ArtifactVersion{}, err
	}
	return publicArtifactVersions([]model.ArtifactVersion{version})[0], nil
}

// RenderArtifactText creates an SVG text-layer artifact linked to an owned image artifact.
func (svc *Service) RenderArtifactText(userID uint, artifactID uint, request RenderTextRequest) (model.Artifact, model.ArtifactVersion, error) {
	title := strings.TrimSpace(request.Title)
	if title == "" {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("render title is required")
	}
	artifact, err := svc.artifacts.AuthorizeDownload(userID, artifactID)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	if !strings.EqualFold(artifact.Kind, "image") {
		return model.Artifact{}, model.ArtifactVersion{}, errors.New("text rendering requires an image artifact")
	}
	versions, err := svc.artifacts.ListVersions(userID, artifactID)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	parent, err := selectParentVersion(versions, request.ArtifactVersionID)
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	layers := []renderTextLayer{{Text: title, Role: "title", Y: 170, Size: 58, Weight: "700"}}
	if subtitle := strings.TrimSpace(request.Subtitle); subtitle != "" {
		layers = append(layers, renderTextLayer{Text: subtitle, Role: "subtitle", Y: 245, Size: 28, Weight: "500"})
	}
	if brand := strings.TrimSpace(request.Brand); brand != "" {
		layers = append(layers, renderTextLayer{Text: brand, Role: "brand", Y: 860, Size: 24, Weight: "600"})
	}
	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"background_artifact_id": artifact.ID,
		"background_version_id":  parent.ID,
		"text_layers":            layers,
	})
	rendered, version, err := svc.artifacts.CreateRenderedArtifact(artifactsvc.CreateRenderedArtifactInput{
		UserID:           userID,
		ConversationID:   artifact.ConversationID,
		AgentRunID:       artifact.AgentRunID,
		ParentArtifactID: artifact.ID,
		ParentVersionID:  parent.ID,
		ArtifactGroupID:  artifact.ArtifactGroupID,
		Name:             "poster-text-layer.svg",
		Kind:             "svg",
		MimeType:         "image/svg+xml",
		Content:          []byte(renderTextLayerSVG(artifact.PreviewURL, layers)),
		Operation:        "render_text",
		Prompt:           strings.Join(renderLayerTexts(layers), "\n"),
		ModelProvider:    "layout_renderer",
		ModelName:        "svg_text_layer_v1",
		SourceRefs:       string(sourceRefs),
	})
	if err != nil {
		return model.Artifact{}, model.ArtifactVersion{}, err
	}
	return publicArtifacts([]model.Artifact{rendered})[0], publicArtifactVersions([]model.ArtifactVersion{version})[0], nil
}

// DownloadArtifact resolves an owned artifact to a local file path.
func (svc *Service) DownloadArtifact(userID uint, artifactID uint) (model.Artifact, string, error) {
	artifact, err := svc.artifacts.AuthorizeDownload(userID, artifactID)
	if err != nil {
		return artifact, "", err
	}
	return artifact, agent_svc.NewObjectStore().Path(artifact.ObjectKey), nil
}

// PreviewArtifact resolves an owned artifact to a local file path for inline preview.
func (svc *Service) PreviewArtifact(userID uint, artifactID uint) (model.Artifact, string, error) {
	return svc.DownloadArtifact(userID, artifactID)
}

// RecordArtifactFeedback writes feedback after validating artifact ownership.
func (svc *Service) RecordArtifactFeedback(
	userID uint,
	artifactID uint,
	request ArtifactFeedbackRequest,
) error {
	artifact, err := svc.artifacts.AuthorizeDownload(userID, artifactID)
	if err != nil {
		return err
	}
	feedbackType := strings.TrimSpace(request.FeedbackType)
	comment := strings.TrimSpace(request.Comment)
	if err := svc.artifacts.RecordFeedback(model.ArtifactFeedback{
		ArtifactID:        artifactID,
		ArtifactVersionID: request.ArtifactVersionID,
		UserID:            userID,
		FeedbackType:      feedbackType,
		Rating:            request.Rating,
		Comment:           comment,
	}); err != nil {
		return err
	}
	_, _, err = svc.memories.ProposeFromArtifactFeedback(memorysvc.ArtifactFeedbackProposalInput{
		UserID:            userID,
		ConversationID:    artifact.ConversationID,
		AgentRunID:        artifact.AgentRunID,
		ArtifactID:        artifactID,
		ArtifactVersionID: request.ArtifactVersionID,
		FeedbackType:      feedbackType,
		Rating:            request.Rating,
		Comment:           comment,
	})
	return err
}

func (svc *Service) recordRunReviewScores(userID uint, state domain.RunState) error {
	if len(state.Artifacts) == 0 {
		return nil
	}
	reviews := candidateReviewsForPersistence(state)
	for _, review := range reviews {
		if review.ArtifactID == 0 || review.VersionID == 0 {
			continue
		}
		if err := svc.artifacts.RecordReviewScores(artifactsvc.ReviewScoresInput{
			UserID:           userID,
			ArtifactID:       review.ArtifactID,
			VersionID:        review.VersionID,
			OverallScore:     review.OverallScore,
			RequirementMatch: review.RequirementMatch,
			CompositionScore: review.CompositionScore,
			TextReadability:  review.TextReadability,
			LayoutScore:      review.LayoutScore,
			RankScore:        review.RankScore,
			Issues:           review.Issues,
			ShouldRefine:     review.ShouldRefine,
			Reviewer:         coalesce(review.Reviewer, coalesce(state.Review.Reviewer, "mock_vision_review")),
			ExtractedText:    review.ExtractedText,
		}); err != nil {
			return err
		}
		if _, _, err := svc.memories.ProposeFromReview(memorysvc.ReviewProposalInput{
			UserID:            userID,
			ConversationID:    state.ConversationID,
			AgentRunID:        state.RunID,
			ArtifactID:        review.ArtifactID,
			ArtifactVersionID: review.VersionID,
			OverallScore:      review.OverallScore,
			Issues:            review.Issues,
			ShouldRefine:      review.ShouldRefine,
			Reviewer:          coalesce(review.Reviewer, state.Review.Reviewer),
			MinScore:          minReviewMemoryScore,
		}); err != nil {
			return err
		}
	}
	return nil
}

func candidateReviewsForPersistence(state domain.RunState) []domain.CandidateReview {
	if len(state.Review.CandidateReviews) > 0 {
		return state.Review.CandidateReviews
	}
	reviews := make([]domain.CandidateReview, 0, len(state.Artifacts))
	for _, artifact := range state.Artifacts {
		reviews = append(reviews, domain.CandidateReview{
			ArtifactID:       artifact.ID,
			VersionID:        artifact.VersionID,
			ImageRef:         artifact.PreviewURL,
			OverallScore:     state.Review.OverallScore,
			RequirementMatch: state.Review.RequirementMatch,
			CompositionScore: state.Review.CompositionScore,
			TextReadability:  state.Review.TextReadability,
			LayoutScore:      state.Review.LayoutScore,
			Issues:           append([]string{}, state.Review.Issues...),
			ShouldRefine:     state.Review.ShouldRefine,
			Reviewer:         state.Review.Reviewer,
		})
	}
	return reviews
}

func (svc *Service) idempotentRunResponse(conversation model.Conversation, userID uint, run model.AgentRun) (map[string]interface{}, error) {
	steps, err := svc.dao.ListSteps(userID, run.ID)
	if err != nil {
		return nil, err
	}
	ledgerItems, err := svc.dao.ListTaskLedgerItems(run.ID)
	if err != nil {
		return nil, err
	}
	toolInvocations, err := svc.dao.ListToolInvocationsByRun(userID, run.ID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"conversation":      conversation,
		"agent_run":         run,
		"steps":             steps,
		"task_ledger_items": ledgerItems,
		"tool_invocations":  toolInvocations,
		"idempotent":        true,
	}, nil
}

// FailTimedOutRuns marks stale running runs failed so queue retries or operators can see a terminal state.
func (svc *Service) FailTimedOutRuns(timeoutSeconds int) (int64, error) {
	if timeoutSeconds <= 0 {
		return 0, nil
	}
	cutoffUnix := int(time.Now().Add(-time.Duration(timeoutSeconds) * time.Second).Unix())
	return svc.dao.MarkTimedOutRunningRuns(
		cutoffUnix,
		fmt.Sprintf("agent v2 run timed out after %d seconds", timeoutSeconds),
	)
}

func buildRunEvents(steps []model.AgentStep, ledgerItems []model.TaskLedgerItem, toolInvocations []model.ToolInvocation) []RunEvent {
	events := make([]RunEvent, 0, len(steps)+len(ledgerItems)+len(toolInvocations))
	for index := range steps {
		step := steps[index]
		events = append(events, RunEvent{
			Type:      "agent_step",
			ID:        step.ID,
			CreatedAt: step.CreatedAt,
			Step:      &step,
		})
	}
	for index := range ledgerItems {
		item := ledgerItems[index]
		events = append(events, RunEvent{
			Type:       "task_ledger_item",
			ID:         item.ID,
			CreatedAt:  item.CreatedAt,
			LedgerItem: &item,
		})
	}
	for index := range toolInvocations {
		invocation := toolInvocations[index]
		events = append(events, RunEvent{
			Type:           "tool_invocation",
			ID:             invocation.ID,
			CreatedAt:      invocation.CreatedAt,
			ToolInvocation: &invocation,
		})
	}
	sort.SliceStable(events, func(i int, j int) bool {
		if events[i].CreatedAt != events[j].CreatedAt {
			return events[i].CreatedAt < events[j].CreatedAt
		}
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].ID < events[j].ID
	})
	for index := range events {
		events[index].Cursor = index + 1
	}
	return events
}

func publicArtifacts(artifacts []model.Artifact) []model.Artifact {
	result := make([]model.Artifact, len(artifacts))
	copy(result, artifacts)
	for index := range result {
		result[index].ObjectKey = ""
		if result[index].ID != 0 {
			result[index].PreviewURL = fmt.Sprintf("/api/v2/artifacts/%d/preview", result[index].ID)
		}
	}
	return result
}

func mergeClarificationAnswer(state domain.RunState, answer string, createdAt int) domain.RunState {
	answer = strings.TrimSpace(answer)
	questions := append([]string{}, state.Requirements.Questions...)
	state.Clarifications = append(state.Clarifications, domain.ClarificationTurn{
		Questions: questions,
		Answer:    answer,
		CreatedAt: createdAt,
	})
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	state.Metadata["latest_clarification_answer"] = answer
	state.Requirements.NeedClarification = false
	state.Requirements.Questions = []string{}
	return state
}

func appendClarificationToRequest(userRequest string, questions []string, answer string) string {
	parts := []string{strings.TrimSpace(userRequest), "补充信息："}
	for _, question := range questions {
		question = strings.TrimSpace(question)
		if question != "" {
			parts = append(parts, "- "+question)
		}
	}
	answer = strings.TrimSpace(answer)
	if answer != "" {
		parts = append(parts, "回答："+answer)
	}
	return strings.Join(nonEmptyStrings(parts), "\n")
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func publicArtifactVersions(versions []model.ArtifactVersion) []model.ArtifactVersion {
	result := make([]model.ArtifactVersion, len(versions))
	copy(result, versions)
	for index := range result {
		result[index].ObjectKey = ""
		result[index].PreviewURL = ""
	}
	return result
}

type imageUploadMetadata struct {
	MimeType string
	Width    int
	Height   int
}

func validateImageUpload(content []byte, fileName string, contentType string) (imageUploadMetadata, error) {
	if len(content) == 0 {
		return imageUploadMetadata{}, errors.New("upload image is empty")
	}
	if int64(len(content)) > MaxImageUploadBytes {
		return imageUploadMetadata{}, fmt.Errorf("upload image exceeds %d bytes", MaxImageUploadBytes)
	}
	detected := http.DetectContentType(content)
	if !allowedImageMime(detected) {
		return imageUploadMetadata{}, fmt.Errorf("unsupported image mime %q", detected)
	}
	if contentType = strings.TrimSpace(strings.Split(contentType, ";")[0]); contentType != "" && !strings.EqualFold(contentType, detected) {
		return imageUploadMetadata{}, fmt.Errorf("upload mime mismatch: header %q detected %q", contentType, detected)
	}
	if err := agentsecurity.ValidateImageExtension(fileName, detected); err != nil {
		return imageUploadMetadata{}, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return imageUploadMetadata{}, fmt.Errorf("decode upload image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return imageUploadMetadata{}, errors.New("upload image dimensions are invalid")
	}
	if config.Width*config.Height > maxImageUploadPixels {
		return imageUploadMetadata{}, fmt.Errorf("upload image pixels exceed %d", maxImageUploadPixels)
	}
	return imageUploadMetadata{MimeType: detected, Width: config.Width, Height: config.Height}, nil
}

func allowedImageMime(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/gif":
		return true
	default:
		return false
	}
}

func safeUploadName(name string, mimeType string) string {
	name = agent_svc.SafeDownloadName(strings.TrimSpace(name))
	if name == "" || name == "." {
		name = "uploaded-image" + extensionForMime(mimeType)
	}
	if filepath.Ext(name) == "" {
		name += extensionForMime(mimeType)
	}
	return name
}

func extensionForMime(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func uploadObjectKey(userID uint, conversationID uint, name string) string {
	return path.Join(
		fmt.Sprintf("user-%d", userID),
		fmt.Sprintf("conversation-%d", conversationID),
		"uploads",
		agentsecurity.RandomObjectKeyPart(),
		name,
	)
}

func selectParentVersion(versions []model.ArtifactVersion, requestedID uint) (model.ArtifactVersion, error) {
	if len(versions) == 0 {
		return model.ArtifactVersion{}, errors.New("artifact has no versions")
	}
	if requestedID > 0 {
		for _, version := range versions {
			if version.ID == requestedID {
				return version, nil
			}
		}
		return model.ArtifactVersion{}, errors.New("requested parent version was not found")
	}
	return versions[len(versions)-1], nil
}

type renderTextLayer struct {
	Text   string `json:"text"`
	Role   string `json:"role"`
	Y      int    `json:"y"`
	Size   int    `json:"size"`
	Weight string `json:"weight"`
}

func renderTextLayerSVG(backgroundRef string, layers []renderTextLayer) string {
	builder := strings.Builder{}
	builder.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1080" height="1350" viewBox="0 0 1080 1350">`)
	builder.WriteString(`<defs><linearGradient id="poster-bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="#1f2933"/><stop offset="58%" stop-color="#526d7a"/><stop offset="100%" stop-color="#e9c46a"/></linearGradient></defs>`)
	builder.WriteString(`<rect width="100%" height="100%" fill="url(#poster-bg)"/>`)
	if strings.TrimSpace(backgroundRef) != "" {
		builder.WriteString(fmt.Sprintf(`<image href="%s" x="0" y="0" width="1080" height="1350" preserveAspectRatio="xMidYMid slice"/>`, html.EscapeString(backgroundRef)))
	}
	builder.WriteString(`<rect x="44" y="88" width="780" height="240" rx="18" fill="#000000" opacity="0.32"/>`)
	for _, layer := range layers {
		builder.WriteString(fmt.Sprintf(
			`<text x="78" y="%d" fill="#fffaf0" font-family="Arial, 'Microsoft YaHei', sans-serif" font-size="%d" font-weight="%s">%s</text>`,
			layer.Y,
			layer.Size,
			html.EscapeString(layer.Weight),
			html.EscapeString(layer.Text),
		))
	}
	builder.WriteString(`</svg>`)
	return builder.String()
}

func renderLayerTexts(layers []renderTextLayer) []string {
	values := make([]string, 0, len(layers))
	for _, layer := range layers {
		if strings.TrimSpace(layer.Text) != "" {
			values = append(values, strings.TrimSpace(layer.Text))
		}
	}
	return values
}

func memoryItemsFromModel(memories []model.ContextMemory) []domain.MemoryItem {
	result := make([]domain.MemoryItem, 0, len(memories))
	for _, memory := range memories {
		if strings.TrimSpace(memory.Content) == "" {
			continue
		}
		result = append(result, domain.MemoryItem{
			ID:         memory.ID,
			Kind:       memory.Kind,
			Content:    memory.Content,
			Confidence: memory.Confidence,
		})
	}
	return result
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

func idempotencyKeyUniqueValue(value string) *string {
	value = normalizeIdempotencyKey(value)
	if value == "" {
		return nil
	}
	return &value
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "constraint failed") ||
		strings.Contains(message, "error 1062")
}

func normalizeCandidateCount(value int) int {
	if value <= 0 {
		return 1
	}
	if value > 3 {
		return 3
	}
	return value
}

func isCancellableRunStatus(status string) bool {
	switch status {
	case domain.RunStatusCreated, domain.RunStatusQueued, domain.RunStatusRunning, domain.RunStatusWaiting:
		return true
	default:
		return false
	}
}

func uintToString(value uint) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(value), 10)
}

// mustJSON 将对象序列化为 JSON，序列化失败时返回空对象字符串。
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
