package tools

import (
	"context"
	"fmt"
	"path"
	"strings"

	"gin-biz-web-api/internal/service/agent_svc"
	agentsecurity "gin-biz-web-api/internal/service/agent_v2/security"
	"gin-biz-web-api/model"
)

// LegacyProvider is the subset of the old provider needed by V2 tools.
type LegacyProvider interface {
	Chat(request agent_svc.ChatRequest) (agent_svc.ChatResult, error)
	Generate(request agent_svc.GenerationRequest) ([]agent_svc.GeneratedFile, error)
}

// LegacyObjectStore stores generated files and returns object metadata.
type LegacyObjectStore interface {
	Save(objectKey string, content []byte) (agent_svc.StoredObject, error)
}

// LegacyProviderAdapter exposes the old provider through V2 tool interfaces.
type LegacyProviderAdapter struct {
	provider LegacyProvider
	store    LegacyObjectStore
	config   model.UserModelConfig
}

// NewLegacyProviderAdapter creates a V2 adapter backed by the existing provider and object store.
func NewLegacyProviderAdapter(config model.UserModelConfig) *LegacyProviderAdapter {
	return NewLegacyProviderAdapterWithDependencies(
		agent_svc.NewProviderWithConfig(config),
		agent_svc.NewObjectStore(),
		config,
	)
}

// NewLegacyProviderAdapterWithDependencies creates an adapter with injectable dependencies for tests.
func NewLegacyProviderAdapterWithDependencies(
	provider LegacyProvider,
	store LegacyObjectStore,
	config model.UserModelConfig,
) *LegacyProviderAdapter {
	return &LegacyProviderAdapter{
		provider: provider,
		store:    store,
		config:   config,
	}
}

// GenerateText implements TextProvider by delegating to the old Chat provider.
func (adapter *LegacyProviderAdapter) GenerateText(
	ctx context.Context,
	request TextRequest,
) (TextResult, error) {
	if err := ctx.Err(); err != nil {
		return TextResult{}, err
	}
	messages := make([]agent_svc.ChatMessage, 0, len(request.Messages)+1)
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, agent_svc.ChatMessage{Role: role, Content: content})
	}
	if len(messages) == 0 {
		messages = append(messages, agent_svc.ChatMessage{
			Role:    "user",
			Content: strings.TrimSpace(request.Prompt),
		})
	}
	result, err := adapter.provider.Chat(agent_svc.ChatRequest{
		System:          strings.TrimSpace(request.System),
		Messages:        messages,
		ModelConfig:     adapter.config,
		Stream:          false,
		ReturnReasoning: true,
	})
	if err != nil {
		return TextResult{}, err
	}
	return TextResult{
		Text:      strings.TrimSpace(result.Content),
		Reasoning: strings.TrimSpace(result.ReasoningContent),
	}, nil
}

// GenerateImage implements ImageGenerationProvider by delegating to the old image provider.
func (adapter *LegacyProviderAdapter) GenerateImage(
	ctx context.Context,
	request ImageGenerationRequest,
) (ImageGenerationResult, error) {
	if err := ctx.Err(); err != nil {
		return ImageGenerationResult{}, err
	}
	files, err := adapter.provider.Generate(agent_svc.GenerationRequest{
		Prompt:          strings.TrimSpace(request.Prompt),
		Intent:          request.Intent,
		TaskType:        request.TaskType,
		AspectRatio:     request.AspectRatio,
		Stream:          true,
		ReturnReasoning: true,
		Temperature:     coalesceString(request.Temperature, adapter.config.Temperature),
		ModelConfig:     adapter.config,
	})
	if err != nil {
		return ImageGenerationResult{}, err
	}
	images := make([]GeneratedImage, 0, len(files))
	for index, file := range files {
		name := safeGeneratedName(file.Name, index)
		objectKey := generatedObjectKey(request, name, index)
		stored, err := adapter.store.Save(objectKey, file.Content)
		if err != nil {
			return ImageGenerationResult{}, err
		}
		images = append(images, GeneratedImage{
			Name:       name,
			Kind:       coalesceString(file.Kind, "image"),
			MimeType:   coalesceString(file.MimeType, "application/octet-stream"),
			ObjectKey:  stored.ObjectKey,
			PreviewURL: stored.PreviewURL,
			SizeBytes:  stored.SizeBytes,
			Hash:       stored.Hash,
		})
	}
	return ImageGenerationResult{Images: images}, nil
}

