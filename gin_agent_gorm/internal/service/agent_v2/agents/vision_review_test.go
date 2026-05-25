package agents

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestMockVisionReviewAgentProducesLowScoreWhenNoArtifact(t *testing.T) {
	agent := NewMockVisionReviewAgent(0.7)

	result, err := agent.Run(context.Background(), domain.RunState{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != domain.StepStatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if score := result.Output["overall_score"].(float64); score >= 0.7 {
		t.Fatalf("overall_score = %f, want below 0.7", score)
	}
	if shouldRefine := result.Output["should_refine"].(bool); !shouldRefine {
		t.Fatal("should_refine = false, want true")
	}
	if result.Output["reflection_draft"] == "" {
		t.Fatal("reflection_draft was empty")
	}
}

func TestMockVisionReviewAgentPassesWhenArtifactsExist(t *testing.T) {
	agent := NewMockVisionReviewAgent(0.7)

	result, err := agent.Run(context.Background(), domain.RunState{
		Artifacts: []domain.ArtifactRef{{ID: 1, VersionID: 2, Kind: "image"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score := result.Output["overall_score"].(float64); score < 0.7 {
		t.Fatalf("overall_score = %f, want at least 0.7", score)
	}
	if shouldRefine := result.Output["should_refine"].(bool); shouldRefine {
		t.Fatal("should_refine = true, want false")
	}
}
