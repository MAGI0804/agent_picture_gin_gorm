package runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"strings"
	"time"

	"github.com/pkg/errors"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/workflow"
	"gin-biz-web-api/model"
)

// Repository defines the persistence boundary used by the workflow executor.
type Repository interface {
	CreateStep(step *model.AgentStep) error
	UpdateStep(stepID uint, attrs map[string]interface{}) error
	UpdateRun(runID uint, attrs map[string]interface{}) error
	FindRunStatus(runID uint) (string, error)
	FindReusableStep(runID uint, stepKey string, inputHash string) (model.AgentStep, bool, error)
	MaxStepAttempt(runID uint, stepKey string, inputHash string) (int, error)
	CountStepAttempts(runID uint) (int, error)
	CountStepAttemptsByKey(runID uint, stepKey string) (int, error)
	FindTaskLedgerItem(runID uint, taskKey string) (model.TaskLedgerItem, bool, error)
	CreateTaskLedgerItem(item *model.TaskLedgerItem) error
	UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error
}

// Executor advances a workflow and records every step attempt.
type Executor struct {
	repo        Repository
	maxAttempts int
	now         func() time.Time
}

var ErrRunCancelled = errors.New("agent v2 run cancelled")

const defaultMaxAttempts = 3

// NewExecutor creates a workflow executor.
func NewExecutor(repo Repository) *Executor {
	return &Executor{
		repo:        repo,
		maxAttempts: defaultMaxAttempts,
		now:         time.Now,
	}
}

