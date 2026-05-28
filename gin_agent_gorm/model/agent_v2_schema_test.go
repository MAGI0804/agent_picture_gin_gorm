package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestAgentV2FirstRoundModelFields(t *testing.T) {
	tests := []struct {
		name       string
		modelType  interface{}
		fieldNames []string
	}{
		{
			name:      "agent run has resumable execution fields",
			modelType: AgentRun{},
			fieldNames: []string{
				"WorkflowName",
				"WorkflowVersion",
				"StateJSON",
				"BudgetJSON",
				"IdempotencyKey",
				"IdempotencyKeyUnique",
				"LockKey",
				"StartedAt",
				"CompletedAt",
				"CancelledAt",
			},
		},
		{
			name:      "agent step has timeline observability fields",
			modelType: AgentStep{},
			fieldNames: []string{
				"StepKey",
				"Attempt",
				"ProviderName",
				"ModelName",
				"DurationMS",
				"CostJSON",
				"InputJSON",
				"OutputJSON",
				"InputHash",
				"OutputHash",
				"ErrorCode",
			},
		},
		{
			name:      "context memory has v2 memory metadata",
			modelType: ContextMemory{},
			fieldNames: []string{
				"Namespace",
				"Scope",
				"SourceType",
				"SourceID",
				"ArtifactID",
				"TagsJSON",
				"Confidence",
				"EmbeddingID",
				"ExpiresAt",
				"LastUsedAt",
				"UseCount",
				"DeletedAt",
			},
		},
		{
			name:      "artifact has version lineage metadata",
			modelType: Artifact{},
			fieldNames: []string{
				"ParentArtifactID",
				"ArtifactGroupID",
				"RankScore",
				"SelectedAt",
				"Visibility",
				"StoragePolicy",
			},
		},
		{
			name:      "task ledger item exists",
			modelType: TaskLedgerItem{},
			fieldNames: []string{
				"AgentRunID",
				"TaskKey",
				"OwnerAgent",
				"Status",
				"DependsOnJSON",
				"InputRefsJSON",
				"OutputRefsJSON",
				"RetryCount",
			},
		},
		{
			name:      "tool invocation exists",
			modelType: ToolInvocation{},
			fieldNames: []string{
				"AgentRunID",
				"AgentStepID",
				"UserID",
				"ToolName",
				"ToolKind",
				"ProviderName",
				"ModelName",
				"Status",
				"InputJSON",
				"OutputJSON",
				"CostJSON",
				"DurationMS",
				"ErrorCode",
			},
		},
		{
			name:      "memory event exists",
			modelType: MemoryEvent{},
			fieldNames: []string{
				"MemoryID",
				"UserID",
				"ConversationID",
				"AgentRunID",
				"EventType",
				"SourceType",
				"SourceID",
				"BeforeJSON",
				"AfterJSON",
			},
		},
		{
			name:      "eval case exists",
			modelType: EvalCase{},
			fieldNames: []string{
				"AgentName",
				"Name",
				"InputJSON",
				"ExpectedJSON",
				"TagsJSON",
				"Status",
				"Weight",
			},
		},
		{
			name:      "eval run exists",
			modelType: EvalRun{},
			fieldNames: []string{
				"EvalCaseID",
				"PromptVersionID",
				"AgentName",
				"Status",
				"Score",
				"MetricsJSON",
				"ErrorMessage",
				"StartedAt",
				"CompletedAt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelType := reflect.TypeOf(tt.modelType)
			for _, fieldName := range tt.fieldNames {
				if _, ok := modelType.FieldByName(fieldName); !ok {
					t.Fatalf("%s is missing field %s", modelType.Name(), fieldName)
				}
			}
		})
	}
}

func TestAgentV2FirstRoundTableNames(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
	}{
		{name: TaskLedgerItem{}.TableName(), tableName: "task_ledger_items"},
		{name: ToolInvocation{}.TableName(), tableName: "tool_invocations"},
		{name: MemoryEvent{}.TableName(), tableName: "memory_events"},
		{name: EvalCase{}.TableName(), tableName: "eval_cases"},
		{name: EvalRun{}.TableName(), tableName: "eval_runs"},
	}

	for _, tt := range tests {
		t.Run(tt.tableName, func(t *testing.T) {
			if tt.name != tt.tableName {
				t.Fatalf("TableName() = %q, want %q", tt.name, tt.tableName)
			}
		})
	}
}

func TestAgentRunIdempotencyUniqueIndexTags(t *testing.T) {
	modelType := reflect.TypeOf(AgentRun{})
	userID, ok := modelType.FieldByName("UserID")
	if !ok {
		t.Fatal("AgentRun.UserID missing")
	}
	idempotencyKeyUnique, ok := modelType.FieldByName("IdempotencyKeyUnique")
	if !ok {
		t.Fatal("AgentRun.IdempotencyKeyUnique missing")
	}
	if !strings.Contains(string(userID.Tag), "uniqueIndex:idx_agent_runs_user_idempotency_unique") {
		t.Fatalf("UserID tag = %q, want composite idempotency unique index", userID.Tag)
	}
	if !strings.Contains(string(idempotencyKeyUnique.Tag), "uniqueIndex:idx_agent_runs_user_idempotency_unique") {
		t.Fatalf("IdempotencyKeyUnique tag = %q, want composite idempotency unique index", idempotencyKeyUnique.Tag)
	}
}
