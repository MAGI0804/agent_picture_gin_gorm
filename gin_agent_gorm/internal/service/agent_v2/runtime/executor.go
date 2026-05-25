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

// Repository 定义执行器需要的数据写入能力，便于后续替换 DAO 或做单元测试。
type Repository interface {
	CreateStep(step *model.AgentStep) error
	UpdateStep(stepID uint, attrs map[string]interface{}) error
	UpdateRun(runID uint, attrs map[string]interface{}) error
}

// Executor 负责推进 workflow：创建 step、执行节点、保存状态、处理失败。
type Executor struct {
	repo Repository
}

// NewExecutor 创建 workflow 执行器。
func NewExecutor(repo Repository) *Executor {
	return &Executor{repo: repo}
}

// Execute 按 workflow 定义的顺序执行所有节点，并把每一步写入 agent_steps。
func (executor *Executor) Execute(
	ctx context.Context,
	state domain.RunState,
	flow workflow.Workflow,
) (domain.RunState, error) {
	// 第一步：标记 run 开始执行，并记录 workflow 名称和版本。
	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":           domain.RunStatusRunning,
		"workflow_name":    flow.Name,
		"workflow_version": flow.Version,
		"started_at":       int(time.Now().Unix()),
	}); err != nil {
		return state, err
	}

	nodes, err := flow.OrderedNodes()
	if err != nil {
		_ = executor.failRun(state.RunID, err)
		return state, err
	}
	if state.Budget.MaxSteps > 0 && len(nodes) > state.Budget.MaxSteps {
		err := errors.Errorf("step budget exceeded: workflow requires %d steps, max_steps is %d", len(nodes), state.Budget.MaxSteps)
		_ = executor.failRun(state.RunID, err)
		return state, err
	}

	for _, node := range nodes {
		start := time.Now()

		// 第二步：执行节点前保存输入快照和 input hash，后续用于重试和审计。
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

		// 第三步：调用具体 Agent 节点。当前是 mock 节点，后续替换真实 Agent。
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

		// 第四步：保存节点输出、输出 hash 和耗时，形成可观察 timeline。
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

		// 第五步：把节点结果合并进 RunState，并保存最新 state_json。
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

	// 第六步：所有节点完成后标记 run 完成。
	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":       domain.RunStatusCompleted,
		"completed_at": int(time.Now().Unix()),
		"state_json":   mustJSON(state),
	}); err != nil {
		return state, err
	}
	return state, nil
}

// failRun 将 run 标记为失败，并保存错误摘要。
func (executor *Executor) failRun(runID uint, err error) error {
	if err == nil {
		err = errors.New("agent v2 run failed")
	}
	return executor.repo.UpdateRun(runID, map[string]interface{}{
		"status":        domain.RunStatusFailed,
		"error_message": err.Error(),
	})
}

// applyStepResult 把节点输出合并到共享 RunState。
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
	case "vision_review_agent":
		if score, ok := result.Output["overall_score"].(float64); ok {
			state.Review.OverallScore = score
		}
		state.Review.Issues = parseIssueList(result.Output["issues"])
		if shouldRefine, ok := result.Output["should_refine"].(bool); ok {
			state.Review.ShouldRefine = shouldRefine
		}
	}
	return state
}

func parseIssueList(value interface{}) []string {
	switch issues := value.(type) {
	case []string:
		return issues
	case []interface{}:
		result := make([]string, 0, len(issues))
		for _, issue := range issues {
			if text, ok := issue.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return []string{}
	}
}

// mustJSON 将对象序列化为 JSON，序列化失败时返回空对象字符串。
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// hashText 计算文本 SHA1，用于记录 step 输入输出快照。
func hashText(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
