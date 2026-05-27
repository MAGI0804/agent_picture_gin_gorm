package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/agents"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"
)

func TestExecutorFailsRunWhenStepBudgetExceeded(t *testing.T) {
	repo := &fakeRepository{}
	executor := NewExecutor(repo)
	state := domain.RunState{
		RunID: 1,
		Budget: domain.RunBudget{
			MaxSteps: 1,
		},
	}
	flow := workflow.Sequential(
		"budget_test",
		"0.1.0",
		agents.NewMockAgent("first", "first", map[string]interface{}{}),
		agents.NewMockAgent("second", "second", map[string]interface{}{}),
	)

	_, err := executor.Execute(context.Background(), state, flow)
	if err == nil {
		t.Fatal("Execute() error = nil, want budget exceeded error")
	}
	if !strings.Contains(err.Error(), "step budget exceeded") {
		t.Fatalf("Execute() error = %q, want step budget exceeded", err.Error())
	}
	if len(repo.steps) != 0 {
		t.Fatalf("created %d steps, want 0", len(repo.steps))
	}
	if repo.lastRunAttrs["status"] != domain.RunStatusFailed {
		t.Fatalf("run status = %#v, want failed", repo.lastRunAttrs["status"])
	}
}

func TestExecutorDoesNotStartCancelledRun(t *testing.T) {
	repo := &fakeRepository{runStatus: domain.RunStatusCancelled}
	executor := NewExecutor(repo)
	flow := workflow.Sequential(
		"cancel_test",
		"0.1.0",
		agents.NewMockAgent("first", "first", map[string]interface{}{}),
	)

	_, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, flow)
	if !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("Execute() error = %v, want ErrRunCancelled", err)
	}
	if len(repo.steps) != 0 {
		t.Fatalf("created %d steps, want 0", len(repo.steps))
	}
	if repo.lastRunAttrs != nil && repo.lastRunAttrs["status"] == domain.RunStatusRunning {
		t.Fatalf("run was marked running after cancellation: %#v", repo.lastRunAttrs)
	}
}

func TestExecutorStopsBeforeNextStepWhenRunCancelled(t *testing.T) {
	repo := &fakeRepository{cancelAfterFirstStep: true}
	executor := NewExecutor(repo)
	flow := workflow.Sequential(
		"cancel_test",
		"0.1.0",
		agents.NewMockAgent("first", "first", map[string]interface{}{}),
		agents.NewMockAgent("second", "second", map[string]interface{}{}),
	)

	_, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, flow)
	if !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("Execute() error = %v, want ErrRunCancelled", err)
	}
	if len(repo.steps) != 1 {
		t.Fatalf("created %d steps, want only the first step", len(repo.steps))
	}
	if repo.runStatus == domain.RunStatusCompleted {
		t.Fatal("run status = completed, want cancelled to remain terminal")
	}
}

func TestApplyStepResultStoresVisionReview(t *testing.T) {
	state := applyStepResult(domain.RunState{}, "vision_review_agent", domain.StepResult{
		Summary: "reviewed",
		Output: map[string]interface{}{
			"overall_score": 0.42,
			"issues": []string{
				"no artifact generated",
			},
			"should_refine": true,
		},
	})

	if state.Review.OverallScore != 0.42 {
		t.Fatalf("overall score = %f, want 0.42", state.Review.OverallScore)
	}
	if len(state.Review.Issues) != 1 || state.Review.Issues[0] != "no artifact generated" {
		t.Fatalf("issues = %#v, want no artifact generated", state.Review.Issues)
	}
	if !state.Review.ShouldRefine {
		t.Fatal("ShouldRefine = false, want true")
	}
}

func TestApplyStepResultStoresVisionReviewFromInterfaceIssues(t *testing.T) {
	state := applyStepResult(domain.RunState{}, "vision_review_agent", domain.StepResult{
		Output: map[string]interface{}{
			"overall_score": 0.30,
			"issues": []interface{}{
				"missing artifact",
				"requirements still need clarification",
			},
		},
	})

	if len(state.Review.Issues) != 2 {
		t.Fatalf("issues = %#v, want 2 issues", state.Review.Issues)
	}
}

type fakeRepository struct {
	steps                []model.AgentStep
	lastRunAttrs         map[string]interface{}
	runStatus            string
	cancelAfterFirstStep bool
}

func (repo *fakeRepository) CreateStep(step *model.AgentStep) error {
	step.ID = uint(len(repo.steps) + 1)
	repo.steps = append(repo.steps, *step)
	return nil
}

func (repo *fakeRepository) UpdateStep(stepID uint, attrs map[string]interface{}) error {
	if repo.cancelAfterFirstStep && stepID == 1 && attrs["status"] == domain.StepStatusCompleted {
		repo.runStatus = domain.RunStatusCancelled
	}
	return nil
}

func (repo *fakeRepository) UpdateRun(runID uint, attrs map[string]interface{}) error {
	repo.lastRunAttrs = attrs
	if status, ok := attrs["status"].(string); ok {
		repo.runStatus = status
	}
	return nil
}

func (repo *fakeRepository) FindRunStatus(runID uint) (string, error) {
	return repo.runStatus, nil
}
