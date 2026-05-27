package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestExecutorReusesCompletedStepOnResume(t *testing.T) {
	firstNode := &countingNode{
		key:     "intent_router",
		summary: "should not run",
	}
	secondNode := &countingNode{
		key:     "second",
		summary: "second ran",
		output:  map[string]interface{}{"intent": "image_generation"},
	}
	reusableResult := domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "first restored",
		Output: map[string]interface{}{
			"task_type": "image_generation",
			"intent":    "generate",
		},
	}
	inputHash := hashText(mustJSON(domain.RunState{RunID: 1}))
	repo := &fakeRepository{
		reusableSteps: map[string]model.AgentStep{
			reusableKey(1, "intent_router", inputHash): {
				BaseModel:  model.BaseModel{ID: 9},
				Status:     domain.StepStatusCompleted,
				Output:     reusableResult.Summary,
				OutputJSON: mustJSON(reusableResult),
			},
		},
	}
	executor := NewExecutor(repo)

	state, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, workflow.Sequential(
		"resume_test",
		"0.1.0",
		firstNode,
		secondNode,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if firstNode.calls != 0 {
		t.Fatalf("first node calls = %d, want 0 because completed step should be reused", firstNode.calls)
	}
	if secondNode.calls != 1 {
		t.Fatalf("second node calls = %d, want 1", secondNode.calls)
	}
	if len(repo.steps) != 1 || repo.steps[0].StepKey != "second" {
		t.Fatalf("created steps = %#v, want only second step", repo.steps)
	}
	if state.Intent != "generate" || state.TaskType != "image_generation" {
		t.Fatalf("restored state = %#v, want reusable result applied", state)
	}
}

func TestExecutorRetriesRetryableProviderError(t *testing.T) {
	node := &flakyNode{
		key:       "provider_node",
		failures:  1,
		err:       fmt.Errorf("provider timeout while generating"),
		summary:   "retry succeeded",
		completed: map[string]interface{}{"intent": "generated"},
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)

	_, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, workflow.Sequential(
		"retry_test",
		"0.1.0",
		node,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if node.calls != 2 {
		t.Fatalf("node calls = %d, want initial call plus retry", node.calls)
	}
	if len(repo.steps) != 2 {
		t.Fatalf("created steps = %d, want 2 attempts", len(repo.steps))
	}
	if repo.steps[0].Attempt != 1 || repo.steps[0].Status != domain.StepStatusRetrying {
		t.Fatalf("first attempt = %#v, want attempt 1 retrying", repo.steps[0])
	}
	if repo.steps[1].Attempt != 2 || repo.steps[1].Status != domain.StepStatusCompleted {
		t.Fatalf("second attempt = %#v, want attempt 2 completed", repo.steps[1])
	}
	if len(repo.ledgerItems) != 1 {
		t.Fatalf("ledger item count = %d, want 1", len(repo.ledgerItems))
	}
	if repo.ledgerItems[0].Status != domain.StepStatusCompleted || repo.ledgerItems[0].RetryCount != 1 {
		t.Fatalf("ledger item = %#v, want completed with one retry", repo.ledgerItems[0])
	}
}

func TestExecutorPausesWhenRequirementNeedsClarification(t *testing.T) {
	promptNode := &countingNode{
		key:     "prompt_agent",
		summary: "should not run",
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)

	state, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, workflow.Sequential(
		"clarification_test",
		"0.1.0",
		agents.NewMockAgent("requirement_agent", "needs clarification", map[string]interface{}{
			"subject":            "poster",
			"need_clarification": true,
			"questions":          []string{"What product should be featured?"},
		}),
		promptNode,
	))
	if !errors.Is(err, ErrRunWaitingForUser) {
		t.Fatalf("Execute() error = %v, want ErrRunWaitingForUser", err)
	}
	if promptNode.calls != 0 {
		t.Fatalf("prompt node calls = %d, want 0 while waiting for user", promptNode.calls)
	}
	if repo.runStatus != domain.RunStatusWaiting {
		t.Fatalf("run status = %q, want waiting_user", repo.runStatus)
	}
	if len(repo.steps) != 1 || repo.steps[0].StepKey != "requirement_agent" || repo.steps[0].Status != domain.StepStatusCompleted {
		t.Fatalf("steps = %#v, want only completed requirement step", repo.steps)
	}
	if !state.Requirements.NeedClarification || len(state.Requirements.Questions) != 1 {
		t.Fatalf("requirements = %#v, want clarification questions preserved", state.Requirements)
	}
}

