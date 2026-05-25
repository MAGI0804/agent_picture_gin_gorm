package agents

import (
	"context"

	"gin-biz-web-api/internal/service/agent_v2/domain"
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
