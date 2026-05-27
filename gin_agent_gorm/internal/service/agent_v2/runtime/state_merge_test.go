package runtime

import (
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestApplyStepResultMergesRealImageWorkflowOutputs(t *testing.T) {
	state := domain.RunState{}

	state = applyStepResult(state, "requirement_agent", domain.StepResult{
		Summary: "requirements",
		Output: map[string]interface{}{
			"subject":            "coffee poster",
			"style":              "editorial",
			"aspect_ratio":       "16:9",
			"must_include":       []string{"coffee", "headline"},
			"must_avoid":         []string{"blur"},
			"need_clarification": false,
			"questions":          []string{},
			"scene":              "sunlit cafe counter",
			"composition":        "centered cup with headline space",
			"text_policy":        "render text separately",
			"layout_hints":       []string{"top headline space"},
			"target_use":         "mobile poster",
		},
	})
	state = applyStepResult(state, "prompt_agent", domain.StepResult{
		Summary: "prompt",
		Output: map[string]interface{}{
			"positive_prompt":        "coffee poster with editorial lighting",
			"negative_prompt":        "blur, watermark",
			"render_text_separately": true,
			"params": map[string]string{
				"aspect_ratio": "16:9",
			},
		},
	})
	state = applyStepResult(state, "image_generation_agent", domain.StepResult{
		Summary: "image",
		Output: map[string]interface{}{
			"generated_images": []domain.GeneratedImageRef{
				{ObjectKey: "object-key", PreviewURL: "/artifacts/object-key"},
			},
		},
	})
	state = applyStepResult(state, "artifact_agent", domain.StepResult{
		Summary: "artifact",
		Artifacts: []domain.ArtifactRef{
			{ID: 10, VersionID: 20, Kind: "image", PreviewURL: "/artifacts/object-key"},
		},
	})

	if state.Requirements.Subject != "coffee poster" {
		t.Fatalf("Subject = %q, want coffee poster", state.Requirements.Subject)
	}
	if state.Requirements.Scene != "sunlit cafe counter" {
		t.Fatalf("Scene = %q, want sunlit cafe counter", state.Requirements.Scene)
	}
	if len(state.Requirements.LayoutHints) != 1 || state.Requirements.LayoutHints[0] != "top headline space" {
		t.Fatalf("LayoutHints = %#v, want top headline space", state.Requirements.LayoutHints)
	}
	if state.Prompts.NegativePrompt != "blur, watermark" {
		t.Fatalf("NegativePrompt = %q, want blur, watermark", state.Prompts.NegativePrompt)
	}
	if !state.Prompts.RenderTextSeparately {
		t.Fatalf("RenderTextSeparately = false, want true")
	}
	if len(state.GeneratedImages) != 1 || state.GeneratedImages[0].ObjectKey != "object-key" {
		t.Fatalf("GeneratedImages = %#v, want object-key", state.GeneratedImages)
	}
	if len(state.Artifacts) != 1 || state.Artifacts[0].VersionID != 20 {
		t.Fatalf("Artifacts = %#v, want version 20", state.Artifacts)
	}
}
