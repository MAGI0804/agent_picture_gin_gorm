package agents

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

func TestSafetyAgentRejectsUnsafePromptBeforeGeneration(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:           "safety",
		Kind:           tools.KindSafety,
		SafetyProvider: fakeSafetyProvider{result: tools.SafetyResult{Allowed: false, Reason: "blocked"}},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := NewSafetyAgent("pre_generation_safety_agent", SafetyPhaseText, registry).Run(context.Background(), domain.RunState{
		UserID:      7,
		UserRequest: "unsafe prompt",
		Prompts:     domain.PromptBundle{PositivePrompt: "unsafe prompt"},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want rejection")
	}
}

func TestSafetyAgentChecksGeneratedImagesAfterGeneration(t *testing.T) {
	provider := &recordingSafetyProvider{result: tools.SafetyResult{Allowed: true, Reason: "allowed"}}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:           "safety",
		Kind:           tools.KindSafety,
		SafetyProvider: provider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	result, err := NewSafetyAgent("post_generation_safety_agent", SafetyPhaseImage, registry).Run(context.Background(), domain.RunState{
		UserID:          7,
		RunID:           9,
		CurrentStepID:   11,
		GeneratedImages: []domain.GeneratedImageRef{{ObjectKey: "private/object.png"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["image_count"] != 1 {
		t.Fatalf("image_count = %#v, want 1", result.Output["image_count"])
	}
	if provider.request.UserID != 7 || provider.request.RunID != 9 || provider.request.StepID != 11 {
		t.Fatalf("safety request identifiers = %#v, want run-scoped identifiers", provider.request)
	}
	if provider.request.ImageRef != "private/object.png" {
		t.Fatalf("ImageRef = %q, want generated object key", provider.request.ImageRef)
	}
}

type fakeSafetyProvider struct {
	result tools.SafetyResult
	err    error
}

func (provider fakeSafetyProvider) CheckContent(ctx context.Context, request tools.SafetyRequest) (tools.SafetyResult, error) {
	if provider.err != nil {
		return tools.SafetyResult{}, provider.err
	}
	return provider.result, nil
}

type recordingSafetyProvider struct {
	result  tools.SafetyResult
	request tools.SafetyRequest
}

func (provider *recordingSafetyProvider) CheckContent(ctx context.Context, request tools.SafetyRequest) (tools.SafetyResult, error) {
	provider.request = request
	return provider.result, nil
}