// EditImage implements ImageEditProvider. The current legacy provider has no native image-input
// field, so this adapter preserves the interface and includes source references in the prompt.
func (adapter *LegacyProviderAdapter) EditImage(
	ctx context.Context,
	request ImageEditRequest,
) (ImageEditResult, error) {
	if err := ctx.Err(); err != nil {
		return ImageEditResult{}, err
	}
	prompt := strings.TrimSpace(request.Prompt)
	if len(request.ImageRefs) > 0 {
		prompt = strings.TrimSpace(fmt.Sprintf(
			"%s\n\nReference image object keys: %s\nEdit the referenced image while preserving the requested subject identity and composition unless the prompt says otherwise.",
			prompt,
			strings.Join(request.ImageRefs, ", "),
		))
	}
	files, err := adapter.provider.Generate(agent_svc.GenerationRequest{
		Prompt:          prompt,
		Intent:          "image_edit",
		TaskType:        coalesceString(request.TaskType, "image_edit"),
		ImageRefs:       append([]string{}, request.ImageRefs...),
		Stream:          true,
		ReturnReasoning: true,
		Temperature:     adapter.config.Temperature,
		ModelConfig:     adapter.config,
	})
	if err != nil {
		return ImageEditResult{}, err
	}
	images := make([]GeneratedImage, 0, len(files))
	for index, file := range files {
		name := safeGeneratedName(file.Name, index)
		objectKey := editObjectKey(request, name, index)
		stored, err := adapter.store.Save(objectKey, file.Content)
		if err != nil {
			return ImageEditResult{}, err
		}
		images = append(images, GeneratedImage{
			Name:       name,
			Kind:       coalesceString(file.Kind, "image"),
			MimeType:   coalesceString(file.MimeType, "application/octet-stream"),
			ObjectKey:  stored.ObjectKey,
			PreviewURL: stored.PreviewURL,
			SizeBytes:  stored.SizeBytes,
			Hash:       stored.Hash,
		})
	}
	return ImageEditResult{Images: images}, nil
}

func safeGeneratedName(name string, index int) string {
	name = agent_svc.SafeDownloadName(strings.TrimSpace(name))
	if name == "" || name == "." {
		return fmt.Sprintf("generated-image-%d.png", index+1)
	}
	return name
}

func generatedObjectKey(request ImageGenerationRequest, name string, index int) string {
	if request.CandidateCount > 1 || request.CandidateStartIndex > 0 {
		name = fmt.Sprintf("candidate-%d-%s", request.CandidateStartIndex+index+1, name)
	}
	return path.Join(
		fmt.Sprintf("user-%d", request.UserID),
		fmt.Sprintf("conversation-%d", request.ConversationID),
		fmt.Sprintf("run-%d", request.RunID),
		"objects",
		agentsecurity.RandomObjectKeyPart(),
		name,
	)
}

func editObjectKey(request ImageEditRequest, name string, index int) string {
	if request.CandidateCount > 1 {
		name = fmt.Sprintf("edit-%d-%s", index+1, name)
	}
	runPart := "manual"
	if request.RunID > 0 {
		runPart = fmt.Sprintf("run-%d", request.RunID)
	}
	return path.Join(
		fmt.Sprintf("user-%d", request.UserID),
		fmt.Sprintf("conversation-%d", request.ConversationID),
		runPart,
		"edits",
		agentsecurity.RandomObjectKeyPart(),
		name,
	)
}

func coalesceString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}
