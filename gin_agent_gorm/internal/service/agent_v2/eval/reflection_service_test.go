package eval

import (
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
)

func TestReflectionServiceDraftsLowScoreReflection(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewReflectionService(repo, 0.7)

	reflection, drafted, err := svc.DraftLowScoreReflection(DraftReflectionInput{
		RunID:     12,
		AgentName: "vision_review_agent",
		Review: domain.ReviewResult{
			OverallScore: 0.42,
			Issues:       []string{"no artifact generated"},
			ShouldRefine: true,
		},
	})
	if err != nil {
		t.Fatalf("DraftLowScoreReflection() error = %v", err)
	}
	if !drafted {
		t.Fatal("drafted = false, want true")
	}
	if reflection.ID == 0 {
		t.Fatal("reflection ID was not assigned")
	}
	if reflection.PromotedToMemory {
		t.Fatal("PromotedToMemory = true, want false for draft")
	}
	if repo.reflection.FailureType != FailureTypeLowReviewScore {
		t.Fatalf("failure type = %q, want %q", repo.reflection.FailureType, FailureTypeLowReviewScore)
	}
}

func TestReflectionServiceSkipsHighScoreReflection(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewReflectionService(repo, 0.7)

	_, drafted, err := svc.DraftLowScoreReflection(DraftReflectionInput{
		RunID:     12,
		AgentName: "vision_review_agent",
		Review: domain.ReviewResult{
			OverallScore: 0.9,
		},
	})
	if err != nil {
		t.Fatalf("DraftLowScoreReflection() error = %v", err)
	}
	if drafted {
		t.Fatal("drafted = true, want false")
	}
	if repo.reflection.ID != 0 {
		t.Fatalf("created reflection %#v, want none", repo.reflection)
	}
}

type fakeRepository struct {
	reflection model.AgentReflection
}

func (repo *fakeRepository) CreateReflection(reflection *model.AgentReflection) error {
	reflection.ID = 100
	repo.reflection = *reflection
	return nil
}