// Execute runs all workflow nodes in dependency order.
func (executor *Executor) Execute(
	ctx context.Context,
	state domain.RunState,
	flow workflow.Workflow,
) (domain.RunState, error) {
	if err := executor.ensureRunNotCancelled(state.RunID); err != nil {
		return state, err
	}
	startedAt := executor.now()
	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":           domain.RunStatusRunning,
		"workflow_name":    flow.Name,
		"workflow_version": flow.Version,
		"started_at":       int(startedAt.Unix()),
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

	usage, err := executor.initialBudgetUsage(state.RunID)
	if err != nil {
		_ = executor.failRun(state.RunID, err)
		return state, err
	}
	for _, node := range nodes {
		if err := executor.ensureRunNotCancelled(state.RunID); err != nil {
			return state, err
		}
		if err := executor.ensureWithinTimeoutBudget(state, startedAt); err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}

		inputJSON := mustJSON(state)
		inputHash := hashText(inputJSON)
		ledgerItem, err := executor.ensureTaskLedgerItem(state.RunID, node.Key(), flow.Dependencies[node.Key()], inputHash)
		if err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}
		reusableStep, ok, err := executor.repo.FindReusableStep(state.RunID, node.Key(), inputHash)
		if err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}
		if ok {
			result, err := stepResultFromCompletedStep(reusableStep)
			if err != nil {
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
				})
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			if err := executor.updateLedgerCompleted(ledgerItem.ID, reusableStep.ID, reusableStep.OutputHash, reusableStep.Attempt-1); err != nil {
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			state = applyStepResult(state, node.Key(), result)
			if err := executor.saveState(state); err != nil {
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			continue
		}

		lastAttempt, err := executor.repo.MaxStepAttempt(state.RunID, node.Key(), inputHash)
		if err != nil {
			_ = executor.failRun(state.RunID, err)
			return state, err
		}
		if lastAttempt >= executor.maxAttempts {
			err := errors.Errorf("step %s retry budget exhausted after %d attempts", node.Key(), lastAttempt)
			_ = executor.failRun(state.RunID, err)
			return state, err
		}

		completed := false
		for attempt := lastAttempt + 1; attempt <= executor.maxAttempts; attempt++ {
			if err := executor.ensureWithinTimeoutBudget(state, startedAt); err != nil {
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
					"retry_count":   maxInt(attempt-1, 0),
				})
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			if err := executor.consumeAttemptBudget(state, &usage, node.Key()); err != nil {
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
					"retry_count":   maxInt(attempt-1, 0),
				})
				_ = executor.failRun(state.RunID, err)
				return state, err
			}

			start := executor.now()
			step := model.AgentStep{
				AgentRunID: state.RunID,
				Name:       node.Key(),
				StepKey:    node.Key(),
				Status:     domain.StepStatusRunning,
				Attempt:    attempt,
				Input:      inputJSON,
				InputJSON:  inputJSON,
				InputHash:  inputHash,
			}
			if err := executor.repo.CreateStep(&step); err != nil {
				_ = executor.failRun(state.RunID, err)
				return state, err
			}

			executionState := state
			executionState.CurrentStepID = step.ID
			result, err := node.Run(ctx, executionState)
			durationMS := executor.now().Sub(start).Milliseconds()
			if cancelErr := executor.ensureRunNotCancelled(state.RunID); cancelErr != nil {
				_ = executor.repo.UpdateStep(step.ID, map[string]interface{}{
					"status":        domain.StepStatusCancelled,
					"error_message": cancelErr.Error(),
					"error_code":    "cancelled",
					"duration_ms":   durationMS,
				})
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusCancelled,
					"error_message": cancelErr.Error(),
					"retry_count":   maxInt(attempt-1, 0),
				})
				return state, cancelErr
			}
			if err != nil {
				status := domain.StepStatusFailed
				if isRetryableProviderError(err) && attempt < executor.maxAttempts {
					status = domain.StepStatusRetrying
				}
				_ = executor.repo.UpdateStep(step.ID, map[string]interface{}{
					"status":        status,
					"error_message": err.Error(),
					"error_code":    classifyStepError(err),
					"duration_ms":   durationMS,
				})
				if status == domain.StepStatusRetrying {
					if ledgerErr := executor.updateLedger(ledgerItem.ID, map[string]interface{}{
						"status":        domain.StepStatusRetrying,
						"error_message": err.Error(),
						"retry_count":   attempt,
					}); ledgerErr != nil {
						_ = executor.failRun(state.RunID, ledgerErr)
						return state, ledgerErr
					}
					continue
				}
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
					"retry_count":   maxInt(attempt-1, 0),
				})
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			if err := executor.ensureWithinTimeoutBudget(state, startedAt); err != nil {
				_ = executor.repo.UpdateStep(step.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
					"error_code":    "timeout_budget_exceeded",
					"duration_ms":   durationMS,
				})
				_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
					"status":        domain.StepStatusFailed,
					"error_message": err.Error(),
					"retry_count":   maxInt(attempt-1, 0),
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
			if err := executor.updateLedgerCompleted(ledgerItem.ID, step.ID, hashText(outputJSON), maxInt(attempt-1, 0)); err != nil {
				_ = executor.failRun(state.RunID, err)
				return state, err
			}

			state = applyStepResult(state, node.Key(), result)
			if err := executor.saveState(state); err != nil {
				_ = executor.failRun(state.RunID, err)
				return state, err
			}
			completed = true
			break
		}
		if !completed {
			err := errors.Errorf("step %s retry budget exhausted after %d attempts", node.Key(), executor.maxAttempts)
			_ = executor.updateLedger(ledgerItem.ID, map[string]interface{}{
				"status":        domain.StepStatusFailed,
				"error_message": err.Error(),
				"retry_count":   executor.maxAttempts - 1,
			})
			_ = executor.failRun(state.RunID, err)
			return state, err
		}
	}

	if err := executor.ensureRunNotCancelled(state.RunID); err != nil {
		return state, err
	}
	if err := executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"status":       domain.RunStatusCompleted,
		"completed_at": int(executor.now().Unix()),
		"state_json":   mustJSON(state),
	}); err != nil {
		return state, err
	}
	return state, nil
}

type budgetUsage struct {
	toolCalls        int
	imageGenerations int
}

func (executor *Executor) initialBudgetUsage(runID uint) (budgetUsage, error) {
	toolCalls, err := executor.repo.CountStepAttempts(runID)
	if err != nil {
		return budgetUsage{}, err
	}
	imageGenerations, err := executor.repo.CountStepAttemptsByKey(runID, "image_generation_agent")
	if err != nil {
		return budgetUsage{}, err
	}
	return budgetUsage{
		toolCalls:        toolCalls,
		imageGenerations: imageGenerations,
	}, nil
}

