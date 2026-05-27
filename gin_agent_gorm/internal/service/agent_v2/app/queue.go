package app

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"

	"gin-biz-web-api/global"
	"gin-biz-web-api/job"
)

// AgentRunQueuePayload is the app-layer command for executing a persisted run.
type AgentRunQueuePayload struct {
	RunID          uint
	UserID         uint
	ConversationID uint
}

// RunQueue is the durable queue boundary used by CreateRunAsync.
type RunQueue interface {
	EnqueueAgentRun(ctx context.Context, payload AgentRunQueuePayload) error
}

// AsyncTaskClient is the subset of asynq.Client used by the V2 run queue.
type AsyncTaskClient interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// AsynqRunQueue enqueues V2 runs into the project's existing Asynq queue.
type AsynqRunQueue struct {
	client AsyncTaskClient
}

func NewAsynqRunQueue(client AsyncTaskClient) *AsynqRunQueue {
	return &AsynqRunQueue{client: client}
}

func NewDefaultRunQueue() RunQueue {
	if global.QueueJobClient == nil {
		return nil
	}
	return NewAsynqRunQueue(global.QueueJobClient)
}

func (queue *AsynqRunQueue) EnqueueAgentRun(ctx context.Context, payload AgentRunQueuePayload) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if queue == nil || queue.client == nil {
		return errors.New("agent v2 run queue client is not configured")
	}
	if payload.RunID == 0 {
		return errors.New("agent v2 run queue run_id is required")
	}
	if payload.UserID == 0 {
		return errors.New("agent v2 run queue user_id is required")
	}
	if payload.ConversationID == 0 {
		return errors.New("agent v2 run queue conversation_id is required")
	}
	task, err := job.NewAgentV2RunTask(job.AgentV2RunPayload{
		RunID:          payload.RunID,
		UserID:         payload.UserID,
		ConversationID: payload.ConversationID,
	})
	if err != nil {
		return err
	}
	_, err = queue.client.Enqueue(task)
	return err
}
