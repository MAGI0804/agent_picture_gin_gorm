package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
)

// PosterRenderAgent creates a text-layer SVG artifact for poster/brand outputs.
type PosterRenderAgent struct {
	writer ArtifactWriter
}

func NewPosterRenderAgent(writer ArtifactWriter) *PosterRenderAgent {
	return &PosterRenderAgent{writer: writer}
}

func (agent *PosterRenderAgent) Key() string {
	return "poster_render_agent"
}

func (agent *PosterRenderAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if !state.Prompts.RenderTextSeparately {
		return domain.StepResult{
			Status:  domain.StepStatusSkipped,
			Summary: "text-layer rendering was not requested",
			Output:  map[string]interface{}{"render_text_separately": false},
		}, nil
	}
	if agent.writer == nil {
		return domain.StepResult{}, errors.New("poster render writer is required")
	}
	background, ok := firstImageArtifact(state.Artifacts)
	if !ok {
		return domain.StepResult{}, errors.New("poster render requires a source image artifact")
	}
	layers := posterTextLayers(state)
	if len(layers) == 0 {
		return domain.StepResult{
			Status:  domain.StepStatusSkipped,
			Summary: "no poster text layers were inferred",
			Output:  map[string]interface{}{"render_text_separately": true, "text_layer_count": 0},
		}, nil
	}
	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"background_artifact_id": background.ID,
		"background_version_id":  background.VersionID,
		"layout_hints":           state.Requirements.LayoutHints,
		"text_policy":            state.Requirements.TextPolicy,
	})
	svg := renderPosterSVG(background.PreviewURL, layers, state.Requirements.AspectRatio)
	artifact, version, err := agent.writer.CreateRenderedArtifact(artifactsvc.CreateRenderedArtifactInput{
		UserID:           state.UserID,
		ConversationID:   state.ConversationID,
		AgentRunID:       state.RunID,
		ParentArtifactID: background.ID,
		ParentVersionID:  background.VersionID,
		Name:             "poster-text-layer.svg",
		Kind:             "svg",
		MimeType:         "image/svg+xml",
		Content:          []byte(svg),
		Operation:        "render_text",
		Prompt:           strings.Join(layerTexts(layers), "\n"),
		ModelProvider:    "layout_renderer",
		ModelName:        "svg_text_layer_v1",
		SourceRefs:       string(sourceRefs),
	})
	if err != nil {
		return domain.StepResult{}, err
	}
	ref := domain.ArtifactRef{
		ID:         artifact.ID,
		VersionID:  version.ID,
		Kind:       artifact.Kind,
		PreviewURL: artifact.PreviewURL,
	}
	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: fmt.Sprintf("rendered %d text layer(s) into SVG artifact", len(layers)),
		Output: map[string]interface{}{
			"rendered_artifact_id": artifact.ID,
			"text_layer_count":     len(layers),
			"rendered_text":        strings.Join(layerTexts(layers), " "),
			"source_refs":          string(sourceRefs),
		},
		Artifacts: []domain.ArtifactRef{ref},
	}, nil
}

type posterTextLayer struct {
	Text   string
	Role   string
	Y      int
	Size   int
	Weight string
}

func firstImageArtifact(artifacts []domain.ArtifactRef) (domain.ArtifactRef, bool) {
	for _, artifact := range artifacts {
		if artifact.ID != 0 && strings.EqualFold(artifact.Kind, "image") {
			return artifact, true
		}
	}
	return domain.ArtifactRef{}, false
}

func posterTextLayers(state domain.RunState) []posterTextLayer {
	title := firstNonEmpty(state.Prompts.Params["title"], state.Requirements.Subject)
	subtitle := firstNonEmpty(state.Prompts.Params["subtitle"], state.Requirements.TargetUse, state.Requirements.Scene)
	brand := firstNonEmpty(state.Prompts.Params["brand"], firstMustInclude(state.Requirements.MustInclude))
	layers := []posterTextLayer{}
	if title != "" {
		layers = append(layers, posterTextLayer{Text: title, Role: "title", Y: 170, Size: 58, Weight: "700"})
	}
	if subtitle != "" && subtitle != title {
		layers = append(layers, posterTextLayer{Text: subtitle, Role: "subtitle", Y: 245, Size: 28, Weight: "500"})
	}
	if brand != "" && brand != title && brand != subtitle {
		layers = append(layers, posterTextLayer{Text: brand, Role: "brand", Y: 860, Size: 24, Weight: "600"})
	}
	return layers
}

func renderPosterSVG(backgroundRef string, layers []posterTextLayer, aspectRatio string) string {
	width, height := 1080, 1350
	if aspectRatio == "16:9" {
		width, height = 1600, 900
	}
	if aspectRatio == "1:1" {
		width, height = 1200, 1200
	}
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height))
	builder.WriteString(`<defs><linearGradient id="poster-bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="#17202a"/><stop offset="55%" stop-color="#314d63"/><stop offset="100%" stop-color="#f2c572"/></linearGradient></defs>`)
	builder.WriteString(`<rect width="100%" height="100%" fill="url(#poster-bg)"/>`)
	if strings.TrimSpace(backgroundRef) != "" {
		builder.WriteString(fmt.Sprintf(`<image href="%s" x="0" y="0" width="%d" height="%d" preserveAspectRatio="xMidYMid slice"/>`, html.EscapeString(backgroundRef), width, height))
	}
	builder.WriteString(`<rect x="44" y="88" width="72%" height="230" rx="18" fill="#000000" opacity="0.32"/>`)
	for _, layer := range layers {
		builder.WriteString(fmt.Sprintf(
			`<text x="78" y="%d" fill="#fffaf0" font-family="Arial, 'Microsoft YaHei', sans-serif" font-size="%d" font-weight="%s">%s</text>`,
			layer.Y,
			layer.Size,
			html.EscapeString(layer.Weight),
			html.EscapeString(layer.Text),
		))
	}
	builder.WriteString(`</svg>`)
	return builder.String()
}

func layerTexts(layers []posterTextLayer) []string {
	values := make([]string, 0, len(layers))
	for _, layer := range layers {
		if strings.TrimSpace(layer.Text) != "" {
			values = append(values, layer.Text)
		}
	}
	return values
}

func firstMustInclude(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