func (executor *Executor) ensureTaskLedgerItem(runID uint, taskKey string, dependencies []string, inputHash string) (model.TaskLedgerItem, error) {
	item, ok, err := executor.repo.FindTaskLedgerItem(runID, taskKey)
	if err != nil {
		return item, err
	}
	inputRefsJSON := mustJSON(map[string]interface{}{
		"input_hash": inputHash,
	})
	if ok {
		err = executor.updateLedger(item.ID, map[string]interface{}{
			"status":          domain.StepStatusRunning,
			"depends_on_json": mustJSON(dependencies),
			"input_refs_json": inputRefsJSON,
			"error_message":   "",
		})
		return item, err
	}
	item = model.TaskLedgerItem{
		AgentRunID:    runID,
		TaskKey:       taskKey,
		OwnerAgent:    taskKey,
		Status:        domain.StepStatusRunning,
		DependsOnJSON: mustJSON(dependencies),
		InputRefsJSON: inputRefsJSON,
	}
	err = executor.repo.CreateTaskLedgerItem(&item)
	return item, err
}

func (executor *Executor) updateLedgerCompleted(itemID uint, stepID uint, outputHash string, retryCount int) error {
	return executor.updateLedger(itemID, map[string]interface{}{
		"status": domain.StepStatusCompleted,
		"output_refs_json": mustJSON(map[string]interface{}{
			"step_id":     stepID,
			"output_hash": outputHash,
		}),
		"retry_count":   maxInt(retryCount, 0),
		"error_message": "",
	})
}

func (executor *Executor) updateLedger(itemID uint, attrs map[string]interface{}) error {
	if itemID == 0 {
		return nil
	}
	return executor.repo.UpdateTaskLedgerItem(itemID, attrs)
}

func (executor *Executor) saveState(state domain.RunState) error {
	return executor.repo.UpdateRun(state.RunID, map[string]interface{}{
		"state_json": mustJSON(state),
		"task_type":  state.TaskType,
		"intent":     state.Intent,
	})
}

func (executor *Executor) consumeAttemptBudget(state domain.RunState, usage *budgetUsage, nodeKey string) error {
	if state.Budget.MaxToolCalls > 0 && usage.toolCalls >= state.Budget.MaxToolCalls {
		return errors.Errorf("tool call budget exceeded: attempted %d calls, max_tool_calls is %d", usage.toolCalls+1, state.Budget.MaxToolCalls)
	}
	if nodeKey == "image_generation_agent" && state.Budget.MaxImageGenerations > 0 && usage.imageGenerations >= state.Budget.MaxImageGenerations {
		return errors.Errorf("image generation budget exceeded: attempted %d generations, max_image_generations is %d", usage.imageGenerations+1, state.Budget.MaxImageGenerations)
	}
	usage.toolCalls++
	if nodeKey == "image_generation_agent" {
		usage.imageGenerations++
	}
	return nil
}

func (executor *Executor) ensureWithinTimeoutBudget(state domain.RunState, startedAt time.Time) error {
	if state.Budget.TimeoutSeconds <= 0 {
		return nil
	}
	elapsed := executor.now().Sub(startedAt)
	if elapsed <= time.Duration(state.Budget.TimeoutSeconds)*time.Second {
		return nil
	}
	return errors.Errorf("run timeout budget exceeded: elapsed %dms, timeout_seconds is %d", elapsed.Milliseconds(), state.Budget.TimeoutSeconds)
}

func stepResultFromCompletedStep(step model.AgentStep) (domain.StepResult, error) {
	var result domain.StepResult
	if strings.TrimSpace(step.OutputJSON) == "" {
		return domain.StepResult{
			Status:  domain.StepStatusCompleted,
			Summary: step.Output,
			Output:  map[string]interface{}{},
		}, nil
	}
	if err := json.Unmarshal([]byte(step.OutputJSON), &result); err != nil {
		return domain.StepResult{}, errors.Wrapf(err, "decode completed step %d output", step.ID)
	}
	if result.Status == "" {
		result.Status = domain.StepStatusCompleted
	}
	if result.Output == nil {
		result.Output = map[string]interface{}{}
	}
	return result, nil
}

func (executor *Executor) ensureRunNotCancelled(runID uint) error {
	status, err := executor.repo.FindRunStatus(runID)
	if err != nil {
		return err
	}
	if status == domain.RunStatusCancelled {
		return ErrRunCancelled
	}
	return nil
}

func (executor *Executor) failRun(runID uint, err error) error {
	if err == nil {
		err = errors.New("agent v2 run failed")
	}
	return executor.repo.UpdateRun(runID, map[string]interface{}{
		"status":        domain.RunStatusFailed,
		"error_message": err.Error(),
	})
}

