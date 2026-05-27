package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"
)

// ExecuteQueuedRun is the durable worker entrypoint for an already-persisted async run.
func (svc *Service) ExecuteQueuedRun(ctx context.Context, payload AgentRunQueuePayload) error {
	if payload.RunID == 0 {
		return errors.New("queued run run_id is required")
	}
	if payload.UserID == 0 {
		return errors.New("queued run user_id is required")
	}
	if payload.ConversationID == 0 {
		return errors.New("queued run conversation_id is required")
	}

	run, err := svc.dao.FindRun(payload.UserID, payload.RunID)
	if err != nil {
		return err
	}
	if run.ConversationID != payload.ConversationID {
		return fmt.Errorf("queued run conversation mismatch: run %d belongs to conversation %d, payload conversation %d", run.ID, run.ConversationID, payload.ConversationID)
	}
	if !isExecutableQueuedRunStatus(run.Status) {
		return nil
	}
	claimed, err := svc.dao.ClaimRunStatus(payload.UserID, payload.RunID, run.Status, domain.RunStatusRunning)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	run.Status = domain.RunStatusRunning

	conversation, err := svc.dao.FindConversation(payload.UserID, payload.ConversationID)
	if err != nil {
		_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
			"status":        domain.RunStatusFailed,
			"error_message": err.Error(),
		})
		return err
	}
	state := queuedRunState(run)
	flow, err := svc.workflowForQueuedRun(payload.UserID, state)
	if err != nil {
		_ = svc.dao.UpdateRun(run.ID, map[string]interface{}{
			"status":        domain.RunStatusFailed,
			"error_message": err.Error(),
		})
		return err
	}
	_, _, err = svc.executePreparedRun(ctx, payload.UserID, conversation, run, state, flow)
	return err
}

func queuedRunState(run model.AgentRun) domain.RunState {
	var state domain.RunState
	if strings.TrimSpace(run.StateJSON) != "" {
		_ = json.Unmarshal([]byte(run.StateJSON), &state)
	}
	state.RunID = run.ID
	state.UserID = run.UserID
	state.ConversationID = run.ConversationID
	if state.TaskType == "" {
		state.TaskType = coalesce(run.TaskType, "image_generation")
	}
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	if state.Budget.MaxSteps == 0 && strings.TrimSpace(run.BudgetJSON) != "" {
		var budget domain.RunBudget
		if err := json.Unmarshal([]byte(run.BudgetJSON), &budget); err == nil {
			state.Budget = budget
		}
	}
	if state.Budget.MaxSteps == 0 {
		state.Budget.MaxSteps = 12
	}
	if state.Budget.MaxImageGenerations == 0 {
		state.Budget.MaxImageGenerations = 1
	}
	if state.Budget.TimeoutSeconds == 0 {
		state.Budget.TimeoutSeconds = 180
	}
	return state
}

func (svc *Service) workflowForQueuedRun(userID uint, state domain.RunState) (workflow.Workflow, error) {
	imageModelConfigID := metadataUint(state.Metadata, "image_model_config_id")
	imageConfig, err := svc.resolveRuntimeModelConfig(userID, "image", imageModelConfigID)
	if err != nil {
		return workflow.Workflow{}, err
	}

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
		return workflow.Workflow{}, err
	}

	textModelConfigID := metadataUint(state.Metadata, "text_model_config_id")
	if textModelConfigID > 0 {
		if textConfig, err := svc.resolveRuntimeModelConfig(userID, "text", textModelConfigID); err == nil {
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
	}

	var visionModelConfigID uint
	visionConfigID := metadataUint(state.Metadata, "vision_model_config_id")
	if visionConfigID > 0 {
		if visionConfig, err := svc.resolveRuntimeModelConfig(userID, "text", visionConfigID); err == nil {
			visionModelConfigID = visionConfig.GlobalID
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
	} else if visionConfig, err := svc.resolveVisionRuntimeModelConfig(userID); err == nil {
		visionModelConfigID = visionConfig.GlobalID
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

	return workflow.ImageGenerationWorkflow(workflow.ImageGenerationWorkflowOptions{
		Registry:            registry,
		ArtifactWriter:      svc.artifacts,
		ImageModelConfigID:  imageConfig.GlobalID,
		VisionModelConfigID: visionModelConfigID,
		CandidateCount:      normalizeCandidateCount(state.Budget.MaxImageGenerations),
		ModelProvider:       imageConfig.Config.Provider,
		ModelName:           runtimeImageModelName(imageConfig.Config),
	}), nil
}

func metadataUint(metadata map[string]string, key string) uint {
	if metadata == nil {
		return 0
	}
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

func isExecutableQueuedRunStatus(status string) bool {
	return status == domain.RunStatusQueued || status == domain.RunStatusFailed
}
