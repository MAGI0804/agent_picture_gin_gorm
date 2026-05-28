package agents

import (
	"context"
	"strings"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestPosterRenderAgentCreatesSVGWhenTextIsRenderedSeparately(t *testing.T) {
	writer := &fakeArtifactWriter{}
	agent := NewPosterRenderAgent(writer)

	result, err := agent.Run(context.Background(), domain.RunState{
		RunID:          11,
		UserID:         7,
		ConversationID: 9,
		Requirements: domain.ImageRequirements{
			Subject:     "新品咖啡上市",
			TargetUse:   "门店竖版海报",
			MustInclude: []string{"ACME Coffee"},
			TextPolicy:  "render text separately",
			AspectRatio: "9:16",
			LayoutHints: []string{"top headline space"},
		},
		Prompts: domain.PromptBundle{
			RenderTextSeparately: true,
			Params:               map[string]string{"title": "新品咖啡上市", "subtitle": "限时尝鲜"},
		},
		Artifacts: []domain.ArtifactRef{
			{ID: 20, VersionID: 21, Kind: "image", PreviewURL: "/api/v2/artifacts/20/preview"},
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != domain.StepStatusCompleted {
		t.Fatalf("Status = %q, want completed", result.Status)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Kind != "svg" {
		t.Fatalf("Artifacts = %#v, want one svg artifact", result.Artifacts)
	}
	if writer.render.ParentArtifactID != 20 || writer.render.ParentVersionID != 21 {
		t.Fatalf("render parent = artifact %d version %d, want 20/21", writer.render.ParentArtifactID, writer.render.ParentVersionID)
	}
	if writer.render.Operation != "render_text" || writer.render.MimeType != "image/svg+xml" {
		t.Fatalf("render metadata = %#v, want SVG render_text", writer.render)
	}
	if !strings.Contains(string(writer.render.Content), "新品咖啡上市") || !strings.Contains(string(writer.render.Content), "限时尝鲜") {
		t.Fatalf("rendered SVG = %s, want layered poster text", string(writer.render.Content))
	}
}

func TestPosterRenderAgentSkipsWhenPromptAllowsModelText(t *testing.T) {
	result, err := NewPosterRenderAgent(&fakeArtifactWriter{}).Run(context.Background(), domain.RunState{
		Prompts: domain.PromptBundle{RenderTextSeparately: false},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != domain.StepStatusSkipped {
		t.Fatalf("Status = %q, want skipped", result.Status)
	}
}
