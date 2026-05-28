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
	CreateRefinedVersion(input artifactsvc.CreateRefinedVersionInput) (model.ArtifactVersion, error)
	CreateRenderedArtifact(input artifactsvc.CreateRenderedArtifactInput) (model.Artifact, model.ArtifactVersion, error)
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
		Summary: "已识别为图片生成任务",
		Output: map[string]interface{}{
			"task_type": "image_generation",
			"intent":    "image_generation",
		},
	}, nil
}

// RequirementAgent extracts a first structured image brief from the user request.
type RequirementAgent struct {
	registry          *tools.Registry
	textModelConfigID uint
}

func NewRequirementAgent() *RequirementAgent {
	return &RequirementAgent{}
}

func NewRequirementAgentWithText(registry *tools.Registry, textModelConfigID uint) *RequirementAgent {
	return &RequirementAgent{registry: registry, textModelConfigID: textModelConfigID}
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

	if agent.registry != nil {
		result, err := agent.runWithTextProvider(ctx, state)
		if err == nil {
			return result, nil
		}
		return fallbackRequirementResult(state, err.Error()), nil
	}
	return fallbackRequirementResult(state, ""), nil
}

func (agent *RequirementAgent) runWithTextProvider(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindText,
		UserID:        state.UserID,
		ModelConfigID: agent.textModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("text provider unavailable: %w", err)
	}
	result, err := tool.TextProvider.GenerateText(ctx, tools.TextRequest{
		UserID: state.UserID,
		RunID:  state.RunID,
		StepID: state.CurrentStepID,
		System: requirementAgentSystemPrompt(),
		Prompt: requirementAgentPrompt(state),
	})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("requirement text provider failed: %w", err)
	}
	requirements, err := parseRequirementJSON(result.Text, state.UserRequest)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("invalid requirement json: %w", err)
	}
	return requirementStepResult(requirements, nil), nil
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
		Summary: fmt.Sprintf("已加载 %d 条记忆", len(state.MemoryContext)),
		Output: map[string]interface{}{
			"memory_count": len(state.MemoryContext),
		},
	}, nil
}

// PromptAgent turns requirements and memory context into an image prompt bundle.
type PromptAgent struct {
	registry          *tools.Registry
	textModelConfigID uint
}

func NewPromptAgent() *PromptAgent {
	return &PromptAgent{}
}

func NewPromptAgentWithText(registry *tools.Registry, textModelConfigID uint) *PromptAgent {
	return &PromptAgent{registry: registry, textModelConfigID: textModelConfigID}
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
	if agent.registry != nil {
		result, err := agent.runWithTextProvider(ctx, state)
		if err == nil {
			return result, nil
		}
		return fallbackPromptResult(state, err.Error()), nil
	}
	return fallbackPromptResult(state, ""), nil
}

func (agent *PromptAgent) runWithTextProvider(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindText,
		UserID:        state.UserID,
		ModelConfigID: agent.textModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("text provider unavailable: %w", err)
	}
	result, err := tool.TextProvider.GenerateText(ctx, tools.TextRequest{
		UserID: state.UserID,
		RunID:  state.RunID,
		StepID: state.CurrentStepID,
		System: promptAgentSystemPrompt(),
		Prompt: promptAgentPrompt(state),
	})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("prompt text provider failed: %w", err)
	}
	bundle, err := parsePromptJSON(result.Text, state)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("invalid prompt json: %w", err)
	}
	return promptStepResult(bundle, nil), nil
}

func fallbackPromptResult(state domain.RunState, schemaIssue string) domain.StepResult {
	source := strings.TrimSpace(state.Requirements.Subject)
	if source == "" {
		source = strings.TrimSpace(state.UserRequest)
	}
	aspectRatio := coalesce(state.Requirements.AspectRatio, "16:9")
	positiveParts := []string{
		source,
		"style: " + coalesce(state.Requirements.Style, "clean commercial visual"),
		"composition: " + coalesce(state.Requirements.Composition, "clear subject, balanced lighting, production-ready detail"),
		"aspect ratio: " + aspectRatio,
	}
	if strings.TrimSpace(state.Requirements.Scene) != "" {
		positiveParts = append(positiveParts, "scene: "+strings.TrimSpace(state.Requirements.Scene))
	}
	if strings.TrimSpace(state.Requirements.TargetUse) != "" {
		positiveParts = append(positiveParts, "target use: "+strings.TrimSpace(state.Requirements.TargetUse))
	}
	for _, hint := range nonEmptyStrings(state.Requirements.LayoutHints) {
		positiveParts = append(positiveParts, "layout hint: "+hint)
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
		RenderTextSeparately: shouldRenderTextSeparately(requirementTextProbe(state)) || textPolicyRequiresSeparate(state.Requirements.TextPolicy),
		Params: map[string]string{
			"aspect_ratio": aspectRatio,
		},
	}
	return promptStepResult(bundle, schemaIssues(schemaIssue))
}

