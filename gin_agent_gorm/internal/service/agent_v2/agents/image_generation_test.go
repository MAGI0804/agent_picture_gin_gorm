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
	err     error
}

func (provider *fakeTextProvider) GenerateText(
	ctx context.Context,
	request tools.TextRequest,
) (tools.TextResult, error) {
	provider.request = request
	if provider.err != nil {
		return tools.TextResult{}, provider.err
	}
	return provider.result, nil
}

type fakeImageEditProvider struct {
	request tools.ImageEditRequest
	result  tools.ImageEditResult
}

func (provider *fakeImageEditProvider) EditImage(
	ctx context.Context,
	request tools.ImageEditRequest,
) (tools.ImageEditResult, error) {
	provider.request = request
	return provider.result, nil
}

type fakeArtifactWriter struct {
	input     CreateCandidateGroupInput
	versions  []model.ArtifactVersion
	render    artifactsvc.CreateRenderedArtifactInput
	artifacts []model.Artifact
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

func (writer *fakeArtifactWriter) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return writer.artifacts, nil
}

func (writer *fakeArtifactWriter) CreateRefinedVersion(input artifactsvc.CreateRefinedVersionInput) (model.ArtifactVersion, error) {
	version := input.Image
	version.ID = 99
	version.ArtifactID = input.ArtifactID
	version.ParentVersionID = input.ParentVersionID
	return version, nil
}

