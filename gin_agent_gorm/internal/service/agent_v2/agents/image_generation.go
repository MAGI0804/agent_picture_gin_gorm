package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/model"
)

type CreateCandidateGroupInput = artifactsvc.CreateCandidateGroupInput

// ArtifactWriter is the persistence capability used by ArtifactAgent.
type ArtifactWriter interface {
	CreateCandidateGroup(input artifactsvc.CreateCandidateGroupInput) ([]model.Artifact, []model.ArtifactVersion, error)
}

// IntentRouterAgent classifies V2 requests into the first supported workflow.
type IntentRouterAgent struct{}

func NewIntentRouterAgent() *IntentRouterAgent {
	return &IntentRouterAgent{}
}

func (agent *IntentRouterAgent) Key() string {
	return "intent_router"
}

func (agent *IntentRouterAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "classified request as image_generation",
		Output: map[string]interface{}{
			"task_type": "image_generation",
			"intent":    "image_generation",
		},
	}, nil
}

// RequirementAgent extracts a first structured image brief from the user request.
type RequirementAgent struct{}

func NewRequirementAgent() *RequirementAgent {
	return &RequirementAgent{}
}

func (agent *RequirementAgent) Key() string {
	return "requirement_agent"
}

func (agent *RequirementAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	userRequest := strings.TrimSpace(state.UserRequest)
	if userRequest == "" {
		return domain.StepResult{}, errors.New("user request is required")
	}
	requirements := domain.ImageRequirements{
		Subject:           truncateRunes(userRequest, 120),
		Style:             inferStyle(userRequest),
		AspectRatio:       inferAspectRatio(userRequest),
		MustInclude:       []string{truncateRunes(userRequest, 80)},
		MustAvoid:         []string{"blur", "watermark", "distorted text", "low quality"},
		NeedClarification: false,
		Questions:         []string{},
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "extracted structured image requirements",
		Output: map[string]interface{}{
			"subject":            requirements.Subject,
			"style":              requirements.Style,
			"aspect_ratio":       requirements.AspectRatio,
			"must_include":       requirements.MustInclude,
			"must_avoid":         requirements.MustAvoid,
			"need_clarification": requirements.NeedClarification,
			"questions":          requirements.Questions,
		},
	}, nil
}

// MemoryAgent carries already loaded memory context through the timeline.
type MemoryAgent struct{}

func NewMemoryAgent() *MemoryAgent {
	return &MemoryAgent{}
}

func (agent *MemoryAgent) Key() string {
	return "memory_agent"
}

func (agent *MemoryAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("loaded %d memory items", len(state.MemoryContext)),
		Output: map[string]interface{}{
			"memory_count": len(state.MemoryContext),
		},
	}, nil
}

// PromptAgent turns requirements and memory context into an image prompt bundle.
type PromptAgent struct{}

func NewPromptAgent() *PromptAgent {
	return &PromptAgent{}
}

func (agent *PromptAgent) Key() string {
	return "prompt_agent"
}

func (agent *PromptAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	source := strings.TrimSpace(state.Requirements.Subject)
	if source == "" {
		source = strings.TrimSpace(state.UserRequest)
	}
	if source == "" {
		return domain.StepResult{}, errors.New("prompt source is required")
	}
	aspectRatio := coalesce(state.Requirements.AspectRatio, "16:9")
	positiveParts := []string{
		source,
		"style: " + coalesce(state.Requirements.Style, "clean commercial visual"),
		"composition: clear subject, balanced lighting, production-ready detail",
		"aspect ratio: " + aspectRatio,
	}
	for _, memory := range state.MemoryContext {
		if strings.TrimSpace(memory.Content) != "" {
			positiveParts = append(positiveParts, "memory: "+strings.TrimSpace(memory.Content))
		}
	}
	negative := strings.Join(nonEmptyStrings(state.Requirements.MustAvoid), ", ")
	if negative == "" {
		negative = "blur, watermark, low quality, distorted anatomy, unreadable text"
	}
	bundle := domain.PromptBundle{
		PositivePrompt:       strings.Join(positiveParts, ", "),
		NegativePrompt:       negative,
		RenderTextSeparately: shouldRenderTextSeparately(source),
		Params: map[string]string{
			"aspect_ratio": aspectRatio,
		},
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "prepared structured image prompt bundle",
		Output: map[string]interface{}{
			"positive_prompt":        bundle.PositivePrompt,
			"negative_prompt":        bundle.NegativePrompt,
			"render_text_separately": bundle.RenderTextSeparately,
			"params":                 bundle.Params,
		},
	}, nil
}

type ImageGenerationAgentOptions struct {
	ImageModelConfigID uint
	CandidateCount     int
}

// ImageGenerationAgent calls the registered image generation capability.
type ImageGenerationAgent struct {
	registry *tools.Registry
	options  ImageGenerationAgentOptions
}

func NewImageGenerationAgent(
	registry *tools.Registry,
	options ImageGenerationAgentOptions,
) *ImageGenerationAgent {
	return &ImageGenerationAgent{registry: registry, options: options}
}

func (agent *ImageGenerationAgent) Key() string {
	return "image_generation_agent"
}

