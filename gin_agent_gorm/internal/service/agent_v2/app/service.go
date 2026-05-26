package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gin-biz-web-api/internal/dao/agent_v2_dao"
	"gin-biz-web-api/internal/service/agent_svc"
	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	memorysvc "gin-biz-web-api/internal/service/agent_v2/memory"
	"gin-biz-web-api/internal/service/agent_v2/runtime"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

const (
	maxIdempotencyKeyLength = 128
	minReviewMemoryScore    = 0.70
)

// Service 是 Agent V2 的应用服务层，负责把 HTTP 请求编排成一次 Agent Run。
type Service struct {
	dao       *agent_v2_dao.AgentV2DAO
	executor  *runtime.Executor
	artifacts *artifactsvc.Service
	memories  *memorysvc.Service
}

// CreateRunRequest 是创建 Agent Run 的请求体。
type CreateRunRequest struct {
	Content            string `json:"content" form:"content" binding:"required"`
	TaskType           string `json:"task_type" form:"task_type"`
	IdempotencyKey     string `json:"idempotency_key" form:"idempotency_key"`
	TextModelConfigID  uint   `json:"text_model_config_id" form:"text_model_config_id"`
	ImageModelConfigID uint   `json:"image_model_config_id" form:"image_model_config_id"`
	CandidateCount     int    `json:"candidate_count" form:"candidate_count"`
}

// MemorySearchRequest 是 V2 记忆查询请求。
type MemorySearchRequest struct {
	ConversationID uint   `json:"conversation_id" form:"conversation_id"`
	Namespace      string `json:"namespace" form:"namespace"`
	Scope          string `json:"scope" form:"scope"`
	Limit          int    `json:"limit" form:"limit"`
	MarkUsed       bool   `json:"mark_used" form:"mark_used"`
}

