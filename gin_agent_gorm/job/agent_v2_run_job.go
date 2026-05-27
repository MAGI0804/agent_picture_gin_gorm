package job

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const TypeAgentV2Run = "agent_v2:run"

// AgentV2RunPayload is the durable queue payload for executing a persisted V2 run.
// Provider config, prompt text and API keys are intentionally read from DB by the worker.
type AgentV2RunPayload struct {
	RunID          uint `json:"run_id"`
	UserID         uint `json:"user_id"`
	ConversationID uint `json:"conversation_id"`
}

func NewAgentV2RunTask(payload AgentV2RunPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeAgentV2Run,
		data,
		asynq.Queue(DefaultQueueName),
		asynq.MaxRetry(2),
	), nil
}

func ParseAgentV2RunPayload(task *asynq.Task) (AgentV2RunPayload, error) {
	var payload AgentV2RunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return AgentV2RunPayload{}, err
	}
	return payload, nil
}
