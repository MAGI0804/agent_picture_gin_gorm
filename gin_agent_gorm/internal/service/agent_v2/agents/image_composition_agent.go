package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"gin-biz-web-api/internal/service/agent_svc"
	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
)

// CompositionStore is the small persistence surface needed by the bitmap composer.
type CompositionStore interface {
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateRenderedArtifact(input artifactsvc.CreateRenderedArtifactInput) (model.Artifact, model.ArtifactVersion, error)
}

// ImageCompositionAgent renders uploaded assets into one final PNG.
type ImageCompositionAgent struct {
	store CompositionStore
}

func NewImageCompositionAgent(store CompositionStore) *ImageCompositionAgent {
	return &ImageCompositionAgent{store: store}
}

func (agent *ImageCompositionAgent) Key() string {
	return "image_composition_agent"
}

func (agent *ImageCompositionAgent) Run(ctx context.Context, state domain.RunState) (domain.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.StepResult{}, err
	}
	if agent.store == nil {
		return domain.StepResult{}, errors.New("composition store is required")
	}
	uploads, err := agent.uploadedImages(state.UserID, state.ConversationID)
	if err != nil {
		return domain.StepResult{}, err
	}
	if len(uploads) == 0 {
		return domain.StepResult{}, errors.New("image composition requires at least one uploaded template image")
	}

	objectStore := agent_svc.NewObjectStore()
	baseImage, err := loadImage(objectStore.Path(uploads[0].ObjectKey))
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("load template image: %w", err)
	}
	canvas := image.NewRGBA(baseImage.Bounds())
	draw.Draw(canvas, canvas.Bounds(), baseImage, baseImage.Bounds().Min, draw.Src)

	iconCount := 0
	for index, icon := range uploads[1:] {
		iconImage, err := loadImage(objectStore.Path(icon.ObjectKey))
		if err != nil {
			continue
		}
		drawIcon(canvas, iconImage, index)
		iconCount++
	}

	texts := compositionTextLines(state)
	if len(texts) > 0 {
		drawTextBlock(canvas, texts)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return domain.StepResult{}, fmt.Errorf("encode composed image: %w", err)
	}

	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"template_artifact_id": uploads[0].ID,
		"icon_artifact_ids":    artifactIDs(uploads[1:]),
		"text_lines":           texts,
	})
	artifact, version, err := agent.store.CreateRenderedArtifact(artifactsvc.CreateRenderedArtifactInput{
		UserID:           state.UserID,
		ConversationID:   state.ConversationID,
		AgentRunID:       state.RunID,
		ParentArtifactID: uploads[0].ID,
		Name:             "composed-image.png",
		Kind:             "image",
		MimeType:         "image/png",
		Content:          buffer.Bytes(),
		Operation:        "compose",
		Prompt:           strings.TrimSpace(state.UserRequest),
		ModelProvider:    "local_bitmap_composer",
		ModelName:        "template_icon_text_v1",
		SourceRefs:       string(sourceRefs),
	})
	if err != nil {
		return domain.StepResult{}, err
	}

	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "composed final PNG image from uploaded template, icons, and text",
		Output: map[string]interface{}{
			"template_artifact_id": uploads[0].ID,
			"icon_count":           iconCount,
			"text_lines":           texts,
			"artifact_id":          artifact.ID,
			"version_id":           version.ID,
		},
		Artifacts: []domain.ArtifactRef{{
			ID:         artifact.ID,
			VersionID:  version.ID,
			Kind:       artifact.Kind,
			PreviewURL: artifact.PreviewURL,
		}},
	}, nil
}

func (agent *ImageCompositionAgent) uploadedImages(userID uint, conversationID uint) ([]model.Artifact, error) {
	artifacts, err := agent.store.ListArtifacts(userID, conversationID)
	if err != nil {
		return nil, err
	}
	uploads := make([]model.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.AgentRunID == 0 && strings.EqualFold(artifact.Kind, "image") && strings.TrimSpace(artifact.ObjectKey) != "" {
			uploads = append(uploads, artifact)
		}
	}
	sort.SliceStable(uploads, func(i int, j int) bool {
		return uploads[i].ID < uploads[j].ID
	})
	return uploads, nil
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	image, _, err := image.Decode(file)
	return image, err
}

