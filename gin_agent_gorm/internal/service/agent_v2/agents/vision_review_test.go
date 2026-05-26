package agents

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
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

func TestVisionReviewAgentUsesRegisteredVisionProvider(t *testing.T) {
	provider := &fakeVisionProvider{
		result: tools.VisionResult{
			Summary:      "clean product image",
			Scores:       map[string]float64{"overall": 0.91},
			Issues:       []string{"minor shadow"},
			ShouldRefine: false,
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:           "gemini-vision",
		Kind:           tools.KindVision,
		Provider:       "google",
		Model:          "gemini-3.5-flash",
		ModelConfigID:  7,
		VisionProvider: provider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	agent := NewVisionReviewAgent(registry, VisionReviewAgentOptions{
		VisionModelConfigID: 7,
		MinPassingScore:     0.7,
	})
	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "make a clean product poster",
		GeneratedImages: []domain.GeneratedImageRef{
			{ObjectKey: "user-1/conversation-1/run-1/generated-image.png"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.request.ImageRef != "user-1/conversation-1/run-1/generated-image.png" {
		t.Fatalf("image ref = %q, want generated image object key", provider.request.ImageRef)
	}
	if score := result.Output["overall_score"].(float64); score != 0.91 {
		t.Fatalf("overall_score = %f, want 0.91", score)
	}
	if result.Output["reviewer"] != "real_vision_review" {
		t.Fatalf("reviewer = %q, want real_vision_review", result.Output["reviewer"])
	}
}

type fakeVisionProvider struct {
	request tools.VisionRequest
	result  tools.VisionResult
}

func (provider *fakeVisionProvider) AnalyzeImage(ctx context.Context, request tools.VisionRequest) (tools.VisionResult, error) {
	provider.request = request
	return provider.result, nil
}
