package agents

import (
	"context"
	"errors"
	"strings"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

// MockVisionReviewAgent provides a deterministic first-pass review node.
type MockVisionReviewAgent struct {
	minPassingScore float64
}

// NewMockVisionReviewAgent creates a mock review agent with a pass threshold.
func NewMockVisionReviewAgent(minPassingScore float64) *MockVisionReviewAgent {
	return &MockVisionReviewAgent{minPassingScore: minPassingScore}
}

// Key returns the workflow node key.
func (agent *MockVisionReviewAgent) Key() string {
	return "vision_review_agent"
}

// Run scores existing artifacts without calling an external VLM.
func (agent *MockVisionReviewAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	score := 0.82
	issues := []string{}
	reflectionDraft := ""

	if len(state.Artifacts) == 0 {
		score = 0.30
		issues = append(issues, "no artifact generated")
		reflectionDraft = "Image generation produced no artifact; check provider output and artifact persistence."
	}
	if state.Requirements.NeedClarification {
		score -= 0.15
		issues = append(issues, "requirements still need clarification")
	}

	shouldRefine := score < agent.minPassingScore
	output := map[string]interface{}{
		"overall_score":    score,
		"issues":           issues,
		"should_refine":    shouldRefine,
		"reflection_draft": reflectionDraft,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "mock vision review completed",
		Output:  output,
	}, nil
}

// VisionReviewAgent calls a registered VLM/OCR-capable provider for real image review.
type VisionReviewAgent struct {
	registry *tools.Registry
	options  VisionReviewAgentOptions
}

// VisionReviewAgentOptions configures real vision review.
type VisionReviewAgentOptions struct {
	VisionModelConfigID uint
	MinPassingScore     float64
}

// NewVisionReviewAgent creates a real provider-backed review agent.
func NewVisionReviewAgent(registry *tools.Registry, options VisionReviewAgentOptions) *VisionReviewAgent {
	if options.MinPassingScore <= 0 {
		options.MinPassingScore = 0.7
	}
	return &VisionReviewAgent{registry: registry, options: options}
}

// Key returns the workflow node key.
func (agent *VisionReviewAgent) Key() string {
	return "vision_review_agent"
}

// Run reviews the first generated image with a registered VisionProvider.
func (agent *VisionReviewAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("tool registry is required")
	}
	imageRef := firstGeneratedImageRef(state)
	if imageRef == "" {
		return domain.StepResult{
			Status:  domain.StepStatusCompleted,
			Summary: "real vision review found no generated image",
			Output: map[string]interface{}{
				"overall_score":    0.30,
				"issues":           []string{"no generated image available for vision review"},
				"should_refine":    true,
				"reflection_draft": "Image generation produced no artifact; check provider output and artifact persistence.",
				"reviewer":         "real_vision_review",
			},
		}, nil
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindVision,
		UserID:        state.UserID,
		ModelConfigID: agent.options.VisionModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	result, err := tool.VisionProvider.AnalyzeImage(ctx, tools.VisionRequest{
		UserID:   state.UserID,
		RunID:    state.RunID,
		StepID:   state.CurrentStepID,
		ImageRef: imageRef,
		Prompt:   visionReviewPrompt(state),
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	score := result.Scores["overall"]
	if score == 0 {
		score = result.Scores["overall_score"]
	}
	if score == 0 {
		score = 0.75
	}
	shouldRefine := result.ShouldRefine || score < agent.options.MinPassingScore
	output := map[string]interface{}{
		"overall_score": score,
		"issues":        result.Issues,
		"should_refine": shouldRefine,
		"summary":       strings.TrimSpace(result.Summary),
		"reviewer":      "real_vision_review",
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "real vision review completed",
		Output:  output,
	}, nil
}

func firstGeneratedImageRef(state domain.RunState) string {
	for _, image := range state.GeneratedImages {
		if strings.TrimSpace(image.ObjectKey) != "" {
			return strings.TrimSpace(image.ObjectKey)
		}
		if strings.TrimSpace(image.PreviewURL) != "" {
			return strings.TrimSpace(image.PreviewURL)
		}
	}
	for _, artifact := range state.Artifacts {
		if strings.TrimSpace(artifact.PreviewURL) != "" {
			return strings.TrimSpace(artifact.PreviewURL)
		}
	}
	return ""
}

func visionReviewPrompt(state domain.RunState) string {
	return strings.TrimSpace("Review this generated image against the user request. " +
		"Return JSON with keys summary, overall_score, issues, should_refine. " +
		"User request: " + state.UserRequest)
}
