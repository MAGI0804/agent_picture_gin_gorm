package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

type SafetyPhase string

const (
	SafetyPhaseText  SafetyPhase = "text"
	SafetyPhaseImage SafetyPhase = "image"
)

// SafetyAgent checks prompt text before generation and image refs after generation.
type SafetyAgent struct {
	key      string
	phase    SafetyPhase
	registry *tools.Registry
}

func NewSafetyAgent(key string, phase SafetyPhase, registry *tools.Registry) *SafetyAgent {
	return &SafetyAgent{key: key, phase: phase, registry: registry}
}

func (agent *SafetyAgent) Key() string {
	return agent.key
}

func (agent *SafetyAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("safety tool registry is required")
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{Kind: tools.KindSafety, UserID: state.UserID})
	if err != nil {
		return domain.StepResult{}, err
	}
	switch agent.phase {
	case SafetyPhaseText:
		return agent.checkText(ctx, state, tool)
	case SafetyPhaseImage:
		return agent.checkImages(ctx, state, tool)
	default:
		return domain.StepResult{}, fmt.Errorf("unsupported safety phase %q", agent.phase)
	}
}

func (agent *SafetyAgent) checkText(ctx context.Context, state domain.RunState, tool tools.Tool) (domain.StepResult, error) {
	text := strings.TrimSpace(state.Prompts.PositivePrompt)
	if text == "" {
		text = strings.TrimSpace(state.UserRequest)
	}
	result, err := tool.SafetyProvider.CheckContent(ctx, tools.SafetyRequest{
		UserID: state.UserID,
		RunID:  state.RunID,
		StepID: state.CurrentStepID,
		Text:   text,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	if !result.Allowed {
		return domain.StepResult{}, fmt.Errorf("文本安全检查拒绝内容: %s", result.Reason)
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "文本安全检查已通过",
		Output: map[string]interface{}{
			"allowed": true,
			"phase":   string(SafetyPhaseText),
		},
	}, nil
}

func (agent *SafetyAgent) checkImages(ctx context.Context, state domain.RunState, tool tools.Tool) (domain.StepResult, error) {
	if len(state.GeneratedImages) == 0 {
		return domain.StepResult{}, errors.New("图片安全检查需要已生成的图片")
	}
	for _, image := range state.GeneratedImages {
		imageRef := strings.TrimSpace(image.ObjectKey)
		if imageRef == "" {
			imageRef = strings.TrimSpace(image.PreviewURL)
		}
		result, err := tool.SafetyProvider.CheckContent(ctx, tools.SafetyRequest{
			UserID:   state.UserID,
			RunID:    state.RunID,
			StepID:   state.CurrentStepID,
			ImageRef: imageRef,
		})
		if err != nil {
			return domain.StepResult{}, err
		}
		if !result.Allowed {
			return domain.StepResult{}, fmt.Errorf("图片安全检查拒绝内容: %s", result.Reason)
		}
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("已通过 %d 张候选图片的安全检查", len(state.GeneratedImages)),
		Output: map[string]interface{}{
			"allowed":     true,
			"phase":       string(SafetyPhaseImage),
			"image_count": len(state.GeneratedImages),
		},
	}, nil
}
