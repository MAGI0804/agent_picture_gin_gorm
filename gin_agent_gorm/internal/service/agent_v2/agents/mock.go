package agents

import (
	"context"
	"fmt"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

// MockAgent 模拟 Agent，用于测试和演示
type MockAgent struct {
	key     string
	summary string
	output  map[string]interface{}
}

// NewMockAgent 创建 MockAgent 实例
func NewMockAgent(key string, summary string, output map[string]interface{}) *MockAgent {
	return &MockAgent{key: key, summary: summary, output: output}
}

// Key 返回 Agent 的唯一标识
func (agent *MockAgent) Key() string {
	return agent.key
}

// Run 执行 Agent 逻辑，返回步骤结果
func (agent *MockAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	select {
	case <-ctx.Done():
		return domain.StepResult{}, ctx.Err()
	default:
	}

	output := map[string]interface{}{
		"run_id":          state.RunID,
		"conversation_id": state.ConversationID,
		"agent":           agent.key,
	}
	for key, value := range agent.output {
		output[key] = value
	}

	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("%s: %s", agent.key, agent.summary),
		Output:  output,
	}, nil
}
