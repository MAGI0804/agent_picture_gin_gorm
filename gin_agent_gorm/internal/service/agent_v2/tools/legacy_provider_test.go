package tools

import (
	"context"
	"strings"
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
	if !strings.HasPrefix(image.ObjectKey, "user-7/conversation-8/run-9/objects/") || !strings.HasSuffix(image.ObjectKey, "/poster.png") {
		t.Fatalf("ObjectKey = %q, want randomized scoped key", image.ObjectKey)
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
	if provider.generationRequest.AspectRatio != "16:9" {
		t.Fatalf("AspectRatio = %q, want request ratio", provider.generationRequest.AspectRatio)
	}
}

func TestLegacyProviderAdapterPrefixesMultiCandidateObjectKeys(t *testing.T) {
	provider := &fakeLegacyProvider{
		files: []agent_svc.GeneratedFile{
			{Name: "poster.png", Kind: "image", MimeType: "image/png", Content: []byte("first")},
			{Name: "poster.png", Kind: "image", MimeType: "image/png", Content: []byte("second")},
		},
	}
	adapter := NewLegacyProviderAdapterWithDependencies(provider, &fakeLegacyObjectStore{}, model.UserModelConfig{
		UserID:     7,
		Provider:   "jimeng",
		ImageModel: "image-model",
	})

	result, err := adapter.GenerateImage(context.Background(), ImageGenerationRequest{
		UserID:              7,
		ConversationID:      8,
		RunID:               9,
		Prompt:              "a clean poster",
		CandidateCount:      2,
		CandidateStartIndex: 1,
	})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(result.Images) != 2 {
		t.Fatalf("len(Images) = %d, want 2", len(result.Images))
	}
	if !strings.HasPrefix(result.Images[0].ObjectKey, "user-7/conversation-8/run-9/objects/") ||
		!strings.HasSuffix(result.Images[0].ObjectKey, "/candidate-2-poster.png") {
		t.Fatalf("first ObjectKey = %q, want randomized candidate-indexed key", result.Images[0].ObjectKey)
	}
	if !strings.HasPrefix(result.Images[1].ObjectKey, "user-7/conversation-8/run-9/objects/") ||
		!strings.HasSuffix(result.Images[1].ObjectKey, "/candidate-3-poster.png") {
		t.Fatalf("second ObjectKey = %q, want randomized candidate-indexed key", result.Images[1].ObjectKey)
	}
	if result.Images[0].ObjectKey == result.Images[1].ObjectKey {
		t.Fatalf("candidate object keys should differ: %#v", result.Images)
	}
}

func TestLegacyProviderAdapterEditImageStoresEditedFilesWithSourceRefsInPrompt(t *testing.T) {
	provider := &fakeLegacyProvider{
		files: []agent_svc.GeneratedFile{
			{Name: "edited.png", Kind: "image", MimeType: "image/png", Content: []byte("edited-bytes")},
		},
	}
	store := &fakeLegacyObjectStore{}
	adapter := NewLegacyProviderAdapterWithDependencies(provider, store, model.UserModelConfig{
		UserID:      7,
		Provider:    "google",
		ImageModel:  "imagen-edit",
		Temperature: "0.3",
	})

	result, err := adapter.EditImage(context.Background(), ImageEditRequest{
		UserID:         7,
		ConversationID: 8,
		TaskType:       "image_edit",
		Prompt:         "make the background warmer",
		ImageRefs:      []string{"objects/original.png"},
		CandidateCount: 1,
	})
	if err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("len(Images) = %d, want 1", len(result.Images))
	}
	if !strings.HasPrefix(result.Images[0].ObjectKey, "user-7/conversation-8/manual/edits/") ||
		!strings.HasSuffix(result.Images[0].ObjectKey, "/edited.png") {
		t.Fatalf("ObjectKey = %q, want randomized manual edit scoped key", result.Images[0].ObjectKey)
	}
	if store.objectKey != result.Images[0].ObjectKey || string(store.content) != "edited-bytes" {
		t.Fatalf("stored key/content = %q/%q, want edited image", store.objectKey, string(store.content))
	}
	if provider.generationRequest.TaskType != "image_edit" {
		t.Fatalf("TaskType = %q, want image_edit", provider.generationRequest.TaskType)
	}
	if !strings.Contains(provider.generationRequest.Prompt, "objects/original.png") {
		t.Fatalf("Prompt = %q, want source image reference", provider.generationRequest.Prompt)
	}
	if len(provider.generationRequest.ImageRefs) != 1 || provider.generationRequest.ImageRefs[0] != "objects/original.png" {
		t.Fatalf("ImageRefs = %#v, want source image passed to provider", provider.generationRequest.ImageRefs)
	}
}