func promptStepResult(bundle domain.PromptBundle, issues []string) domain.StepResult {
	if bundle.Params == nil {
		bundle.Params = map[string]string{}
	}
	output := map[string]interface{}{
		"positive_prompt":        bundle.PositivePrompt,
		"negative_prompt":        bundle.NegativePrompt,
		"render_text_separately": bundle.RenderTextSeparately,
		"params":                 bundle.Params,
	}
	if len(issues) > 0 {
		output["schema_issues"] = issues
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "已生成结构化图片提示词",
		Output:  output,
	}
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
	prompt = truncateRunes(prompt, tool.Capability.MaxPromptChars)
	negativePrompt := truncateRunes(state.Prompts.NegativePrompt, tool.Capability.MaxPromptChars)
	aspectRatio := supportedAspectRatio(state.Requirements.AspectRatio, tool.Capability.SupportedRatios)
	count := constrainedCandidateCount(agent.options.CandidateCount, tool.Capability.MaxCandidates)

	images := make([]domain.GeneratedImageRef, 0, count)
	for len(images) < count {
		remaining := count - len(images)
		result, err := tool.ImageGenerationProvider.GenerateImage(ctx, tools.ImageGenerationRequest{
			UserID:              state.UserID,
			ConversationID:      state.ConversationID,
			RunID:               state.RunID,
			StepID:              state.CurrentStepID,
			TaskType:            state.TaskType,
			Intent:              state.Intent,
			Prompt:              prompt,
			NegativePrompt:      negativePrompt,
			AspectRatio:         aspectRatio,
			CandidateCount:      remaining,
			CandidateStartIndex: len(images),
		})
		if err != nil {
			return domain.StepResult{}, err
		}
		if len(result.Images) == 0 {
			break
		}
		for _, image := range result.Images {
			if len(images) >= count {
				break
			}
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
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("已生成 %d 张候选图片", len(images)),
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
		Summary:   fmt.Sprintf("已保存 %d 个候选产物", len(refs)),
		Output:    map[string]interface{}{"artifact_count": len(refs)},
		Artifacts: refs,
	}, nil
}

type requirementProviderOutput struct {
	Subject           string   `json:"subject"`
	Style             string   `json:"style"`
	AspectRatio       string   `json:"aspect_ratio"`
	MustInclude       []string `json:"must_include"`
	MustAvoid         []string `json:"must_avoid"`
	NeedClarification bool     `json:"need_clarification"`
	Questions         []string `json:"questions"`
	Scene             string   `json:"scene"`
	Composition       string   `json:"composition"`
	TextPolicy        string   `json:"text_policy"`
	LayoutHints       []string `json:"layout_hints"`
	TargetUse         string   `json:"target_use"`
}

type promptProviderOutput struct {
	PositivePrompt       string            `json:"positive_prompt"`
	NegativePrompt       string            `json:"negative_prompt"`
	RenderTextSeparately bool              `json:"render_text_separately"`
	Params               map[string]string `json:"params"`
}

func fallbackRequirementResult(state domain.RunState, schemaIssue string) domain.StepResult {
	userRequest := strings.TrimSpace(state.UserRequest)
	textPolicy := "image model may render simple text when needed"
	layoutHints := []string{}
	if shouldRenderTextSeparately(userRequest) {
		textPolicy = "render text separately"
		layoutHints = append(layoutHints, "reserve clean space for text overlay")
	}
	questions := inferClarificationQuestions(userRequest)
	requirements := domain.ImageRequirements{
		Subject:           truncateRunes(userRequest, 120),
		Style:             inferStyle(userRequest),
		AspectRatio:       inferAspectRatio(userRequest),
		MustInclude:       []string{truncateRunes(userRequest, 80)},
		MustAvoid:         []string{"blur", "watermark", "distorted text", "low quality"},
		NeedClarification: len(questions) > 0,
		Questions:         questions,
		Scene:             truncateRunes(userRequest, 160),
		Composition:       "clear subject, balanced lighting, production-ready detail",
		TextPolicy:        textPolicy,
		LayoutHints:       layoutHints,
		TargetUse:         inferTargetUse(userRequest),
	}
	return requirementStepResult(requirements, schemaIssues(schemaIssue))
}

func requirementStepResult(requirements domain.ImageRequirements, issues []string) domain.StepResult {
	output := map[string]interface{}{
		"subject":            requirements.Subject,
		"style":              requirements.Style,
		"aspect_ratio":       requirements.AspectRatio,
		"must_include":       requirements.MustInclude,
		"must_avoid":         requirements.MustAvoid,
		"need_clarification": requirements.NeedClarification,
		"questions":          requirements.Questions,
		"scene":              requirements.Scene,
		"composition":        requirements.Composition,
		"text_policy":        requirements.TextPolicy,
		"layout_hints":       requirements.LayoutHints,
		"target_use":         requirements.TargetUse,
	}
	if len(issues) > 0 {
		output["schema_issues"] = issues
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "已提取结构化图片需求",
		Output:  output,
	}
}

func parseRequirementJSON(raw string, userRequest string) (domain.ImageRequirements, error) {
	var decoded requirementProviderOutput
	if err := decodeProviderJSONObject(raw, &decoded); err != nil {
		return domain.ImageRequirements{}, err
	}
	requirements := domain.ImageRequirements{
		Subject:           strings.TrimSpace(decoded.Subject),
		Style:             coalesce(decoded.Style, inferStyle(userRequest)),
		AspectRatio:       coalesce(decoded.AspectRatio, inferAspectRatio(userRequest)),
		MustInclude:       nonEmptyStrings(decoded.MustInclude),
		MustAvoid:         nonEmptyStrings(decoded.MustAvoid),
		NeedClarification: decoded.NeedClarification,
		Questions:         nonEmptyStrings(decoded.Questions),
		Scene:             strings.TrimSpace(decoded.Scene),
		Composition:       strings.TrimSpace(decoded.Composition),
		TextPolicy:        strings.TrimSpace(decoded.TextPolicy),
		LayoutHints:       nonEmptyStrings(decoded.LayoutHints),
		TargetUse:         strings.TrimSpace(decoded.TargetUse),
	}
	if requirements.Subject == "" {
		return domain.ImageRequirements{}, errors.New("subject is required")
	}
	if !isAllowedAspectRatio(requirements.AspectRatio) {
		return domain.ImageRequirements{}, fmt.Errorf("unsupported aspect_ratio %q", requirements.AspectRatio)
	}
	if len(requirements.MustInclude) == 0 {
		requirements.MustInclude = []string{truncateRunes(requirements.Subject, 80)}
	}
	if len(requirements.MustAvoid) == 0 {
		requirements.MustAvoid = []string{"blur", "watermark", "distorted text", "low quality"}
	}
	if requirements.Scene == "" {
		requirements.Scene = truncateRunes(userRequest, 160)
	}
	if requirements.Composition == "" {
		requirements.Composition = "clear subject, balanced lighting, production-ready detail"
	}
	if requirements.TextPolicy == "" && shouldRenderTextSeparately(requirementTextProbe(domain.RunState{
		UserRequest:  userRequest,
		Requirements: requirements,
	})) {
		requirements.TextPolicy = "render text separately"
	}
	if textPolicyRequiresSeparate(requirements.TextPolicy) && len(requirements.LayoutHints) == 0 {
		requirements.LayoutHints = []string{"reserve clean space for text overlay"}
	}
	if requirements.TargetUse == "" {
		requirements.TargetUse = inferTargetUse(userRequest)
	}
	if requirements.NeedClarification && len(requirements.Questions) == 0 {
		requirements.Questions = defaultClarificationQuestions()
	}
	return requirements, nil
}

func parsePromptJSON(raw string, state domain.RunState) (domain.PromptBundle, error) {
	var decoded promptProviderOutput
	if err := decodeProviderJSONObject(raw, &decoded); err != nil {
		return domain.PromptBundle{}, err
	}
	bundle := domain.PromptBundle{
		PositivePrompt:       strings.TrimSpace(decoded.PositivePrompt),
		NegativePrompt:       strings.TrimSpace(decoded.NegativePrompt),
		RenderTextSeparately: decoded.RenderTextSeparately || shouldRenderTextSeparately(requirementTextProbe(state)) || textPolicyRequiresSeparate(state.Requirements.TextPolicy),
		Params:               cleanStringMap(decoded.Params),
	}
	if bundle.PositivePrompt == "" {
		return domain.PromptBundle{}, errors.New("positive_prompt is required")
	}
	if bundle.NegativePrompt == "" {
		bundle.NegativePrompt = "blur, watermark, low quality, distorted anatomy, unreadable text"
	}
	if bundle.Params == nil {
		bundle.Params = map[string]string{}
	}
	aspectRatio := coalesce(state.Requirements.AspectRatio, "16:9")
	if !isAllowedAspectRatio(aspectRatio) {
		aspectRatio = "16:9"
	}
	if providerAspectRatio := strings.TrimSpace(bundle.Params["aspect_ratio"]); providerAspectRatio != "" {
		if !isAllowedAspectRatio(providerAspectRatio) {
			return domain.PromptBundle{}, fmt.Errorf("unsupported params.aspect_ratio %q", providerAspectRatio)
		}
		aspectRatio = providerAspectRatio
	}
	bundle.Params["aspect_ratio"] = aspectRatio
	return bundle, nil
}

func decodeProviderJSONObject(raw string, target interface{}) error {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return errors.New("json object is required")
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), target); err != nil {
		return err
	}
	return nil
}

