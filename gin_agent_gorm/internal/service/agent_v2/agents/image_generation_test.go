package agents

import (
	"context"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/model"
)

type fakeImageProvider struct {
	request tools.ImageGenerationRequest
	result  tools.ImageGenerationResult
}

func (provider *fakeImageProvider) GenerateImage(
	ctx context.Context,
	request tools.ImageGenerationRequest,
) (tools.ImageGenerationResult, error) {
	provider.request = request
	return provider.result, nil
}

type fakeArtifactWriter struct {
	input    CreateCandidateGroupInput
	versions []model.ArtifactVersion
}

func (writer *fakeArtifactWriter) CreateCandidateGroup(
	input CreateCandidateGroupInput,
) ([]model.Artifact, []model.ArtifactVersion, error) {
	writer.input = input
	artifacts := make([]model.Artifact, 0, len(input.Artifacts))
	for index, candidate := range input.Artifacts {
		artifact := candidate.Artifact
		artifact.ID = uint(index + 10)
		artifacts = append(artifacts, artifact)
	}
	if len(writer.versions) > 0 {
		return artifacts, writer.versions, nil
	}
	versions := make([]model.ArtifactVersion, 0, len(input.Artifacts))
	for index := range input.Artifacts {
		versions = append(versions, model.ArtifactVersion{BaseModel: model.BaseModel{ID: uint(index + 20)}})
	}
	return artifacts, versions, nil
}

func TestImageGenerationAgentUsesRegisteredImageTool(t *testing.T) {
	provider := &fakeImageProvider{
		result: tools.ImageGenerationResult{
			Images: []tools.GeneratedImage{
				{
					Name:       "image.png",
					Kind:       "image",
					MimeType:   "image/png",
					ObjectKey:  "object-key",
					PreviewURL: "/artifacts/object-key",
					SizeBytes:  12,
					Hash:       "hash",
				},
			},
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:                    "image-model",
		Kind:                    tools.KindImageGeneration,
		ModelConfigID:           42,
		ImageGenerationProvider: provider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewImageGenerationAgent(registry, ImageGenerationAgentOptions{
		ImageModelConfigID: 42,
		CandidateCount:     2,
	})

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:          3,
		UserID:         4,
		ConversationID: 5,
		TaskType:       "image_generation",
		Requirements: domain.ImageRequirements{
			AspectRatio: "16:9",
		},
		Prompts: domain.PromptBundle{
			PositivePrompt: "bright product photo",
			NegativePrompt: "blur",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.request.Prompt != "bright product photo" {
		t.Fatalf("Prompt = %q, want positive prompt", provider.request.Prompt)
	}
	if provider.request.NegativePrompt != "blur" {
		t.Fatalf("NegativePrompt = %q, want blur", provider.request.NegativePrompt)
	}
	if provider.request.CandidateCount != 2 {
		t.Fatalf("CandidateCount = %d, want 2", provider.request.CandidateCount)
	}
	images, ok := result.Output["generated_images"].([]domain.GeneratedImageRef)
	if !ok {
		t.Fatalf("generated_images type = %T, want []domain.GeneratedImageRef", result.Output["generated_images"])
	}
	if len(images) != 1 || images[0].ObjectKey != "object-key" {
		t.Fatalf("generated images = %#v, want object-key", images)
	}
}

func TestArtifactAgentCreatesArtifactVersionsFromGeneratedImages(t *testing.T) {
	writer := &fakeArtifactWriter{}
	agent := NewArtifactAgent(writer, ArtifactAgentOptions{
		ModelProvider: "jimeng",
		ModelName:     "seedream",
	})

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:          3,
		UserID:         4,
		ConversationID: 5,
		Requirements: domain.ImageRequirements{
			AspectRatio: "16:9",
		},
		Prompts: domain.PromptBundle{
			PositivePrompt: "poster",
			NegativePrompt: "blur",
		},
		GeneratedImages: []domain.GeneratedImageRef{
			{
				Name:       "poster.png",
				Kind:       "image",
				MimeType:   "image/png",
				ObjectKey:  "key",
				PreviewURL: "/artifacts/key",
				SizeBytes:  100,
				Hash:       "hash",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if writer.input.AgentRunID != 3 || writer.input.UserID != 4 || writer.input.ConversationID != 5 {
		t.Fatalf("writer input scope = %#v, want run/user/conversation scope", writer.input)
	}
	if len(writer.input.Artifacts) != 1 {
		t.Fatalf("len(writer.input.Artifacts) = %d, want 1", len(writer.input.Artifacts))
	}
	version := writer.input.Artifacts[0].Version
	if version.Prompt != "poster" || version.NegativePrompt != "blur" {
		t.Fatalf("version prompts = %#v, want prompt metadata", version)
	}
	if version.ModelProvider != "jimeng" || version.ModelName != "seedream" {
		t.Fatalf("version model = %s/%s, want jimeng/seedream", version.ModelProvider, version.ModelName)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("len(result.Artifacts) = %d, want 1", len(result.Artifacts))
	}
	if result.Artifacts[0].ID != 10 || result.Artifacts[0].VersionID != 20 {
		t.Fatalf("artifact refs = %#v, want created artifact and version ids", result.Artifacts)
	}
}