func (agent *ImageGenerationAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("tool registry is required")
	}
	prompt := strings.TrimSpace(state.Prompts.PositivePrompt)
	if prompt == "" {
		prompt = strings.TrimSpace(state.UserRequest)
	}
	if prompt == "" {
		return domain.StepResult{}, errors.New("image prompt is required")
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindImageGeneration,
		UserID:        state.UserID,
		ModelConfigID: agent.options.ImageModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	count := agent.options.CandidateCount
	if count <= 0 {
		count = 1
	}
	if count > 3 {
		count = 3
	}
	result, err := tool.ImageGenerationProvider.GenerateImage(ctx, tools.ImageGenerationRequest{
		UserID:         state.UserID,
		ConversationID: state.ConversationID,
		RunID:          state.RunID,
		StepID:         state.CurrentStepID,
		TaskType:       state.TaskType,
		Intent:         state.Intent,
		Prompt:         prompt,
		NegativePrompt: state.Prompts.NegativePrompt,
		AspectRatio:    coalesce(state.Requirements.AspectRatio, "16:9"),
		CandidateCount: count,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	images := make([]domain.GeneratedImageRef, 0, len(result.Images))
	for _, image := range result.Images {
		images = append(images, domain.GeneratedImageRef{
			Name:       image.Name,
			Kind:       coalesce(image.Kind, "image"),
			MimeType:   image.MimeType,
			ObjectKey:  image.ObjectKey,
			PreviewURL: image.PreviewURL,
			SizeBytes:  image.SizeBytes,
			Hash:       image.Hash,
		})
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("generated %d image candidate(s)", len(images)),
		Output: map[string]interface{}{
			"generated_images": images,
		},
	}, nil
}

type ArtifactAgentOptions struct {
	ModelProvider string
	ModelName     string
}

// ArtifactAgent persists generated image candidates as artifacts and versions.
type ArtifactAgent struct {
	writer  ArtifactWriter
	options ArtifactAgentOptions
}

func NewArtifactAgent(writer ArtifactWriter, options ArtifactAgentOptions) *ArtifactAgent {
	return &ArtifactAgent{writer: writer, options: options}
}

func (agent *ArtifactAgent) Key() string {
	return "artifact_agent"
}

func (agent *ArtifactAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.writer == nil {
		return domain.StepResult{}, errors.New("artifact writer is required")
	}
	if len(state.GeneratedImages) == 0 {
		return domain.StepResult{}, errors.New("generated images are required")
	}
	params := map[string]string{
		"aspect_ratio": coalesce(state.Requirements.AspectRatio, "16:9"),
	}
	for key, value := range state.Prompts.Params {
		params[key] = value
	}
	paramsJSON, _ := json.Marshal(params)

	candidates := make([]artifactsvc.CreateArtifactWithVersionInput, 0, len(state.GeneratedImages))
	for index, image := range state.GeneratedImages {
		kind := coalesce(image.Kind, "image")
		mimeType := coalesce(image.MimeType, "application/octet-stream")
		name := coalesce(image.Name, fmt.Sprintf("generated-image-%d.png", index+1))
		candidates = append(candidates, artifactsvc.CreateArtifactWithVersionInput{
			Artifact: model.Artifact{
				Name:       name,
				Kind:       kind,
				MimeType:   mimeType,
				ObjectKey:  image.ObjectKey,
				PreviewURL: image.PreviewURL,
				SizeBytes:  image.SizeBytes,
				Hash:       image.Hash,
				RankScore:  float64(len(state.GeneratedImages) - index),
			},
			Version: model.ArtifactVersion{
				VersionNo:        1,
				Operation:        "generate",
				Prompt:           state.Prompts.PositivePrompt,
				NegativePrompt:   state.Prompts.NegativePrompt,
				ModelProvider:    agent.options.ModelProvider,
				ModelName:        agent.options.ModelName,
				GenerationParams: string(paramsJSON),
				ObjectKey:        image.ObjectKey,
				PreviewURL:       image.PreviewURL,
				Hash:             image.Hash,
			},
		})
	}
	artifacts, versions, err := agent.writer.CreateCandidateGroup(artifactsvc.CreateCandidateGroupInput{
		AgentRunID:     state.RunID,
		UserID:         state.UserID,
		ConversationID: state.ConversationID,
		Artifacts:      candidates,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	refs := make([]domain.ArtifactRef, 0, len(artifacts))
	for index, artifact := range artifacts {
		var versionID uint
		if index < len(versions) {
			versionID = versions[index].ID
		}
		refs = append(refs, domain.ArtifactRef{
			ID:         artifact.ID,
			VersionID:  versionID,
			Kind:       artifact.Kind,
			PreviewURL: artifact.PreviewURL,
		})
	}
	return domain.StepResult{
		Status:    domain.StepStatusCompleted,
		Summary:   fmt.Sprintf("persisted %d artifact candidate(s)", len(refs)),
		Output:    map[string]interface{}{"artifact_count": len(refs)},
		Artifacts: refs,
	}, nil
}

func inferAspectRatio(text string) string {
	normalized := strings.ToLower(text)
	switch {
	case strings.Contains(normalized, "9:16"), strings.Contains(normalized, "vertical"):
		return "9:16"
	case strings.Contains(normalized, "1:1"), strings.Contains(normalized, "square"):
		return "1:1"
	case strings.Contains(normalized, "4:3"):
		return "4:3"
	default:
		return "16:9"
	}
}

func inferStyle(text string) string {
	normalized := strings.ToLower(text)
	switch {
	case strings.Contains(normalized, "poster"):
		return "poster design"
	case strings.Contains(normalized, "photo"), strings.Contains(normalized, "realistic"):
		return "photorealistic"
	case strings.Contains(normalized, "anime"), strings.Contains(normalized, "illustration"):
		return "illustration"
	default:
		return "clean commercial visual"
	}
}

func shouldRenderTextSeparately(text string) bool {
	for _, r := range text {
		if r > utf8.RuneSelf {
			return true
		}
	}
	return strings.Contains(strings.ToLower(text), "text") || strings.Contains(text, "\"")
}

func nonEmptyStrings(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			output = append(output, value)
		}
	}
	return output
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func coalesce(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}