func isRetryableProviderError(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, context.Canceled) {
		return false
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	message := strings.ToLower(err.Error())
	retryableFragments := []string{
		"timeout",
		"deadline exceeded",
		"temporar",
		"connection reset",
		"connection refused",
		"i/o timeout",
		"rate limit",
		"too many requests",
		"429",
		"502",
		"503",
		"504",
	}
	for _, fragment := range retryableFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func classifyStepError(err error) string {
	if err == nil {
		return ""
	}
	if stderrors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if isRetryableProviderError(err) {
		return "retryable_provider_error"
	}
	return "provider_error"
}

// applyStepResult merges one node output into the shared RunState.
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
		if subject, ok := result.Output["subject"].(string); ok {
			state.Requirements.Subject = subject
		}
		if style, ok := result.Output["style"].(string); ok {
			state.Requirements.Style = style
		}
		if aspectRatio, ok := result.Output["aspect_ratio"].(string); ok {
			state.Requirements.AspectRatio = aspectRatio
		}
		state.Requirements.MustInclude = parseIssueList(result.Output["must_include"])
		state.Requirements.MustAvoid = parseIssueList(result.Output["must_avoid"])
		if needClarification, ok := result.Output["need_clarification"].(bool); ok {
			state.Requirements.NeedClarification = needClarification
		}
		state.Requirements.Questions = parseIssueList(result.Output["questions"])
	case "prompt_agent":
		if prompt, ok := result.Output["positive_prompt"].(string); ok {
			state.Prompts.PositivePrompt = prompt
		}
		if prompt, ok := result.Output["negative_prompt"].(string); ok {
			state.Prompts.NegativePrompt = prompt
		}
		if renderTextSeparately, ok := result.Output["render_text_separately"].(bool); ok {
			state.Prompts.RenderTextSeparately = renderTextSeparately
		}
		state.Prompts.Params = parseStringMap(result.Output["params"])
	case "image_generation_agent":
		state.GeneratedImages = parseGeneratedImages(result.Output["generated_images"])
	case "artifact_agent":
		state.Artifacts = append(state.Artifacts, result.Artifacts...)
	case "vision_review_agent":
		if score, ok := result.Output["overall_score"].(float64); ok {
			state.Review.OverallScore = score
		}
		state.Review.Issues = parseIssueList(result.Output["issues"])
		if shouldRefine, ok := result.Output["should_refine"].(bool); ok {
			state.Review.ShouldRefine = shouldRefine
		}
		if reviewer, ok := result.Output["reviewer"].(string); ok {
			state.Review.Reviewer = reviewer
		}
	}
	return state
}

func parseStringMap(value interface{}) map[string]string {
	switch params := value.(type) {
	case map[string]string:
		return params
	case map[string]interface{}:
		result := make(map[string]string, len(params))
		for key, value := range params {
			if text, ok := value.(string); ok {
				result[key] = text
			}
		}
		return result
	default:
		return map[string]string{}
	}
}

func parseGeneratedImages(value interface{}) []domain.GeneratedImageRef {
	switch images := value.(type) {
	case []domain.GeneratedImageRef:
		return images
	case []interface{}:
		result := make([]domain.GeneratedImageRef, 0, len(images))
		for _, image := range images {
			if item, ok := image.(domain.GeneratedImageRef); ok {
				result = append(result, item)
				continue
			}
			if item, ok := image.(map[string]interface{}); ok {
				result = append(result, domain.GeneratedImageRef{
					Name:       stringValue(item["name"]),
					Kind:       stringValue(item["kind"]),
					MimeType:   stringValue(item["mime_type"]),
					ObjectKey:  stringValue(item["object_key"]),
					PreviewURL: stringValue(item["preview_url"]),
					SizeBytes:  int64Value(item["size_bytes"]),
					Hash:       stringValue(item["hash"]),
				})
			}
		}
		return result
	default:
		return []domain.GeneratedImageRef{}
	}
}

func stringValue(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func int64Value(value interface{}) int64 {
	switch number := value.(type) {
	case int64:
		return number
	case int:
		return int64(number)
	case float64:
		return int64(number)
	default:
		return 0
	}
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
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

// mustJSON serializes a value for deterministic step snapshots.
func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// hashText computes a SHA1 snapshot hash for step inputs and outputs.
func hashText(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
