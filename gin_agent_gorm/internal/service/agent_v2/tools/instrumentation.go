package tools

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/model"
)

// InvocationStore is the persistence boundary for tool invocation tracing.
type InvocationStore interface {
	CreateToolInvocation(invocation *model.ToolInvocation) error
	UpdateToolInvocation(invocationID uint, attrs map[string]interface{}) error
}

// InstrumentTool wraps tool providers so external calls are persisted as tool_invocations.
func InstrumentTool(tool Tool, store InvocationStore) Tool {
	if store == nil {
		return tool
	}
	if tool.TextProvider != nil {
		tool.TextProvider = instrumentedTextProvider{tool: tool, store: store, next: tool.TextProvider}
	}
	if tool.ImageGenerationProvider != nil {
		tool.ImageGenerationProvider = instrumentedImageGenerationProvider{tool: tool, store: store, next: tool.ImageGenerationProvider}
	}
	if tool.VisionProvider != nil {
		tool.VisionProvider = instrumentedVisionProvider{tool: tool, store: store, next: tool.VisionProvider}
	}
	return tool
}

type instrumentedTextProvider struct {
	tool  Tool
	store InvocationStore
	next  TextProvider
}

func (provider instrumentedTextProvider) GenerateText(ctx context.Context, request TextRequest) (TextResult, error) {
	startedAt := time.Now()
	invocation, err := provider.startInvocation(request.UserID, request.RunID, request.StepID, textInputSummary(request), startedAt)
	if err != nil {
		return TextResult{}, err
	}
	result, err := provider.next.GenerateText(ctx, request)
	provider.finishInvocation(invocation.ID, startedAt, textOutputSummary(result), err)
	return result, err
}

func (provider instrumentedTextProvider) startInvocation(userID uint, runID uint, stepID uint, input interface{}, startedAt time.Time) (model.ToolInvocation, error) {
	return startInvocation(provider.store, provider.tool, userID, runID, stepID, input, startedAt)
}

func (provider instrumentedTextProvider) finishInvocation(invocationID uint, startedAt time.Time, output interface{}, err error) {
	finishInvocation(provider.store, invocationID, startedAt, output, provider.tool.Capability.CostPolicy, err)
}

type instrumentedImageGenerationProvider struct {
	tool  Tool
	store InvocationStore
	next  ImageGenerationProvider
}

func (provider instrumentedImageGenerationProvider) GenerateImage(ctx context.Context, request ImageGenerationRequest) (ImageGenerationResult, error) {
	startedAt := time.Now()
	invocation, startErr := startInvocation(provider.store, provider.tool, request.UserID, request.RunID, request.StepID, imageInputSummary(request), startedAt)
	if startErr != nil {
		return ImageGenerationResult{}, startErr
	}
	result, err := provider.next.GenerateImage(ctx, request)
	finishInvocation(provider.store, invocation.ID, startedAt, imageOutputSummary(result), provider.tool.Capability.CostPolicy, err)
	return result, err
}

type instrumentedVisionProvider struct {
	tool  Tool
	store InvocationStore
	next  VisionProvider
}

func (provider instrumentedVisionProvider) AnalyzeImage(ctx context.Context, request VisionRequest) (VisionResult, error) {
	startedAt := time.Now()
	invocation, startErr := startInvocation(provider.store, provider.tool, request.UserID, request.RunID, request.StepID, visionInputSummary(request), startedAt)
	if startErr != nil {
		return VisionResult{}, startErr
	}
	result, err := provider.next.AnalyzeImage(ctx, request)
	finishInvocation(provider.store, invocation.ID, startedAt, visionOutputSummary(result), provider.tool.Capability.CostPolicy, err)
	return result, err
}

func startInvocation(store InvocationStore, tool Tool, userID uint, runID uint, stepID uint, input interface{}, startedAt time.Time) (model.ToolInvocation, error) {
	invocation := model.ToolInvocation{
		AgentRunID:   runID,
		AgentStepID:  stepID,
		UserID:       userID,
		ToolName:     tool.Name,
		ToolKind:     tool.Kind,
		ProviderName: tool.Provider,
		ModelName:    tool.Model,
		Status:       "running",
		InputJSON:    mustJSON(input),
		StartedAt:    int(startedAt.Unix()),
	}
	if err := store.CreateToolInvocation(&invocation); err != nil {
		return model.ToolInvocation{}, fmt.Errorf("create tool invocation: %w", err)
	}
	return invocation, nil
}

func finishInvocation(store InvocationStore, invocationID uint, startedAt time.Time, output interface{}, costPolicy string, err error) {
	if invocationID == 0 {
		return
	}
	completedAt := time.Now()
	attrs := map[string]interface{}{
		"completed_at": int(completedAt.Unix()),
		"duration_ms":  completedAt.Sub(startedAt).Milliseconds(),
		"cost_json":    mustJSON(map[string]string{"policy": strings.TrimSpace(costPolicy)}),
	}
	if err != nil {
		attrs["status"] = "failed"
		attrs["error_message"] = err.Error()
		attrs["error_code"] = classifyProviderError(err)
	} else {
		attrs["status"] = "completed"
		attrs["output_json"] = mustJSON(output)
	}
	_ = store.UpdateToolInvocation(invocationID, attrs)
}

func textInputSummary(request TextRequest) map[string]interface{} {
	return map[string]interface{}{
		"system_chars":  len(request.System),
		"prompt_chars":  len(request.Prompt),
		"message_count": len(request.Messages),
	}
}

func textOutputSummary(result TextResult) map[string]interface{} {
	return map[string]interface{}{
		"text_chars":      len(result.Text),
		"reasoning_chars": len(result.Reasoning),
	}
}

func imageInputSummary(request ImageGenerationRequest) map[string]interface{} {
	return map[string]interface{}{
		"prompt":          request.Prompt,
		"negative_prompt": request.NegativePrompt,
		"aspect_ratio":    request.AspectRatio,
		"candidate_count": request.CandidateCount,
		"task_type":       request.TaskType,
		"intent":          request.Intent,
	}
}

func imageOutputSummary(result ImageGenerationResult) map[string]interface{} {
	images := make([]map[string]interface{}, 0, len(result.Images))
	for _, image := range result.Images {
		images = append(images, map[string]interface{}{
			"name":       image.Name,
			"kind":       image.Kind,
			"mime_type":  image.MimeType,
			"size_bytes": image.SizeBytes,
			"hash":       image.Hash,
		})
	}
	return map[string]interface{}{
		"image_count": len(result.Images),
		"images":      images,
	}
}

func visionInputSummary(request VisionRequest) map[string]interface{} {
	return map[string]interface{}{
		"image_ref_present": strings.TrimSpace(request.ImageRef) != "",
		"prompt_chars":      len(request.Prompt),
	}
}

func visionOutputSummary(result VisionResult) map[string]interface{} {
	return map[string]interface{}{
		"summary":       result.Summary,
		"scores":        result.Scores,
		"issues":        result.Issues,
		"should_refine": result.ShouldRefine,
	}
}

func classifyProviderError(err error) string {
	if err == nil {
		return ""
	}
	if stderrors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return "retryable_provider_error"
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
			return "retryable_provider_error"
		}
	}
	return "provider_error"
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
