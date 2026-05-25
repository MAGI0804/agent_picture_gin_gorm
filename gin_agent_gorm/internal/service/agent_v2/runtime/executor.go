package runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"
)

// Repository 数据访问接口，定义执行器需要的数据库操作
type Repository interface {
	CreateStep(step *model.AgentStep) error
	UpdateStep(stepID uint, attrs map[string]interface{}) error
	UpdateRun(runID uint, attrs map[string]interface{}) error
}

// Executor 工作流执行器，负责按顺序执行工作流中的所有节点
type Executor struct {
	repo Repository
}

// NewExecutor 创建 Executor 实例
func NewExecutor(repo Repository) *Executor {
	return &Executor{repo: repo}
}

// Execute 执行工作流，按顺序运行所有节点
func (executor *Executor) Execute(ctx context.Context, state domain.RunState, flow workflow.Workflow) (domain.RunState, error) {
	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":           domain.RunStatusRunning,
		"workflow_name":    flow.Name,
		"workflow_version": flow.Version,
		"started_at":       int(time.Now().Unix()),
	}); err != nil {
		return state, err
	}

	for _, node := range flow.Nodes {
		start := time.Now()
		inputJSON := mustJSON(state)
		step := model.AgentStep{
			AgentRunID: state.RunID,
			Name:       node.Key(),
			StepKey:    node.Key(),
			Status:     domain.StepStatusRunning,
			Attempt:    1,
			Input:      inputJSON,
			InputJSON:  inputJSON,
			InputHash:  hashText(inputJSON),
		}
		if err := executor.repo.CreateStep(&step); err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}

		result, err := node.Run(ctx, state)
		durationMS := time.Since(start).Milliseconds()
		if err != nil {
			_ = executor.repo.UpdateStep(step.ID, map[string]interface{}{
				"status":        domain.StepStatusFailed,
				"error_message": err.Error(),
				"duration_ms":   durationMS,
			})
			_ = executor.failRun(state.RunID, err)
			return state, err
		}

		if result.Status == "" {
			result.Status = domain.StepStatusCompleted
		}
		outputJSON := mustJSON(result)
		if err := executor.repo.UpdateStep(step.ID, map[string]interface{}{
			"status":      result.Status,
			"output":      result.Summary,
			"output_json": outputJSON,
			"output_hash": hashText(outputJSON),
			"duration_ms": durationMS,
		}); err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}

		state = applyStepResult(state, node.Key(), result)
		if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
			"state_json": mustJSON(state),
			"task_type":  state.TaskType,
			"intent":     state.Intent,
		}); err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}
	}

	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":       domain.RunStatusCompleted,
		"completed_at": int(time.Now().Unix()),
		"state_json":   mustJSON(state),
	}); err != nil {
		return state, err
	}
	return state, nil
}

// failRun 将运行标记为失败
func (executor *Executor) failRun(runID uint, err error) error {
	if err == nil {
		err = errors.New("agent v2 run failed")
	}
	return executor.repo.UpdateRun(runID, map[string]interface{}{
		"status":        domain.RunStatusFailed,
		"error_message": err.Error(),
	})
}

// applyStepResult 将步骤结果应用到运行状态
func applyStepResult(state domain.RunState, key string, result domain.StepResult) domain.RunState {
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	state.Metadata[key] = result.Summary

	switch key {
	case "intent_router":
		if taskType, ok := result.Output["task_type"].(string); ok && taskType != "" {
			state.TaskType = taskType
		}
		if intent, ok := result.Output["intent"].(string); ok && intent != "" {
			state.Intent = intent
		}
	case "requirement_agent":
		state.Requirements.NeedClarification = false
	case "prompt_agent":
		if prompt, ok := result.Output["positive_prompt"].(string); ok {
			state.Prompts.PositivePrompt = prompt
		}
	}
	return state
}

// mustJSON 将对象序列化为 JSON，出错时返回空对象
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// hashText 计算文本的 SHA1 哈希值
func hashText(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
