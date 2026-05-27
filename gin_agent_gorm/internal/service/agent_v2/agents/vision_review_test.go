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

func TestMockVisionReviewAgentReviewsEachCandidate(t *testing.T) {
	agent := NewMockVisionReviewAgent(0.7)

	result, err := agent.Run(context.Background(), domain.RunState{
		Artifacts: []domain.ArtifactRef{
			{ID: 1, VersionID: 10, Kind: "image", PreviewURL: "/preview/1"},
			{ID: 2, VersionID: 20, Kind: "image", PreviewURL: "/preview/2"},
			{ID: 3, VersionID: 30, Kind: "image", PreviewURL: "/preview/3"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	reviews, ok := result.Output["candidate_reviews"].([]domain.CandidateReview)
	if !ok {
		t.Fatalf("candidate_reviews type = %T, want []domain.CandidateReview", result.Output["candidate_reviews"])
	}
	if len(reviews) != 3 {
		t.Fatalf("candidate_reviews = %#v, want 3 reviews", reviews)
	}
	if reviews[1].ArtifactID != 2 || reviews[1].VersionID != 20 {
		t.Fatalf("second review = %#v, want artifact/version IDs", reviews[1])
	}
	if reviews[0].OverallScore <= reviews[1].OverallScore {
		t.Fatalf("candidate scores = %#v, want deterministic descending seed scores", reviews)
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

func TestVisionReviewAgentReviewsEachArtifactCandidate(t *testing.T) {
	provider := &fakeVisionProvider{
		resultsByImageRef: map[string]tools.VisionResult{
			"object-key-1": {
				Summary: "first candidate",
				Scores:  map[string]float64{"overall": 0.77},
				Issues:  []string{"minor crop"},
			},
			"object-key-2": {
				Summary:      "second candidate",
				Scores:       map[string]float64{"overall": 0.91},
				Issues:       []string{},
				ShouldRefine: false,
			},
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:           "gemini-vision",
		Kind:           tools.KindVision,
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
			{ObjectKey: "object-key-1", PreviewURL: "/static/1"},
			{ObjectKey: "object-key-2", PreviewURL: "/static/2"},
		},
		Artifacts: []domain.ArtifactRef{
			{ID: 1, VersionID: 10, Kind: "image", PreviewURL: "/preview/1"},
			{ID: 2, VersionID: 20, Kind: "image", PreviewURL: "/preview/2"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("vision provider calls = %d, want 2", len(provider.requests))
	}
	reviews, ok := result.Output["candidate_reviews"].([]domain.CandidateReview)
	if !ok {
		t.Fatalf("candidate_reviews type = %T, want []domain.CandidateReview", result.Output["candidate_reviews"])
	}
	if len(reviews) != 2 {
		t.Fatalf("candidate_reviews = %#v, want 2 reviews", reviews)
	}
	if reviews[0].ArtifactID != 1 || reviews[0].VersionID != 10 || reviews[0].ImageRef != "object-key-1" {
		t.Fatalf("first review = %#v, want first artifact candidate with object key image ref", reviews[0])
	}
	if reviews[1].OverallScore != 0.91 {
		t.Fatalf("second score = %f, want 0.91", reviews[1].OverallScore)
	}
	if result.Output["overall_score"].(float64) != 0.91 {
		t.Fatalf("overall_score = %f, want best candidate score", result.Output["overall_score"].(float64))
	}
}

func TestRankerAgentRanksCandidateReviews(t *testing.T) {
	agent := NewRankerAgent()

	result, err := agent.Run(context.Background(), domain.RunState{
		Requirements: domain.ImageRequirements{Subject: "coffee poster"},
		MemoryContext: []domain.MemoryItem{
			{Content: "prefer warm product lighting", Confidence: 0.9},
		},
		Review: domain.ReviewResult{
			CandidateReviews: []domain.CandidateReview{
				{ArtifactID: 1, VersionID: 10, OverallScore: 0.72, Issues: []string{"minor crop"}},
				{ArtifactID: 2, VersionID: 20, OverallScore: 0.91},
				{ArtifactID: 3, VersionID: 30, OverallScore: 0.65, ShouldRefine: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	reviews, ok := result.Output["candidate_reviews"].([]domain.CandidateReview)
	if !ok {
		t.Fatalf("candidate_reviews type = %T, want []domain.CandidateReview", result.Output["candidate_reviews"])
	}
	if len(reviews) != 3 {
		t.Fatalf("candidate reviews = %#v, want 3", reviews)
	}
	if reviews[0].ArtifactID != 2 {
		t.Fatalf("top ranked artifact = %d, want 2", reviews[0].ArtifactID)
	}
	if reviews[0].RankScore <= reviews[1].RankScore {
		t.Fatalf("rank scores = %#v, want descending order", reviews)
	}
	if result.Output["overall_score"].(float64) != reviews[0].OverallScore {
		t.Fatalf("overall_score = %f, want top review score %f", result.Output["overall_score"].(float64), reviews[0].OverallScore)
	}
}

type fakeVisionProvider struct {
	request           tools.VisionRequest
	requests          []tools.VisionRequest
	result            tools.VisionResult
	resultsByImageRef map[string]tools.VisionResult
}

func (provider *fakeVisionProvider) AnalyzeImage(ctx context.Context, request tools.VisionRequest) (tools.VisionResult, error) {
	provider.request = request
	provider.requests = append(provider.requests, request)
	if provider.resultsByImageRef != nil {
		if result, ok := provider.resultsByImageRef[request.ImageRef]; ok {
			return result, nil
		}
	}
	return provider.result, nil
}
