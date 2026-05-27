package app

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"gin-biz-web-api/job"
)

type fakeAsyncTaskClient struct {
	task *asynq.Task
	err  error
}

func (client *fakeAsyncTaskClient) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	client.task = task
	return nil, client.err
}

func TestAsynqRunQueueEnqueuesAgentRunTask(t *testing.T) {
	client := &fakeAsyncTaskClient{}
	queue := NewAsynqRunQueue(client)

	err := queue.EnqueueAgentRun(context.Background(), AgentRunQueuePayload{
		RunID:          53,
		UserID:         1,
		ConversationID: 39,
	})
	if err != nil {
		t.Fatalf("EnqueueAgentRun() error = %v", err)
	}
	if client.task == nil {
		t.Fatal("EnqueueAgentRun() did not enqueue a task")
	}
	if client.task.Type() != job.TypeAgentV2Run {
		t.Fatalf("task.Type() = %q, want %q", client.task.Type(), job.TypeAgentV2Run)
	}
	payload, err := job.ParseAgentV2RunPayload(client.task)
	if err != nil {
		t.Fatalf("ParseAgentV2RunPayload() error = %v", err)
	}
	if payload.RunID != 53 || payload.UserID != 1 || payload.ConversationID != 39 {
		t.Fatalf("payload = %#v, want queued run identifiers", payload)
	}
}

func TestAsynqRunQueueRejectsMissingClient(t *testing.T) {
	queue := NewAsynqRunQueue(nil)

	if err := queue.EnqueueAgentRun(context.Background(), AgentRunQueuePayload{
		RunID:          1,
		UserID:         1,
		ConversationID: 1,
	}); err == nil {
		t.Fatal("EnqueueAgentRun() error = nil, want missing client error")
	}
}

func TestAsynqRunQueueReturnsClientError(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	queue := NewAsynqRunQueue(&fakeAsyncTaskClient{err: wantErr})

	err := queue.EnqueueAgentRun(context.Background(), AgentRunQueuePayload{
		RunID:          1,
		UserID:         1,
		ConversationID: 1,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("EnqueueAgentRun() error = %v, want %v", err, wantErr)
	}
}