func TestExecutorDoesNotRetryBusinessError(t *testing.T) {
	node := &flakyNode{
		key:      "business_node",
		failures: 1,
		err:      fmt.Errorf("image prompt is required"),
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)

	_, err := executor.Execute(context.Background(), domain.RunState{RunID: 1}, workflow.Sequential(
		"business_error_test",
		"0.1.0",
		node,
	))
	if err == nil {
		t.Fatal("Execute() error = nil, want business error")
	}
	if node.calls != 1 {
		t.Fatalf("node calls = %d, want no retry", node.calls)
	}
	if len(repo.steps) != 1 || repo.steps[0].Attempt != 1 || repo.steps[0].Status != domain.StepStatusFailed {
		t.Fatalf("steps = %#v, want one failed attempt", repo.steps)
	}
}

func TestExecutorFailsWhenImageGenerationBudgetExceededByRetry(t *testing.T) {
	node := &flakyNode{
		key:      "image_generation_agent",
		failures: 1,
		err:      fmt.Errorf("provider rate limit 429"),
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)
	state := domain.RunState{
		RunID: 1,
		Budget: domain.RunBudget{
			MaxImageGenerations: 1,
		},
	}

	_, err := executor.Execute(context.Background(), state, workflow.Sequential(
		"image_budget_test",
		"0.1.0",
		node,
	))
	if err == nil || !strings.Contains(err.Error(), "image generation budget exceeded") {
		t.Fatalf("Execute() error = %v, want image generation budget exceeded", err)
	}
	if node.calls != 1 {
		t.Fatalf("node calls = %d, want second attempt blocked by budget", node.calls)
	}
	if len(repo.steps) != 1 || repo.steps[0].Status != domain.StepStatusRetrying {
		t.Fatalf("steps = %#v, want first attempt recorded as retrying", repo.steps)
	}
	if repo.lastRunAttrs["status"] != domain.RunStatusFailed {
		t.Fatalf("run status = %#v, want failed", repo.lastRunAttrs["status"])
	}
}

func TestExecutorFailsWhenToolCallBudgetExceededByRetry(t *testing.T) {
	node := &flakyNode{
		key:      "provider_node",
		failures: 1,
		err:      fmt.Errorf("temporary network timeout"),
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)
	state := domain.RunState{
		RunID: 1,
		Budget: domain.RunBudget{
			MaxToolCalls: 1,
		},
	}

	_, err := executor.Execute(context.Background(), state, workflow.Sequential(
		"tool_budget_test",
		"0.1.0",
		node,
	))
	if err == nil || !strings.Contains(err.Error(), "tool call budget exceeded") {
		t.Fatalf("Execute() error = %v, want tool call budget exceeded", err)
	}
	if node.calls != 1 {
		t.Fatalf("node calls = %d, want second attempt blocked by budget", node.calls)
	}
}

