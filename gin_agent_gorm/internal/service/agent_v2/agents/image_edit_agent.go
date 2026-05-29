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

// ImageEditStore is the persistence and source lookup surface used by the AI edit agent.
type ImageEditStore interface {
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateCandidateGroup(input artifactsvc.CreateCandidateGroupInput) ([]model.Artifact, []model.ArtifactVersion, error)
}

type ImageEditAgentOptions struct {
	ImageModelConfigID uint
	CandidateCount     int
	ModelProvider      string
	ModelName          string
}

// ImageEditAgent calls the image edit provider with uploaded template/icon references.
type ImageEditAgent struct {
	registry *tools.Registry
	store    ImageEditStore
	options  ImageEditAgentOptions
}

func NewImageEditAgent(registry *tools.Registry, store ImageEditStore, options ImageEditAgentOptions) *ImageEditAgent {
	return &ImageEditAgent{registry: registry, store: store, options: options}
}

func (agent *ImageEditAgent) Key() string {
	return "image_edit_agent"
}

func (agent *ImageEditAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.registry == nil {
		return domain.StepResult{}, errors.New("tool registry is required")
	}
	if agent.store == nil {
		return domain.StepResult{}, errors.New("image edit store is required")
	}
	inputs, err := uploadedImageArtifacts(agent.store, state)
	if err != nil {
		return domain.StepResult{}, err
	}
	if len(inputs) == 0 {
		return domain.StepResult{}, errors.New("image edit requires at least one uploaded template image")
	}

	tool, err := agent.registry.FindTool(tools.FindToolRequest{
		Kind:          tools.KindImageEdit,
		UserID:        state.UserID,
		ModelConfigID: agent.options.ImageModelConfigID,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	prompt := editPromptFromState(state)
	if strings.TrimSpace(prompt) == "" {
		return domain.StepResult{}, errors.New("image edit prompt is required")
	}
	imageRefs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		imageRefs = append(imageRefs, input.ObjectKey)
	}
	count := constrainedCandidateCount(agent.options.CandidateCount, tool.Capability.MaxCandidates)
	result, err := tool.ImageEditProvider.EditImage(ctx, tools.ImageEditRequest{
		UserID:         state.UserID,
		ConversationID: state.ConversationID,
		RunID:          state.RunID,
		StepID:         state.CurrentStepID,
		TaskType:       "image_edit",
		Prompt:         prompt,
		ImageRefs:      imageRefs,
		CandidateCount: count,
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	if len(result.Images) == 0 {
		return domain.StepResult{}, errors.New("image edit provider returned no images")
	}

	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"input_artifact_ids": artifactIDs(inputs),
		"image_refs":         imageRefs,
	})
	params, _ := json.Marshal(map[string]interface{}{
		"source":          "ai_image_edit",
		"input_count":     len(inputs),
		"candidate_count": len(result.Images),
	})
	candidates := make([]artifactsvc.CreateArtifactWithVersionInput, 0, len(result.Images))
	for index, image := range result.Images {
		kind := coalesce(image.Kind, "image")
		mimeType := coalesce(image.MimeType, "application/octet-stream")
		name := coalesce(image.Name, fmt.Sprintf("edited-image-%d.png", index+1))
		candidates = append(candidates, artifactsvc.CreateArtifactWithVersionInput{
			Artifact: model.Artifact{
				Name:             name,
				Kind:             kind,
				MimeType:         mimeType,
				ObjectKey:        image.ObjectKey,
				PreviewURL:       image.PreviewURL,
				SizeBytes:        image.SizeBytes,
				Hash:             image.Hash,
				ParentArtifactID: inputs[0].ID,
				RankScore:        float64(len(result.Images) - index),
				Visibility:       "private",
				StoragePolicy:    "local_private",
			},
			Version: model.ArtifactVersion{
				VersionNo:        1,
				Operation:        "ai_edit",
				Prompt:           prompt,
				ModelProvider:    agent.options.ModelProvider,
				ModelName:        agent.options.ModelName,
				GenerationParams: string(params),
				SourceRefs:       string(sourceRefs),
				ObjectKey:        image.ObjectKey,
				PreviewURL:       image.PreviewURL,
				Hash:             image.Hash,
			},
		})
	}
	artifacts, versions, err := agent.store.CreateCandidateGroup(artifactsvc.CreateCandidateGroupInput{
		AgentRunID:      state.RunID,
		UserID:          state.UserID,
		ConversationID:  state.ConversationID,
		ArtifactGroupID: fmt.Sprintf("run-%d-ai-edits", state.RunID),
		Artifacts:       candidates,
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
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("edited %d image candidate(s) with AI image edit provider", len(refs)),
		Output: map[string]interface{}{
			"input_artifact_ids": artifactIDs(inputs),
			"image_ref_count":    len(imageRefs),
			"artifact_count":     len(refs),
		},
		Artifacts: refs,
	}, nil
}

func uploadedImageArtifacts(store interface {
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
}, state domain.RunState) ([]model.Artifact, error) {
	artifacts, err := store.ListArtifacts(state.UserID, state.ConversationID)
	if err != nil {
		return nil, err
	}
	if ids := inputArtifactIDs(state.Metadata); len(ids) > 0 {
		byID := make(map[uint]model.Artifact, len(artifacts))
		for _, artifact := range artifacts {
			if strings.EqualFold(artifact.Kind, "image") && strings.TrimSpace(artifact.ObjectKey) != "" {
				byID[artifact.ID] = artifact
			}
		}
		selected := make([]model.Artifact, 0, len(ids))
		for _, id := range ids {
			if artifact, ok := byID[id]; ok {
				selected = append(selected, artifact)
			}
		}
		return selected, nil
	}
	uploads := make([]model.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.AgentRunID == 0 && strings.EqualFold(artifact.Kind, "image") && strings.TrimSpace(artifact.ObjectKey) != "" {
			uploads = append(uploads, artifact)
		}
	}
	return uploads, nil
}

func editPromptFromState(state domain.RunState) string {
	parts := []string{}
	if strings.TrimSpace(state.UserRequest) != "" {
		parts = append(parts, "Original user request:\n"+strings.TrimSpace(state.UserRequest))
	}
	if strings.TrimSpace(state.Prompts.PositivePrompt) != "" {
		parts = append(parts, strings.TrimSpace(state.Prompts.PositivePrompt))
	}
	parts = append(parts,
		"Use the first reference image as the template/base image.",
		"Use the remaining reference images as logos/icons/materials when requested.",
		"Preserve the template composition unless the user explicitly asks to change it.",
		"Do not render exact typography, font-size labels, color values, or instruction text into the image. Precise text and logo placement will be rendered by the final compositor.",
		"Focus the AI edit on the photographic/base-image result and keep clear negative space for the requested layout.",
	)
	return strings.Join(parts, "\n\n")
}
