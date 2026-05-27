package agents

import (
	"context"
	"strings"
	"testing"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/model"
)

type fakeImageProvider struct {
	request  tools.ImageGenerationRequest
	requests []tools.ImageGenerationRequest
	result   tools.ImageGenerationResult
	results  []tools.ImageGenerationResult
}

func (provider *fakeImageProvider) GenerateImage(
	ctx context.Context,
	request tools.ImageGenerationRequest,
) (tools.ImageGenerationResult, error) {
	provider.request = request
	provider.requests = append(provider.requests, request)
	if len(provider.results) > 0 {
		index := len(provider.requests) - 1
		if index >= len(provider.results) {
			index = len(provider.results) - 1
		}
		return provider.results[index], nil
	}
	return provider.result, nil
}

type fakeTextProvider struct {
	request tools.TextRequest
	result  tools.TextResult
}

func (provider *fakeTextProvider) GenerateText(
	ctx context.Context,
	request tools.TextRequest,
) (tools.TextResult, error) {
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

func (writer *fakeArtifactWriter) CreateRefinedVersion(input artifactsvc.CreateRefinedVersionInput) (model.ArtifactVersion, error) {
	version := input.Image
	version.ID = 99
	version.ArtifactID = input.ArtifactID
	version.ParentVersionID = input.ParentVersionID
	return version, nil
}

func TestRequirementAgentUsesTextProviderStructuredJSON(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{
			Text: `{
				"subject": "coffee launch poster",
				"style": "editorial product photography",
				"aspect_ratio": "9:16",
				"must_include": ["coffee cup", "launch headline"],
				"must_avoid": ["watermark"],
				"need_clarification": false,
				"questions": [],
				"scene": "sunlit cafe counter",
				"composition": "centered cup with headline space",
				"text_policy": "render text separately",
				"layout_hints": ["leave top third empty", "strong product focus"],
				"target_use": "mobile poster"
			}`,
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewRequirementAgentWithText(registry, 7)

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:         3,
		CurrentStepID: 4,
		UserID:        5,
		UserRequest:   "Make a vertical coffee launch poster",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if textProvider.request.UserID != 5 || textProvider.request.RunID != 3 || textProvider.request.StepID != 4 {
		t.Fatalf("text request scope = %#v, want run/user/step scope", textProvider.request)
	}
	if result.Output["subject"] != "coffee launch poster" {
		t.Fatalf("subject = %#v, want provider subject", result.Output["subject"])
	}
	if result.Output["scene"] != "sunlit cafe counter" {
		t.Fatalf("scene = %#v, want provider scene", result.Output["scene"])
	}
	layoutHints, ok := result.Output["layout_hints"].([]string)
	if !ok || len(layoutHints) != 2 {
		t.Fatalf("layout_hints = %#v, want provider hints", result.Output["layout_hints"])
	}
}

func TestRequirementAgentFallsBackOnInvalidProviderJSON(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{Text: "not json"},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewRequirementAgentWithText(registry, 7)

	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "square product photo with logo",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["subject"] != "square product photo with logo" {
		t.Fatalf("subject = %#v, want rule fallback", result.Output["subject"])
	}
	issues, ok := result.Output["schema_issues"].([]string)
	if !ok || len(issues) == 0 {
		t.Fatalf("schema_issues = %#v, want fallback issue", result.Output["schema_issues"])
	}
}

func TestRequirementAgentAsksClarificationForVagueRequest(t *testing.T) {
	agent := NewRequirementAgent()

	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "make an image",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["need_clarification"] != true {
		t.Fatalf("need_clarification = %#v, want true", result.Output["need_clarification"])
	}
	questions, ok := result.Output["questions"].([]string)
	if !ok || len(questions) == 0 {
		t.Fatalf("questions = %#v, want clarification questions", result.Output["questions"])
	}
}

func TestPromptAgentUsesTextProviderStructuredJSONAndMemory(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{
			Text: `{
				"positive_prompt": "premium coffee poster with emerald brand accent",
				"negative_prompt": "blur, watermark",
				"render_text_separately": false,
				"params": {"aspect_ratio": "16:9", "seed_style": "editorial"}
			}`,
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewPromptAgentWithText(registry, 7)

	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "Make a coffee poster",
		Requirements: domain.ImageRequirements{
			Subject:     "coffee poster",
			Style:       "editorial",
			AspectRatio: "16:9",
		},
		MemoryContext: []domain.MemoryItem{
			{Content: "Brand accent color is emerald", Confidence: 0.92},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(textProvider.request.Prompt, "Brand accent color is emerald") {
		t.Fatalf("provider prompt = %q, want memory context", textProvider.request.Prompt)
	}
	if result.Output["positive_prompt"] != "premium coffee poster with emerald brand accent" {
		t.Fatalf("positive_prompt = %#v, want provider prompt", result.Output["positive_prompt"])
	}
	params, ok := result.Output["params"].(map[string]string)
	if !ok || params["seed_style"] != "editorial" {
		t.Fatalf("params = %#v, want provider params", result.Output["params"])
	}
}

func TestPromptAgentFallsBackOnInvalidProviderJSON(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{Text: "not json"},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewPromptAgentWithText(registry, 7)

	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "Make a coffee poster",
		Requirements: domain.ImageRequirements{
			Subject:     "coffee poster",
			Style:       "editorial",
			AspectRatio: "16:9",
			MustAvoid:   []string{"blur"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output["positive_prompt"].(string), "coffee poster") {
		t.Fatalf("positive_prompt = %#v, want rule fallback", result.Output["positive_prompt"])
	}
	issues, ok := result.Output["schema_issues"].([]string)
	if !ok || len(issues) == 0 {
		t.Fatalf("schema_issues = %#v, want fallback issue", result.Output["schema_issues"])
	}
}

func TestPromptAgentSetsRenderTextSeparatelyForChinesePoster(t *testing.T) {
	agent := NewPromptAgent()

	result, err := agent.Run(context.Background(), domain.RunState{
		UserRequest: "制作中文咖啡新品海报，标题是夏日上新",
		Requirements: domain.ImageRequirements{
			Subject:     "中文咖啡新品海报，标题是夏日上新",
			Style:       "poster design",
			AspectRatio: "9:16",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["render_text_separately"] != true {
		t.Fatalf("render_text_separately = %#v, want true for Chinese poster text", result.Output["render_text_separately"])
	}
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
		CandidateCount:     1,
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
	if provider.request.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1", provider.request.CandidateCount)
	}
	images, ok := result.Output["generated_images"].([]domain.GeneratedImageRef)
	if !ok {
		t.Fatalf("generated_images type = %T, want []domain.GeneratedImageRef", result.Output["generated_images"])
	}
	if len(images) != 1 || images[0].ObjectKey != "object-key" {
		t.Fatalf("generated images = %#v, want object-key", images)
	}
}

func TestImageGenerationAgentTopsUpMissingCandidatesWithAdditionalProviderCalls(t *testing.T) {
	provider := &fakeImageProvider{
		results: []tools.ImageGenerationResult{
			{Images: []tools.GeneratedImage{{ObjectKey: "object-key-1"}}},
			{Images: []tools.GeneratedImage{{ObjectKey: "object-key-2"}}},
			{Images: []tools.GeneratedImage{{ObjectKey: "object-key-3"}}},
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
		CandidateCount:     3,
	})

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:          3,
		UserID:         4,
		ConversationID: 5,
		Prompts: domain.PromptBundle{
			PositivePrompt: "bright product photo",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(provider.requests) != 3 {
		t.Fatalf("provider calls = %d, want 3", len(provider.requests))
	}
	for index, want := range []int{3, 2, 1} {
		if provider.requests[index].CandidateCount != want {
			t.Fatalf("request %d candidate count = %d, want %d", index, provider.requests[index].CandidateCount, want)
		}
		if provider.requests[index].CandidateStartIndex != index {
			t.Fatalf("request %d candidate start index = %d, want %d", index, provider.requests[index].CandidateStartIndex, index)
		}
	}
	images, ok := result.Output["generated_images"].([]domain.GeneratedImageRef)
	if !ok {
		t.Fatalf("generated_images type = %T, want []domain.GeneratedImageRef", result.Output["generated_images"])
	}
	if len(images) != 3 {
		t.Fatalf("generated images = %#v, want 3 candidates", images)
	}
	if images[1].ObjectKey != "object-key-2" || images[2].ObjectKey != "object-key-3" {
		t.Fatalf("generated images = %#v, want provider outputs in order", images)
	}
}

func TestImageGenerationAgentAppliesToolCapabilityLimits(t *testing.T) {
	provider := &fakeImageProvider{
		result: tools.ImageGenerationResult{
			Images: []tools.GeneratedImage{{ObjectKey: "object-key"}},
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "image-model",
		Kind:          tools.KindImageGeneration,
		ModelConfigID: 42,
		Capability: tools.Capability{
			MaxPromptChars:  24,
			SupportedRatios: []string{"1:1"},
			MaxCandidates:   1,
		},
		ImageGenerationProvider: provider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	agent := NewImageGenerationAgent(registry, ImageGenerationAgentOptions{
		ImageModelConfigID: 42,
		CandidateCount:     5,
	})

	_, err := agent.Run(context.Background(), domain.RunState{
		Requirements: domain.ImageRequirements{
			AspectRatio: "9:16",
		},
		Prompts: domain.PromptBundle{
			PositivePrompt: strings.Repeat("detailed product prompt ", 4),
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := len([]rune(provider.request.Prompt)); got > 24 {
		t.Fatalf("prompt rune length = %d, want <= 24; prompt = %q", got, provider.request.Prompt)
	}
	if provider.request.AspectRatio != "1:1" {
		t.Fatalf("AspectRatio = %q, want supported ratio fallback", provider.request.AspectRatio)
	}
	if provider.request.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want capability max", provider.request.CandidateCount)
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

func TestArtifactAgentCreatesIndependentVersionsForThreeCandidates(t *testing.T) {
	writer := &fakeArtifactWriter{}
	agent := NewArtifactAgent(writer, ArtifactAgentOptions{
		ModelProvider: "google",
		ModelName:     "imagen",
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
		},
		GeneratedImages: []domain.GeneratedImageRef{
			{Name: "candidate-1.png", Kind: "image", MimeType: "image/png", ObjectKey: "key-1"},
			{Name: "candidate-2.png", Kind: "image", MimeType: "image/png", ObjectKey: "key-2"},
			{Name: "candidate-3.png", Kind: "image", MimeType: "image/png", ObjectKey: "key-3"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(writer.input.Artifacts) != 3 {
		t.Fatalf("len(writer.input.Artifacts) = %d, want 3", len(writer.input.Artifacts))
	}
	if len(result.Artifacts) != 3 {
		t.Fatalf("len(result.Artifacts) = %d, want 3", len(result.Artifacts))
	}
	for index, candidate := range writer.input.Artifacts {
		if candidate.Artifact.ObjectKey == "" || candidate.Version.ObjectKey == "" {
			t.Fatalf("candidate %d did not persist artifact and version object keys: %#v", index, candidate)
		}
		if candidate.Artifact.ObjectKey != candidate.Version.ObjectKey {
			t.Fatalf("candidate %d artifact/version object keys differ: %#v", index, candidate)
		}
	}
	if result.Artifacts[1].ID != 11 || result.Artifacts[1].VersionID != 21 {
		t.Fatalf("second artifact ref = %#v, want independent second artifact/version", result.Artifacts[1])
	}
}
