package job

import (
	"testing"

	"github.com/hibiken/asynq"
)

func TestNewAgentV2RunTaskEncodesDurablePayload(t *testing.T) {
	task, err := NewAgentV2RunTask(AgentV2RunPayload{
		RunID:          53,
		UserID:         1,
		ConversationID: 39,
	})
	if err != nil {
		t.Fatalf("NewAgentV2RunTask() error = %v", err)
	}
	if task.Type() != TypeAgentV2Run {
		t.Fatalf("task.Type() = %q, want %q", task.Type(), TypeAgentV2Run)
	}

	payload, err := ParseAgentV2RunPayload(task)
	if err != nil {
		t.Fatalf("ParseAgentV2RunPayload() error = %v", err)
	}
	if payload.RunID != 53 || payload.UserID != 1 || payload.ConversationID != 39 {
		t.Fatalf("payload = %#v, want run/user/conversation IDs", payload)
	}
}

func TestParseAgentV2RunPayloadRejectsInvalidJSON(t *testing.T) {
	task := asynq.NewTask(TypeAgentV2Run, []byte("{"))

	if _, err := ParseAgentV2RunPayload(task); err == nil {
		t.Fatal("ParseAgentV2RunPayload() error = nil, want invalid JSON error")
	}
}
