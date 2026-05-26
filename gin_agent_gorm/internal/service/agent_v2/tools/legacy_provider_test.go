package tools

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/model"
)

type fakeLegacyProvider struct {
	chatRequest       agent_svc.ChatRequest
	generationRequest agent_svc.GenerationRequest
	chatResult        agent_svc.ChatResult
	files             []agent_svc.GeneratedFile
}

func (provider *fakeLegacyProvider) Chat(request agent_svc.ChatRequest) (agent_svc.ChatResult, error) {
	provider.chatRequest = request
	return provider.chatResult, nil
}

func (provider *fakeLegacyProvider) Generate(request agent_svc.GenerationRequest) ([]agent_svc.GeneratedFile, error) {
	provider.generationRequest = request
	return provider.files, nil
}

type fakeLegacyObjectStore struct {
	objectKey string
	content   []byte
}

func (store *fakeLegacyObjectStore) Save(objectKey string, content []byte) (agent_svc.StoredObject, error) {
	store.objectKey = objectKey
	store.content = append([]byte{}, content...)
	return agent_svc.StoredObject{
		ObjectKey:  objectKey,
		PreviewURL: "/artifacts/" + objectKey,
		SizeBytes:  int64(len(content)),
		Hash:       "sha256",
	}, nil
}

func TestLegacyProviderAdapterGenerateTextDelegatesToChat(t *testing.T) {
	provider := &fakeLegacyProvider{
		chatResult: agent_svc.ChatResult{
			Content:          "structured prompt",
			ReasoningContent: "reasoning",
		},
	}
	adapter := NewLegacyProviderAdapterWithDependencies(provider, &fakeLegacyObjectStore{}, model.UserModelConfig{
		UserID:      7,
		Provider:    "openai-compatible",
		ChatModel:   "text-model",
		Temperature: "0.4",
	})

	result, err := adapter.GenerateText(context.Background(), TextRequest{
		System: "system prompt",
		Prompt: "user prompt",
	})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if result.Text != "structured prompt" {
		t.Fatalf("Text = %q, want %q", result.Text, "structured prompt")
	}
	if result.Reasoning != "reasoning" {
		t.Fatalf("Reasoning = %q, want %q", result.Reasoning, "reasoning")
	}
	if provider.chatRequest.System != "system prompt" {
		t.Fatalf("System = %q, want %q", provider.chatRequest.System, "system prompt")
	}
	if len(provider.chatRequest.Messages) != 1 || provider.chatRequest.Messages[0].Content != "user prompt" {
		t.Fatalf("Messages = %#v, want one user prompt", provider.chatRequest.Messages)
	}
}

func TestLegacyProviderAdapterGenerateImageStoresGeneratedFiles(t *testing.T) {
	provider := &fakeLegacyProvider{
		files: []agent_svc.GeneratedFile{
			{
				Name:     "../poster.png",
				Kind:     "image",
				MimeType: "image/png",
				Content:  []byte("png-bytes"),
			},
		},
	}
	store := &fakeLegacyObjectStore{}
	adapter := NewLegacyProviderAdapterWithDependencies(provider, store, model.UserModelConfig{
		UserID:      7,
		Provider:    "jimeng",
		ImageModel:  "image-model",
		Temperature: "0.4",
	})

	result, err := adapter.GenerateImage(context.Background(), ImageGenerationRequest{
		UserID:         7,
		ConversationID: 8,
		RunID:          9,
		Prompt:         "a clean poster",
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
	image := result.Images[0]
	if image.Name != "poster.png" {
		t.Fatalf("Name = %q, want sanitized filename", image.Name)
	}
	if image.ObjectKey != "user-7/conversation-8/run-9/poster.png" {
		t.Fatalf("ObjectKey = %q, want scoped key", image.ObjectKey)
	}
	if image.SizeBytes != int64(len("png-bytes")) {
		t.Fatalf("SizeBytes = %d, want %d", image.SizeBytes, len("png-bytes"))
	}
	if store.objectKey != image.ObjectKey {
		t.Fatalf("stored key = %q, want %q", store.objectKey, image.ObjectKey)
	}
	if string(store.content) != "png-bytes" {
		t.Fatalf("stored content = %q, want image bytes", string(store.content))
	}
	if provider.generationRequest.Prompt != "a clean poster" {
		t.Fatalf("Prompt = %q, want %q", provider.generationRequest.Prompt, "a clean poster")
	}
}
