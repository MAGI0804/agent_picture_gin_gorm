package app

import (
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
)

func TestQueuedRunStateRestoresRunIDAndBudgetDefaults(t *testing.T) {
	state := queuedRunState(model.AgentRun{
		BaseModel:      model.BaseModel{ID: 53},
		UserID:         1,
		ConversationID: 39,
		TaskType:       "image_generation",
		StateJSON:      `{"user_request":"make a poster","budget":{"max_steps":3}}`,
	})

	if state.RunID != 53 || state.UserID != 1 || state.ConversationID != 39 {
		t.Fatalf("state identifiers = %#v, want DB identifiers restored", state)
	}
	if state.UserRequest != "make a poster" {
		t.Fatalf("UserRequest = %q, want restored request", state.UserRequest)
	}
	if state.Budget.MaxSteps != 3 {
		t.Fatalf("MaxSteps = %d, want state budget", state.Budget.MaxSteps)
	}
	if state.Budget.MaxImageGenerations != 1 || state.Budget.MaxToolCalls != defaultRunMaxToolCalls || state.Budget.TimeoutSeconds != 180 {
		t.Fatalf("budget defaults = %#v, want image count and timeout defaults", state.Budget)
	}
}

func TestQueuedRunStateUsesRunBudgetJSONWhenStateBudgetMissing(t *testing.T) {
	state := queuedRunState(model.AgentRun{
		BaseModel:      model.BaseModel{ID: 7},
		UserID:         2,
		ConversationID: 3,
		TaskType:       "image_generation",
		BudgetJSON:     `{"max_steps":9,"max_image_generations":2,"timeout_seconds":240}`,
	})

	if state.Budget != (domain.RunBudget{MaxSteps: 9, MaxImageGenerations: 2, MaxToolCalls: defaultRunMaxToolCalls, TimeoutSeconds: 240, MaxAutoRefines: 1}) {
		t.Fatalf("Budget = %#v, want budget_json values", state.Budget)
	}
}

func TestMetadataUintParsesModelConfigID(t *testing.T) {
	got := metadataUint(map[string]string{"image_model_config_id": "6"}, "image_model_config_id")

	if got != 6 {
		t.Fatalf("metadataUint() = %d, want 6", got)
	}
}

func TestMergeClarificationAnswerAppendsAnswerAndClearsWaitingState(t *testing.T) {
	state := domain.RunState{
		UserRequest: "make a product poster",
		Requirements: domain.ImageRequirements{
			NeedClarification: true,
			Questions:         []string{"Which product should be featured?"},
		},
	}

	merged := mergeClarificationAnswer(state, "Feature the cold brew bottle.", 123)

	if merged.Requirements.NeedClarification {
		t.Fatal("NeedClarification = true, want false after answer")
	}
	if len(merged.Requirements.Questions) != 0 {
		t.Fatalf("Questions = %#v, want cleared", merged.Requirements.Questions)
	}
	if !strings.Contains(merged.UserRequest, "Feature the cold brew bottle.") {
		t.Fatalf("UserRequest = %q, want appended answer", merged.UserRequest)
	}
	if len(merged.Clarifications) != 1 || merged.Clarifications[0].Answer != "Feature the cold brew bottle." {
		t.Fatalf("Clarifications = %#v, want recorded clarification", merged.Clarifications)
	}
	if merged.Clarifications[0].CreatedAt != 123 {
		t.Fatalf("CreatedAt = %d, want 123", merged.Clarifications[0].CreatedAt)
	}
}

func TestIsExecutableQueuedRunStatusAllowsQueueRetryStatuses(t *testing.T) {
	if !isExecutableQueuedRunStatus(domain.RunStatusQueued) {
		t.Fatal("queued status should be executable")
	}
	if !isExecutableQueuedRunStatus(domain.RunStatusFailed) {
		t.Fatal("failed status should be executable for Asynq retry")
	}
	if isExecutableQueuedRunStatus(domain.RunStatusCompleted) {
		t.Fatal("completed status should not be executable")
	}
}