func TestExecutorFailsWhenTotalTimeoutBudgetExceeded(t *testing.T) {
	current := time.Unix(1000, 0)
	node := &countingNode{
		key:     "slow_node",
		summary: "slow success",
		onRun: func() {
			current = current.Add(11 * time.Second)
		},
	}
	repo := &fakeRepository{}
	executor := NewExecutor(repo)
	executor.now = func() time.Time {
		return current
	}
	state := domain.RunState{
		RunID: 1,
		Budget: domain.RunBudget{
			TimeoutSeconds: 10,
		},
	}

	_, err := executor.Execute(context.Background(), state, workflow.Sequential(
		"timeout_budget_test",
		"0.1.0",
		node,
	))
	if err == nil || !strings.Contains(err.Error(), "run timeout budget exceeded") {
		t.Fatalf("Execute() error = %v, want run timeout budget exceeded", err)
	}
	if len(repo.steps) != 1 || repo.steps[0].Status != domain.StepStatusFailed {
		t.Fatalf("steps = %#v, want slow step marked failed", repo.steps)
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
	reusableSteps        map[string]model.AgentStep
	ledgerItems          []model.TaskLedgerItem
}

func (repo *fakeRepository) CreateStep(step *model.AgentStep) error {
	step.ID = uint(len(repo.steps) + 1)
	repo.steps = append(repo.steps, *step)
	return nil
}

func (repo *fakeRepository) UpdateStep(stepID uint, attrs map[string]interface{}) error {
	for index := range repo.steps {
		if repo.steps[index].ID != stepID {
			continue
		}
		if status, ok := attrs["status"].(string); ok {
			repo.steps[index].Status = status
		}
		if errorMessage, ok := attrs["error_message"].(string); ok {
			repo.steps[index].ErrorMessage = errorMessage
		}
		if errorCode, ok := attrs["error_code"].(string); ok {
			repo.steps[index].ErrorCode = errorCode
		}
		if output, ok := attrs["output"].(string); ok {
			repo.steps[index].Output = output
		}
		if outputJSON, ok := attrs["output_json"].(string); ok {
			repo.steps[index].OutputJSON = outputJSON
		}
		if outputHash, ok := attrs["output_hash"].(string); ok {
			repo.steps[index].OutputHash = outputHash
		}
		if durationMS, ok := attrs["duration_ms"].(int64); ok {
			repo.steps[index].DurationMS = durationMS
		}
	}
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

func (repo *fakeRepository) FindReusableStep(runID uint, stepKey string, inputHash string) (model.AgentStep, bool, error) {
	if repo.reusableSteps == nil {
		return model.AgentStep{}, false, nil
	}
	step, ok := repo.reusableSteps[reusableKey(runID, stepKey, inputHash)]
	return step, ok, nil
}

func (repo *fakeRepository) MaxStepAttempt(runID uint, stepKey string, inputHash string) (int, error) {
	maxAttempt := 0
	for _, step := range repo.steps {
		if step.AgentRunID == runID && step.StepKey == stepKey && step.InputHash == inputHash && step.Attempt > maxAttempt {
			maxAttempt = step.Attempt
		}
	}
	return maxAttempt, nil
}

func (repo *fakeRepository) CountStepAttempts(runID uint) (int, error) {
	count := 0
	for _, step := range repo.steps {
		if step.AgentRunID == runID {
			count++
		}
	}
	for key, step := range repo.reusableSteps {
		if strings.HasPrefix(key, fmt.Sprintf("%d:", runID)) && step.ID != 0 {
			count++
		}
	}
	return count, nil
}

func (repo *fakeRepository) CountStepAttemptsByKey(runID uint, stepKey string) (int, error) {
	count := 0
	for _, step := range repo.steps {
		if step.AgentRunID == runID && step.StepKey == stepKey {
			count++
		}
	}
	for key, step := range repo.reusableSteps {
		if strings.HasPrefix(key, fmt.Sprintf("%d:%s:", runID, stepKey)) && step.ID != 0 {
			count++
		}
	}
	return count, nil
}

func (repo *fakeRepository) FindTaskLedgerItem(runID uint, taskKey string) (model.TaskLedgerItem, bool, error) {
	for _, item := range repo.ledgerItems {
		if item.AgentRunID == runID && item.TaskKey == taskKey {
			return item, true, nil
		}
	}
	return model.TaskLedgerItem{}, false, nil
}

func (repo *fakeRepository) CreateTaskLedgerItem(item *model.TaskLedgerItem) error {
	item.ID = uint(len(repo.ledgerItems) + 1)
	repo.ledgerItems = append(repo.ledgerItems, *item)
	return nil
}

func (repo *fakeRepository) UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error {
	for index := range repo.ledgerItems {
		if repo.ledgerItems[index].ID != itemID {
			continue
		}
		if status, ok := attrs["status"].(string); ok {
			repo.ledgerItems[index].Status = status
		}
		if outputRefsJSON, ok := attrs["output_refs_json"].(string); ok {
			repo.ledgerItems[index].OutputRefsJSON = outputRefsJSON
		}
		if retryCount, ok := attrs["retry_count"].(int); ok {
			repo.ledgerItems[index].RetryCount = retryCount
		}
		if errorMessage, ok := attrs["error_message"].(string); ok {
			repo.ledgerItems[index].ErrorMessage = errorMessage
		}
	}
	return nil
}

func reusableKey(runID uint, stepKey string, inputHash string) string {
	return fmt.Sprintf("%d:%s:%s", runID, stepKey, inputHash)
}

type countingNode struct {
	key     string
	calls   int
	summary string
	output  map[string]interface{}
	onRun   func()
}

func (node *countingNode) Key() string {
	return node.key
}

func (node *countingNode) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	node.calls++
	if node.onRun != nil {
		node.onRun()
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: node.summary,
		Output:  node.output,
	}, nil
}

type flakyNode struct {
	key       string
	calls     int
	failures  int
	err       error
	summary   string
	completed map[string]interface{}
}

func (node *flakyNode) Key() string {
	return node.key
}

func (node *flakyNode) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	node.calls++
	if node.calls <= node.failures {
		return domain.StepResult{}, node.err
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: node.summary,
		Output:  node.completed,
	}, nil
}
