package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/model"
)

// RefinedVersionWriter is the persistence capability used by RefinerAgent.
type RefinedVersionWriter interface {
	CreateRefinedVersion(input artifactsvc.CreateRefinedVersionInput) (model.ArtifactVersion, error)
}

// RefinerAgentOptions configures automatic one-shot refinement.
type RefinerAgentOptions struct {
	ImageModelConfigID uint
	ModelProvider      string
	ModelName          string
}

// RefinerAgent creates at most one improved image version after a low review score.
type RefinerAgent struct {
	registry *tools.Registry
	writer   RefinedVersionWriter
	options  RefinerAgentOptions
}

func NewRefinerAgent(registry *tools.Registry, writer RefinedVersionWriter, options RefinerAgentOptions) *RefinerAgent {
	return &RefinerAgent{registry: registry, writer: writer, options: options}
}

func (agent *RefinerAgent) Key() string {
	return "refiner_agent"
}

func (agent *RefinerAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if !state.Review.ShouldRefine {
		return refinerSkipped("review did not request refinement"), nil
	}
	if state.Budget.MaxAutoRefines <= 0 {
		return refinerSkipped("auto refine budget is exhausted"), nil
	}
	target, ok := selectRefineTarget(state)
	if !ok {
		return refinerSkipped("no reviewed artifact candidate available"), nil
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("tool registry is required")
	}
	if agent.writer == nil {
		return domain.StepResult{}, errors.New("refined version writer is required")
	}
	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindImageGeneration,
		UserID:        state.UserID,
		ModelConfigID: agent.options.ImageModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	prompt := truncateRunes(refinePrompt(state, target), tool.Capability.MaxPromptChars)
	negativePrompt := truncateRunes(coalesce(state.Prompts.NegativePrompt, "blur, watermark, low quality, unreadable text"), tool.Capability.MaxPromptChars)
	aspectRatio := supportedAspectRatio(state.Requirements.AspectRatio, tool.Capability.SupportedRatios)
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
		CandidateCount:      1,
		CandidateStartIndex: len(state.GeneratedImages),
		Temperature:         "0.2",
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	if len(result.Images) == 0 {
		return domain.StepResult{}, errors.New("refiner provider returned no image")
	}
	image := result.Images[0]
	sourceRefs, _ := json.Marshal([]map[string]interface{}{
		{
			"artifact_id":        target.ArtifactID,
			"parent_version_id":  target.VersionID,
			"review_score":       target.OverallScore,
			"review_issues":      target.Issues,
			"refine_source_step": "refiner_agent",
		},
	})
	version, err := agent.writer.CreateRefinedVersion(artifactsvc.CreateRefinedVersionInput{
		UserID:          state.UserID,
		ArtifactID:      target.ArtifactID,
		ParentVersionID: target.VersionID,
		AgentRunID:      state.RunID,
		Image: model.ArtifactVersion{
			Operation:        "refine",
			Prompt:           prompt,
			NegativePrompt:   negativePrompt,
			ModelProvider:    agent.options.ModelProvider,
			ModelName:        agent.options.ModelName,
			GenerationParams: fmt.Sprintf(`{"aspect_ratio":%q,"auto_refine":true}`, aspectRatio),
			SourceRefs:       string(sourceRefs),
			ObjectKey:        image.ObjectKey,
			PreviewURL:       image.PreviewURL,
			Hash:             image.Hash,
		},
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	ref := domain.ArtifactRef{
		ID:         target.ArtifactID,
		VersionID:  version.ID,
		Kind:       coalesce(image.Kind, "image"),
		PreviewURL: image.PreviewURL,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("created refined version %d from version %d", version.ID, target.VersionID),
		Output: map[string]interface{}{
			"refined":           true,
			"artifact_id":       target.ArtifactID,
			"parent_version_id": target.VersionID,
			"version_id":        version.ID,
			"reason":            strings.Join(target.Issues, "; "),
			"refined_versions":  []domain.ArtifactRef{ref},
		},
		Artifacts: []domain.ArtifactRef{ref},
	}, nil
}

func refinerSkipped(reason string) domain.StepResult {
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "auto refine skipped: " + reason,
		Output: map[string]interface{}{
			"refined": false,
			"reason":  reason,
		},
	}
}

func selectRefineTarget(state domain.RunState) (domain.CandidateReview, bool) {
	reviews := state.Review.CandidateReviews
	if len(reviews) == 0 {
		return domain.CandidateReview{}, false
	}
	best := domain.CandidateReview{}
	for _, review := range reviews {
		if review.ArtifactID == 0 || review.VersionID == 0 {
			continue
		}
		if best.ArtifactID == 0 ||
			(review.ShouldRefine && !best.ShouldRefine) ||
			(review.ShouldRefine == best.ShouldRefine && review.OverallScore < best.OverallScore) {
			best = review
		}
	}
	return best, best.ArtifactID != 0 && best.VersionID != 0
}

func refinePrompt(state domain.RunState, target domain.CandidateReview) string {
	parts := []string{
		"Create an improved replacement image based on the original request.",
		"Original request: " + state.UserRequest,
		"Current prompt: " + state.Prompts.PositivePrompt,
	}
	if len(target.Issues) > 0 {
		parts = append(parts, "Fix these review issues: "+strings.Join(target.Issues, "; "))
	}
	if state.Requirements.TextPolicy != "" {
		parts = append(parts, "Text policy: "+state.Requirements.TextPolicy)
	}
	if len(state.Requirements.LayoutHints) > 0 {
		parts = append(parts, "Layout hints: "+strings.Join(state.Requirements.LayoutHints, "; "))
	}
	parts = append(parts, "Preserve the successful visual direction, but correct the review failures.")
	return strings.Join(nonEmptyStrings(parts), "\n")
}