func drawIcon(canvas *image.RGBA, icon image.Image, index int) {
	bounds := canvas.Bounds()
	size := maxIntAgent(bounds.Dx()/8, 64)
	if size > 180 {
		size = 180
	}
	resized := imaging.Fit(icon, size, size, imaging.Lanczos)
	margin := maxIntAgent(bounds.Dx()/32, 24)
	gap := maxIntAgent(size/5, 12)
	x := bounds.Max.X - margin - resized.Bounds().Dx() - index*(size+gap)
	y := bounds.Max.Y - margin - resized.Bounds().Dy()
	if x < margin {
		x = margin + (index%3)*(size+gap)
		y = bounds.Max.Y - margin - resized.Bounds().Dy() - (index/3)*(size+gap)
	}
	point := image.Pt(x, y)
	draw.Draw(canvas, resized.Bounds().Add(point), resized, resized.Bounds().Min, draw.Over)
}

func compositionTextLines(state domain.RunState) []string {
	source := strings.TrimSpace(state.UserRequest)
	lines := extractQuotedTexts(source)
	if len(lines) == 0 {
		for _, key := range []string{"title", "subtitle", "brand"} {
			if value := strings.TrimSpace(state.Prompts.Params[key]); value != "" {
				lines = append(lines, value)
			}
		}
	}
	if len(lines) == 0 && strings.TrimSpace(state.Requirements.Subject) != "" {
		lines = append(lines, strings.TrimSpace(state.Requirements.Subject))
	}
	if len(lines) == 0 && source != "" {
		lines = append(lines, truncateRunes(source, 28))
	}
	if len(lines) > 3 {
		return lines[:3]
	}
	return lines
}

func extractQuotedTexts(source string) []string {
	pairs := [][2]string{
		{"\u201c", "\u201d"},
		{"\"", "\""},
		{"'", "'"},
		{"\u300c", "\u300d"},
		{"\u300a", "\u300b"},
	}
	lines := []string{}
	for _, pair := range pairs {
		remaining := source
		for {
			start := strings.Index(remaining, pair[0])
			if start < 0 {
				break
			}
			afterStart := remaining[start+len(pair[0]):]
			end := strings.Index(afterStart, pair[1])
			if end < 0 {
				break
			}
			text := strings.TrimSpace(afterStart[:end])
			if text != "" {
				lines = append(lines, text)
			}
			remaining = afterStart[end+len(pair[1]):]
		}
	}
	return cleanTextLines(lines)
}

func drawTextBlock(canvas *image.RGBA, lines []string) {
	bounds := canvas.Bounds()
	padding := maxIntAgent(bounds.Dx()/32, 28)
	boxWidth := bounds.Dx() - padding*2
	titleSize := float64(maxIntAgent(bounds.Dx()/22, 34))
	subtitleSize := titleSize * 0.58
	faceTitle := loadFontFace(titleSize)
	faceSubtitle := loadFontFace(subtitleSize)
	lineHeight := int(titleSize * 1.25)
	boxHeight := padding + lineHeight
	if len(lines) > 1 {
		boxHeight += (len(lines) - 1) * int(subtitleSize*1.35)
	}
	boxHeight += padding / 2
	box := image.Rect(padding, padding, padding+boxWidth, padding+boxHeight)
	draw.Draw(canvas, box, &image.Uniform{C: color.RGBA{0, 0, 0, 118}}, image.Point{}, draw.Over)

	y := padding + int(titleSize)
	for index, line := range lines {
		face := faceSubtitle
		if index == 0 {
			face = faceTitle
		}
		wrapped := wrapText(line, face, boxWidth-padding)
		for _, text := range wrapped {
			drawString(canvas, face, padding+padding/2, y, text)
			if index == 0 {
				y += lineHeight
			} else {
				y += int(subtitleSize * 1.35)
			}
		}
	}
}

func drawString(canvas *image.RGBA, face font.Face, x int, y int, text string) {
	drawer := font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(color.RGBA{255, 250, 240, 255}),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(text)
}

func wrapText(text string, face font.Face, maxWidth int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	lines := []string{}
	current := ""
	for _, r := range runes {
		next := current + string(r)
		if current != "" && font.MeasureString(face, next).Ceil() > maxWidth {
			lines = append(lines, current)
			current = string(r)
			continue
		}
		current = next
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func loadFontFace(size float64) font.Face {
	for _, path := range fontCandidates() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func fontCandidates() []string {
	return []string{
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "simhei.ttf"),
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "msyh.ttc"),
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	}
}

func artifactIDs(artifacts []model.Artifact) []uint {
	ids := make([]uint, 0, len(artifacts))
	for _, artifact := range artifacts {
		ids = append(ids, artifact.ID)
	}
	return ids
}

func maxIntAgent(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func cleanTextLines(values []string) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && utf8.ValidString(value) {
			output = append(output, value)
		}
	}
	return output
}
