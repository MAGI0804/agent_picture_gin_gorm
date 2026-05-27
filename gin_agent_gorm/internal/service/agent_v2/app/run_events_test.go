package app

import (
	"testing"

	"gin-biz-web-api/model"
)

func TestBuildRunEventsSortsAndAssignsCursor(t *testing.T) {
	events := buildRunEvents(
		[]model.AgentStep{
			{
				BaseModel: model.BaseModel{ID: 3},
				Name:      "prompt_agent",
				CommonTimestampsField: model.CommonTimestampsField{
					CreatedAt: 20,
				},
			},
		},
		[]model.TaskLedgerItem{
			{
				BaseModel: model.BaseModel{ID: 1},
				TaskKey:   "intent_router",
				CommonTimestampsField: model.CommonTimestampsField{
					CreatedAt: 10,
				},
			},
		},
		[]model.ToolInvocation{
			{
				BaseModel: model.BaseModel{ID: 2},
				ToolName:  "imagen",
				CommonTimestampsField: model.CommonTimestampsField{
					CreatedAt: 20,
				},
			},
		},
	)

	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Cursor != 1 || events[0].Type != "task_ledger_item" {
		t.Fatalf("events[0] = %#v, want first ledger event", events[0])
	}
	if events[1].Cursor != 2 || events[1].Type != "agent_step" {
		t.Fatalf("events[1] = %#v, want stable step event before tool at same timestamp", events[1])
	}
	if events[2].Cursor != 3 || events[2].Type != "tool_invocation" {
		t.Fatalf("events[2] = %#v, want third tool event", events[2])
	}
}