func requirementAgentSystemPrompt() string {
	return strings.Join([]string{
		"Return strict JSON only for an image generation requirement schema.",
		"Required keys: subject, style, aspect_ratio, must_include, must_avoid, need_clarification, questions, scene, composition, text_policy, layout_hints, target_use.",
		"Supported aspect_ratio values: 1:1, 4:3, 16:9, 9:16.",
		"For Chinese posters or complex text, set text_policy to render text separately.",
	}, "\n")
}

func requirementAgentPrompt(state domain.RunState) string {
	return marshalPromptPayload(map[string]interface{}{
		"user_request":   state.UserRequest,
		"task_type":      state.TaskType,
		"intent":         state.Intent,
		"clarifications": state.Clarifications,
	})
}

func promptAgentSystemPrompt() string {
	return strings.Join([]string{
		"Return strict JSON only for an image model prompt bundle.",
		"Required keys: positive_prompt, negative_prompt, render_text_separately, params.",
		"params must include aspect_ratio when known.",
		"Use stable memory context only as factual constraints; do not invent unsupported claims.",
	}, "\n")
}

func promptAgentPrompt(state domain.RunState) string {
	return marshalPromptPayload(map[string]interface{}{
		"user_request":   state.UserRequest,
		"requirements":   state.Requirements,
		"memory_context": state.MemoryContext,
	})
}