// PromoteMemoryRequest confirms a draft memory proposal.
type PromoteMemoryRequest struct {
	Confidence float64 `json:"confidence" form:"confidence"`
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

	imageConfig, err := svc.resolveRuntimeModelConfig(userID, "image", request.ImageModelConfigID)
	if err != nil {
		return nil, err
	}
	textConfig, textConfigErr := svc.resolveRuntimeModelConfig(userID, "text", request.TextModelConfigID)
	visionConfig, visionConfigErr := svc.resolveVisionRuntimeModelConfig(userID)

	registry := tools.NewRegistry()
	imageAdapter := tools.NewLegacyProviderAdapter(imageConfig.Config)
	if err := registry.Register(tools.Tool{
		Name:          runtimeImageModelName(imageConfig.Config),
		Kind:          tools.KindImageGeneration,
		Provider:      imageConfig.Config.Provider,
		Model:         runtimeImageModelName(imageConfig.Config),
		ModelConfigID: imageConfig.GlobalID,
		Capability: tools.Capability{
			SupportedRatios: []string{"1:1", "4:3", "16:9", "9:16"},
			MaxCandidates:   3,
			CostPolicy:      "real_provider",
		},
		ImageGenerationProvider: imageAdapter,
	}); err != nil {
		return nil, err
	}
	if textConfigErr == nil {
		textAdapter := tools.NewLegacyProviderAdapter(textConfig.Config)
		_ = registry.Register(tools.Tool{
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
		})
	}
	if visionConfigErr == nil {
		_ = registry.Register(tools.Tool{
			Name:          runtimeTextModelName(visionConfig.Config),
			Kind:          tools.KindVision,
			Provider:      visionConfig.Config.Provider,
			Model:         runtimeTextModelName(visionConfig.Config),
			ModelConfigID: visionConfig.GlobalID,
			Capability: tools.Capability{
				SupportsImageInput: true,
				CostPolicy:         "real_provider",
			},
			VisionProvider: tools.NewGoogleVisionProvider(visionConfig.Config),
		})
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
			MaxImageGenerations: normalizeCandidateCount(request.CandidateCount),
			TimeoutSeconds:      180,
		},
		Metadata: map[string]string{
			"runtime":               "agent_v2_real_image_generation",
			"image_model_config_id": uintToString(imageConfig.GlobalID),
			"text_model_config_id":  uintToString(textConfig.GlobalID),
			"image_model_provider":  imageConfig.Config.Provider,
			"image_model_name":      runtimeImageModelName(imageConfig.Config),
		},
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
		ConversationID:   conversation.ID,
		UserID:           userID,
		TriggerMessageID: message.ID,
		Status:           domain.RunStatusCreated,
		TaskType:         state.TaskType,
		WorkflowName:     "image_generation_v2",
		WorkflowVersion:  "0.3.0",
		StateJSON:        mustJSON(state),
		BudgetJSON:       mustJSON(state.Budget),
		IdempotencyKey:   idempotencyKey,
		ImageModelName:   runtimeImageModelName(imageConfig.Config),
	}
	if textConfigErr == nil {
		run.TextModelName = runtimeTextModelName(textConfig.Config)
	}
	if err := svc.dao.CreateRun(&run); err != nil {
		return nil, err
	}
	_ = svc.dao.UpdateMessageAgentRunID(message.ID, run.ID)

	// 第五步：执行固定真实图片 workflow，调用 V2 tool registry 并写入 artifact/version。
	state.RunID = run.ID
	flow := workflow.ImageGenerationWorkflow(workflow.ImageGenerationWorkflowOptions{
		Registry:            registry,
		ArtifactWriter:      svc.artifacts,
		ImageModelConfigID:  imageConfig.GlobalID,
		VisionModelConfigID: visionConfig.GlobalID,
		CandidateCount:      normalizeCandidateCount(request.CandidateCount),
		ModelProvider:       imageConfig.Config.Provider,
		ModelName:           runtimeImageModelName(imageConfig.Config),
	})
	if async {
		if err := svc.dao.UpdateRun(run.ID, map[string]interface{}{"status": domain.RunStatusQueued}); err != nil {
			return nil, err
		}
		run.Status = domain.RunStatusQueued
		go func() {
			_, _, _ = svc.executePreparedRun(context.Background(), userID, conversation, run, state, flow)
		}()
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

// ListArtifactVersions returns all versions of an artifact owned by the user.
func (svc *Service) ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error) {
	versions, err := svc.artifacts.ListVersions(userID, artifactID)
	if err != nil {
		return nil, err
	}
	return publicArtifactVersions(versions), nil
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
	for _, artifact := range state.Artifacts {
		if artifact.ID == 0 || artifact.VersionID == 0 {
			continue
		}
		if err := svc.artifacts.RecordReviewScores(artifactsvc.ReviewScoresInput{
			UserID:       userID,
			ArtifactID:   artifact.ID,
			VersionID:    artifact.VersionID,
			OverallScore: state.Review.OverallScore,
			Issues:       state.Review.Issues,
			ShouldRefine: state.Review.ShouldRefine,
			Reviewer:     coalesce(state.Review.Reviewer, "mock_vision_review"),
		}); err != nil {
			return err
		}
		if _, _, err := svc.memories.ProposeFromReview(memorysvc.ReviewProposalInput{
			UserID:            userID,
			ConversationID:    state.ConversationID,
			AgentRunID:        state.RunID,
			ArtifactID:        artifact.ID,
			ArtifactVersionID: artifact.VersionID,
			OverallScore:      state.Review.OverallScore,
			Issues:            state.Review.Issues,
			ShouldRefine:      state.Review.ShouldRefine,
			Reviewer:          state.Review.Reviewer,
			MinScore:          minReviewMemoryScore,
		}); err != nil {
			return err
		}
	}
	return nil
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

func publicArtifactVersions(versions []model.ArtifactVersion) []model.ArtifactVersion {
	result := make([]model.ArtifactVersion, len(versions))
	copy(result, versions)
	for index := range result {
		result[index].ObjectKey = ""
		result[index].PreviewURL = ""
	}
	return result
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

func normalizeCandidateCount(value int) int {
	if value <= 0 {
		return 1
	}
	if value > 3 {
		return 3
	}
	return value
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