func (writer *fakeArtifactWriter) CreateRenderedArtifact(input artifactsvc.CreateRenderedArtifactInput) (model.Artifact, model.ArtifactVersion, error) {
	writer.render = input
	return model.Artifact{
			BaseModel:        model.BaseModel{ID: 30},
			UserID:           input.UserID,
			ConversationID:   input.ConversationID,
			AgentRunID:       input.AgentRunID,
			Name:             input.Name,
			Kind:             input.Kind,
			MimeType:         input.MimeType,
			PreviewURL:       "/api/v2/artifacts/30/preview",
			ParentArtifactID: input.ParentArtifactID,
		}, model.ArtifactVersion{
			BaseModel:       model.BaseModel{ID: 31},
			ArtifactID:      30,
			ParentVersionID: input.ParentVersionID,
			Operation:       input.Operation,
		}, nil
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

func TestRequirementAgentRequiredTextReturnsProviderError(t *testing.T) {
	textProvider := &fakeTextProvider{err: context.DeadlineExceeded}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := NewRequirementAgentWithRequiredText(registry, 7).Run(context.Background(), domain.RunState{
		UserRequest: "把这张模板图加上标题和图标",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want provider error")
	}
	if !strings.Contains(err.Error(), "requirement text provider failed") {
		t.Fatalf("Run() error = %v, want provider failure surfaced", err)
	}
}

func TestRequirementAgentAcceptsProviderListFieldsAsStrings(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{
			Text: `{
				"subject": "children fashion model on white background",
				"style": "commercial catalog photography",
				"aspect_ratio": "3:4",
				"must_include": "pink dress child model",
				"must_avoid": "watermark, blur",
				"need_clarification": false,
				"questions": "",
				"scene": "studio portrait",
				"composition": "full body model with clean background",
				"text_policy": "render text separately",
				"layout_hints": "leave right side clean\navoid covering the dress",
				"target_use": "kids clothing catalog"
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
	result, err := NewRequirementAgentWithText(registry, 7).Run(context.Background(), domain.RunState{
		UserRequest: "children clothing model, white background, pink dress",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["aspect_ratio"] != "3:4" {
		t.Fatalf("aspect_ratio = %#v, want 3:4", result.Output["aspect_ratio"])
	}
	hints, ok := result.Output["layout_hints"].([]string)
	if !ok || len(hints) != 2 {
		t.Fatalf("layout_hints = %#v, want split string hints", result.Output["layout_hints"])
	}
}

func TestRequirementAgentAcceptsProviderClarificationBoolAsString(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{
			Text: `{
				"subject": "template edit with exact text",
				"style": "commercial layout edit",
				"aspect_ratio": "3:4",
				"must_include": ["Do Small Things"],
				"must_avoid": ["blur"],
				"need_clarification": "false",
				"questions": [],
				"scene": "existing clothing template",
				"composition": "preserve template",
				"text_policy": "render text directly",
				"layout_hints": ["50px from top"],
				"target_use": "product poster"
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

	result, err := NewRequirementAgentWithRequiredText(registry, 7).Run(context.Background(), domain.RunState{
		UserRequest: "add exact text to template",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["need_clarification"] != false {
		t.Fatalf("need_clarification = %#v, want false", result.Output["need_clarification"])
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

func TestRequirementAgentAsksTargetedClarificationQuestions(t *testing.T) {
	result, err := NewRequirementAgent().Run(context.Background(), domain.RunState{
		UserRequest: "white background",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	questions, ok := result.Output["questions"].([]string)
	if !ok || len(questions) == 0 {
		t.Fatalf("questions = %#v, want targeted clarification", result.Output["questions"])
	}
	if !strings.Contains(questions[0], "\u4e3b\u4f53") {
		t.Fatalf("first question = %q, want subject-specific question", questions[0])
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

func TestPromptAgentPreservesExactReferenceEditInstructions(t *testing.T) {
	textProvider := &fakeTextProvider{
		result: tools.TextResult{
			Text: `{
				"positive_prompt": "Professional e-commerce product photography of a children's POLO vest.",
				"negative_prompt": "cluttered background, text, watermark, logo, blurry",
				"render_text_separately": true,
				"params": {"aspect_ratio": "3:4"}
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

	result, err := NewPromptAgentWithRequiredText(registry, 7).Run(context.Background(), domain.RunState{
		UserRequest: "左上角文字内容：Do Small Things\n字体颜色：灰色 #b6b6b4\n左下角 LOGO 设置",
		Requirements: domain.ImageRequirements{
			Subject:           "template edit",
			TextPolicy:        "render text directly",
			LayoutHints:       []string{"距左边缘 50px、距上边缘 50px"},
			AspectRatio:       "3:4",
			MustInclude:       []string{"Do Small Things", "LOGO"},
			MustAvoid:         []string{"blur"},
			NeedClarification: false,
		},
		Metadata: map[string]string{
			"input_artifact_ids": "[54,55]",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	positive := result.Output["positive_prompt"].(string)
	if !strings.Contains(positive, "Do Small Things") || !strings.Contains(positive, "#b6b6b4") || !strings.Contains(positive, "50px") {
		t.Fatalf("positive_prompt = %q, want exact original constraints preserved", positive)
	}
	negative := strings.ToLower(result.Output["negative_prompt"].(string))
	if strings.Contains(negative, "text") || strings.Contains(negative, "logo") {
		t.Fatalf("negative_prompt = %q, should not reject requested text/logo", negative)
	}
	if result.Output["render_text_separately"] != false {
		t.Fatalf("render_text_separately = %#v, want false for bitmap edit", result.Output["render_text_separately"])
	}
}

func TestPromptAgentRequiredTextReturnsProviderError(t *testing.T) {
	textProvider := &fakeTextProvider{err: context.DeadlineExceeded}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "text-model",
		Kind:          tools.KindText,
		ModelConfigID: 7,
		TextProvider:  textProvider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := NewPromptAgentWithRequiredText(registry, 7).Run(context.Background(), domain.RunState{
		UserRequest: "把这张模板图加上标题和图标",
		Requirements: domain.ImageRequirements{
			Subject: "模板图编辑",
		},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want provider error")
	}
	if !strings.Contains(err.Error(), "prompt text provider failed") {
		t.Fatalf("Run() error = %v, want provider failure surfaced", err)
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

func TestImageEditAgentUsesSelectedUploadedArtifactsAndPersistsAIEdit(t *testing.T) {
	provider := &fakeImageEditProvider{
		result: tools.ImageEditResult{
			Images: []tools.GeneratedImage{
				{
					Name:       "edited.png",
					Kind:       "image",
					MimeType:   "image/png",
					ObjectKey:  "edited-key",
					PreviewURL: "/artifacts/edited-key",
					SizeBytes:  25,
					Hash:       "edited-hash",
				},
			},
		},
	}
	registry := tools.NewRegistry()
	if err := registry.Register(tools.Tool{
		Name:          "image-edit-model",
		Kind:          tools.KindImageEdit,
		ModelConfigID: 42,
		Capability: tools.Capability{
			MaxCandidates:      1,
			SupportsImageInput: true,
		},
		ImageEditProvider: provider,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	writer := &fakeArtifactWriter{
		artifacts: []model.Artifact{
			{BaseModel: model.BaseModel{ID: 51}, UserID: 4, ConversationID: 5, AgentRunID: 0, Kind: "image", ObjectKey: "template-key"},
			{BaseModel: model.BaseModel{ID: 52}, UserID: 4, ConversationID: 5, AgentRunID: 0, Kind: "image", ObjectKey: "icon-key"},
			{BaseModel: model.BaseModel{ID: 99}, UserID: 4, ConversationID: 5, AgentRunID: 3, Kind: "image", ObjectKey: "old-run-key"},
		},
	}
	agent := NewImageEditAgent(registry, writer, ImageEditAgentOptions{
		ImageModelConfigID: 42,
		CandidateCount:     3,
		ModelProvider:      "google",
		ModelName:          "image-edit",
	})

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:          3,
		CurrentStepID:  8,
		UserID:         4,
		ConversationID: 5,
		UserRequest:    "在模板图上加标题，右下角放 icon",
		Prompts: domain.PromptBundle{
			PositivePrompt: "Edit the template image with title text and place the icon in the lower right corner.",
		},
		Metadata: map[string]string{
			"input_artifact_ids": "[51,52]",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.request.UserID != 4 || provider.request.RunID != 3 || provider.request.StepID != 8 {
		t.Fatalf("edit request scope = %#v, want run/user/step scope", provider.request)
	}
	if got, want := strings.Join(provider.request.ImageRefs, ","), "template-key,icon-key"; got != want {
		t.Fatalf("ImageRefs = %q, want %q", got, want)
	}
	if provider.request.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want capped by tool capability", provider.request.CandidateCount)
	}
	if !strings.Contains(provider.request.Prompt, "first reference image as the template") {
		t.Fatalf("Prompt = %q, want template instruction", provider.request.Prompt)
	}
	if len(writer.input.Artifacts) != 1 {
		t.Fatalf("len(writer.input.Artifacts) = %d, want one edited artifact", len(writer.input.Artifacts))
	}
	candidate := writer.input.Artifacts[0]
	if candidate.Version.Operation != "ai_edit" {
		t.Fatalf("Operation = %q, want ai_edit", candidate.Version.Operation)
	}
	if candidate.Artifact.ParentArtifactID != 51 {
		t.Fatalf("ParentArtifactID = %d, want template artifact id", candidate.Artifact.ParentArtifactID)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].ID != 10 || result.Artifacts[0].VersionID != 20 {
		t.Fatalf("artifact refs = %#v, want persisted edit refs", result.Artifacts)
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