func marshalPromptPayload(payload map[string]interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func schemaIssues(issue string) []string {
	issue = strings.TrimSpace(issue)
	if issue == "" {
		return nil
	}
	return []string{issue}
}

func cleanStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func requirementTextProbe(state domain.RunState) string {
	parts := []string{
		state.UserRequest,
		state.Requirements.Subject,
		state.Requirements.Scene,
		state.Requirements.Composition,
		state.Requirements.TextPolicy,
		state.Requirements.TargetUse,
	}
	parts = append(parts, state.Requirements.MustInclude...)
	parts = append(parts, state.Requirements.LayoutHints...)
	return strings.Join(nonEmptyStrings(parts), " ")
}

func textPolicyRequiresSeparate(textPolicy string) bool {
	normalized := strings.ToLower(strings.TrimSpace(textPolicy))
	return strings.Contains(normalized, "separate") ||
		strings.Contains(normalized, "overlay") ||
		strings.Contains(normalized, "external") ||
		strings.Contains(normalized, "do not render text")
}

func supportedAspectRatio(requested string, supported []string) string {
	requested = coalesce(requested, "16:9")
	for _, ratio := range supported {
		if strings.TrimSpace(ratio) == requested {
			return requested
		}
	}
	if len(supported) == 0 {
		if isAllowedAspectRatio(requested) {
			return requested
		}
		return "16:9"
	}
	for _, ratio := range supported {
		if strings.TrimSpace(ratio) == "16:9" {
			return "16:9"
		}
	}
	for _, ratio := range supported {
		ratio = strings.TrimSpace(ratio)
		if ratio != "" {
			return ratio
		}
	}
	return "16:9"
}

func constrainedCandidateCount(requested int, maxCandidates int) int {
	if requested <= 0 {
		requested = 1
	}
	if maxCandidates > 0 && requested > maxCandidates {
		requested = maxCandidates
	}
	if requested > 3 {
		requested = 3
	}
	if requested <= 0 {
		return 1
	}
	return requested
}

func isAllowedAspectRatio(value string) bool {
	switch strings.TrimSpace(value) {
	case "1:1", "4:3", "16:9", "9:16":
		return true
	default:
		return false
	}
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

func inferTargetUse(text string) string {
	normalized := strings.ToLower(text)
	switch {
	case strings.Contains(normalized, "poster"):
		return "poster"
	case strings.Contains(normalized, "banner"):
		return "banner"
	case strings.Contains(normalized, "avatar"):
		return "avatar"
	default:
		return "general image generation"
	}
}

func inferClarificationQuestions(text string) []string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return defaultClarificationQuestions()
	}
	vagueRequests := []string{
		"make an image",
		"generate an image",
		"draw a picture",
		"做一张图",
		"生成图片",
		"随便做",
	}
	for _, request := range vagueRequests {
		if normalized == request {
			return defaultClarificationQuestions()
		}
	}
	if len([]rune(normalized)) < 8 || len(strings.Fields(normalized)) <= 2 {
		return defaultClarificationQuestions()
	}
	return []string{}
}

func defaultClarificationQuestions() []string {
	return []string{
		"图片的主体应该是什么？",
		"希望采用什么风格、用途或画面比例？",
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
