package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gin-biz-web-api/model"
)

func TestInstrumentToolRecordsImageGenerationInvocation(t *testing.T) {
	provider := &fakeTracingImageProvider{
		result: ImageGenerationResult{
			Images: []GeneratedImage{
				{Name: "poster.png", ObjectKey: "private/object.png", SizeBytes: 123, Hash: "hash"},
			},
		},
	}
	store := &fakeInvocationStore{}
	tool := InstrumentTool(Tool{
		Name:                    "imagen",
		Kind:                    KindImageGeneration,
		Provider:                "google",
		Model:                   "imagen-3",
		Capability:              Capability{CostPolicy: "real_provider"},
		ImageGenerationProvider: provider,
	}, store)

	result, err := tool.ImageGenerationProvider.GenerateImage(context.Background(), ImageGenerationRequest{
		UserID:         7,
		RunID:          9,
		StepID:         11,
		Prompt:         "clean poster",
		NegativePrompt: "blur",
		AspectRatio:    "16:9",
		CandidateCount: 1,
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(Images) = %d, want 1", len(result.Images))
	}
	if store.created.AgentRunID != 9 || store.created.AgentStepID != 11 || store.created.UserID != 7 {
		t.Fatalf("created invocation identifiers = %#v", store.created)
	}
	if store.created.ToolName != "imagen" || store.created.ProviderName != "google" || store.created.ModelName != "imagen-3" {
		t.Fatalf("created invocation tool metadata = %#v", store.created)
	}
	if !strings.Contains(store.created.InputJSON, "clean poster") {
		t.Fatalf("InputJSON = %q, want prompt summary", store.created.InputJSON)
	}
	if store.updated["status"] != "completed" {
		t.Fatalf("updated status = %#v, want completed", store.updated["status"])
	}
	if strings.Contains(store.updated["output_json"].(string), "private/object.png") {
		t.Fatalf("OutputJSON leaked object key: %s", store.updated["output_json"])
	}
}

func TestInstrumentToolRecordsProviderFailure(t *testing.T) {
	provider := &fakeTracingImageProvider{err: errors.New("provider timeout")}
	store := &fakeInvocationStore{}
	tool := InstrumentTool(Tool{
		Name:                    "imagen",
		Kind:                    KindImageGeneration,
		Provider:                "google",
		Model:                   "imagen-3",
		ImageGenerationProvider: provider,
	}, store)

	_, err := tool.ImageGenerationProvider.GenerateImage(context.Background(), ImageGenerationRequest{
		UserID: 7,
		RunID:  9,
		StepID: 11,
		Prompt: "clean poster",
	})
	if err == nil {
		t.Fatal("GenerateImage() error = nil, want provider error")
	}
	if store.updated["status"] != "failed" {
		t.Fatalf("updated status = %#v, want failed", store.updated["status"])
	}
	if store.updated["error_code"] != "retryable_provider_error" {
		t.Fatalf("error_code = %#v, want retryable_provider_error", store.updated["error_code"])
	}
}

func TestInstrumentToolRecordsSafetyWithoutPromptLeak(t *testing.T) {
	provider := &fakeTracingSafetyProvider{result: SafetyResult{Allowed: true, Reason: "allowed"}}
	store := &fakeInvocationStore{}
	tool := InstrumentTool(Tool{
		Name:           "safety",
		Kind:           KindSafety,
		Provider:       "local",
		Model:          "policy-v1",
		SafetyProvider: provider,
	}, store)

	_, err := tool.SafetyProvider.CheckContent(context.Background(), SafetyRequest{
		UserID: 7,
		RunID:  9,
		StepID: 11,
		Text:   "very sensitive prompt with token abc",
	})
	if err != nil {
		t.Fatalf("CheckContent() error = %v", err)
	}
	if store.created.AgentRunID != 9 || store.created.AgentStepID != 11 || store.created.UserID != 7 {
		t.Fatalf("created invocation identifiers = %#v", store.created)
	}
	if strings.Contains(store.created.InputJSON, "sensitive prompt") || strings.Contains(store.created.InputJSON, "token abc") {
		t.Fatalf("InputJSON leaked safety text: %s", store.created.InputJSON)
	}
	if store.updated["status"] != "completed" {
		t.Fatalf("updated status = %#v, want completed", store.updated["status"])
	}
}

type fakeTracingImageProvider struct {
	result ImageGenerationResult
	err    error
}

type fakeTracingSafetyProvider struct {
	result SafetyResult
	err    error
}

func (provider *fakeTracingSafetyProvider) CheckContent(ctx context.Context, request SafetyRequest) (SafetyResult, error) {
	if provider.err != nil {
		return SafetyResult{}, provider.err
	}
	return provider.result, nil
}

func (provider *fakeTracingImageProvider) GenerateImage(ctx context.Context, request ImageGenerationRequest) (ImageGenerationResult, error) {
	if provider.err != nil {
		return ImageGenerationResult{}, provider.err
	}
	return provider.result, nil
}

type fakeInvocationStore struct {
	created model.ToolInvocation
	updated map[string]interface{}
}

func (store *fakeInvocationStore) CreateToolInvocation(invocation *model.ToolInvocation) error {
	invocation.ID = 1
	store.created = *invocation
	return nil
}

func (store *fakeInvocationStore) UpdateToolInvocation(invocationID uint, attrs map[string]interface{}) error {
	store.updated = attrs
	return nil
}
