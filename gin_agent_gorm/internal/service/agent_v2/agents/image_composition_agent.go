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
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// ImageCompositionAgent renders uploaded assets into one final bitmap.
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
	inputs, err := agent.compositionInputs(state)
	if err != nil {
		return domain.StepResult{}, err
	}

	objectStore := agent_svc.NewObjectStore()
	templateImage, err := loadImage(objectStore.Path(inputs.Template.ObjectKey))
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("load template image: %w", err)
	}
	canvas := image.NewRGBA(templateImage.Bounds())
	baseImage := templateImage
	if inputs.Base.ID != inputs.Template.ID {
		aiImage, loadErr := loadImage(objectStore.Path(inputs.Base.ObjectKey))
		if loadErr == nil && sameImageAspect(templateImage.Bounds(), aiImage.Bounds()) {
			baseImage = aiImage
			inputs.AIBaseUsed = true
		} else if loadErr != nil {
			inputs.AIBaseNote = loadErr.Error()
		} else {
			inputs.AIBaseNote = "ai edit output aspect ratio differs from template"
		}
	}
	drawBaseImage(canvas, baseImage)

	iconCount := 0
	texts := []string{}
	layout := scaleExactTemplateLayoutForCanvas(parseExactTemplateLayout(state.UserRequest), canvas.Bounds())
	if layout.IsExact {
		iconCount = drawExactTemplateLayout(canvas, objectStore, inputs.Icons, layout)
		texts = layout.Texts()
	} else {
		for index, icon := range inputs.Icons {
			iconImage, err := loadImage(objectStore.Path(icon.ObjectKey))
			if err != nil {
				continue
			}
			drawIcon(canvas, iconImage, index)
			iconCount++
		}

		texts = compositionTextLines(state)
		if len(texts) > 0 {
			drawTextBlock(canvas, texts)
		}
	}

	content, mimeType, fileName, err := encodeComposedImage(canvas, inputs.Template)
	if err != nil {
		return domain.StepResult{}, err
	}

	sourceRefs, _ := json.Marshal(map[string]interface{}{
		"template_artifact_id": inputs.Template.ID,
		"ai_base_artifact_id":  inputs.Base.ID,
		"ai_base_used":         inputs.AIBaseUsed,
		"ai_base_note":         inputs.AIBaseNote,
		"icon_artifact_ids":    artifactIDs(inputs.Icons),
		"text_lines":           texts,
		"output_width":         canvas.Bounds().Dx(),
		"output_height":        canvas.Bounds().Dy(),
		"output_mime_type":     mimeType,
	})
	artifact, version, err := agent.store.CreateRenderedArtifact(artifactsvc.CreateRenderedArtifactInput{
		UserID:           state.UserID,
		ConversationID:   state.ConversationID,
		AgentRunID:       state.RunID,
		ParentArtifactID: inputs.Template.ID,
		Name:             fileName,
		Kind:             "image",
		MimeType:         mimeType,
		Content:          content,
		Operation:        "compose",
		Prompt:           strings.TrimSpace(state.UserRequest),
		ModelProvider:    "local_bitmap_composer",
		ModelName:        "template_icon_text_v2",
		SourceRefs:       string(sourceRefs),
	})
	if err != nil {
		return domain.StepResult{}, err
	}

	return domain.StepResult{
		Status:  domain.StepStatusCompleted,
		Summary: "composed final bitmap image from uploaded template, icons, and exact text",
		Output: map[string]interface{}{
			"template_artifact_id": inputs.Template.ID,
			"ai_base_artifact_id":  inputs.Base.ID,
			"ai_base_used":         inputs.AIBaseUsed,
			"icon_count":           iconCount,
			"text_lines":           texts,
			"artifact_id":          artifact.ID,
			"version_id":           version.ID,
			"output_width":         canvas.Bounds().Dx(),
			"output_height":        canvas.Bounds().Dy(),
			"output_mime_type":     mimeType,
		},
		Artifacts: []domain.ArtifactRef{{
			ID:         artifact.ID,
			VersionID:  version.ID,
			Kind:       artifact.Kind,
			PreviewURL: artifact.PreviewURL,
		}},
	}, nil
}

type compositionInputs struct {
	Template   model.Artifact
	Base       model.Artifact
	Icons      []model.Artifact
	AIBaseUsed bool
	AIBaseNote string
}

func (agent *ImageCompositionAgent) compositionInputs(state domain.RunState) (compositionInputs, error) {
	artifacts, err := agent.store.ListArtifacts(state.UserID, state.ConversationID)
	if err != nil {
		return compositionInputs{}, err
	}
	uploads := selectedUploadedImages(artifacts, state)
	if len(uploads) == 0 {
		return compositionInputs{}, errors.New("image composition requires at least one uploaded template image")
	}
	inputs := compositionInputs{
		Template: uploads[0],
		Base:     uploads[0],
		Icons:    uploads[1:],
	}
	if aiBase, ok := selectAIEditBaseArtifact(artifacts, state, uploads[0].ID); ok {
		inputs.Base = aiBase
	}
	return inputs, nil
}

func (agent *ImageCompositionAgent) uploadedImages(state domain.RunState) ([]model.Artifact, error) {
	artifacts, err := agent.store.ListArtifacts(state.UserID, state.ConversationID)
	if err != nil {
		return nil, err
	}
	return selectedUploadedImages(artifacts, state), nil
}

func selectedUploadedImages(artifacts []model.Artifact, state domain.RunState) []model.Artifact {
	if ids := inputArtifactIDs(state.Metadata); len(ids) > 0 {
		byID := make(map[uint]model.Artifact, len(artifacts))
		for _, artifact := range artifacts {
			if isUsableImageArtifact(artifact) {
				byID[artifact.ID] = artifact
			}
		}
		selected := make([]model.Artifact, 0, len(ids))
		for _, id := range ids {
			if artifact, ok := byID[id]; ok {
				selected = append(selected, artifact)
			}
		}
		return selected
	}
	uploads := make([]model.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.AgentRunID == 0 && isUsableImageArtifact(artifact) {
			uploads = append(uploads, artifact)
		}
	}
	sort.SliceStable(uploads, func(i int, j int) bool {
		return uploads[i].ID < uploads[j].ID
	})
	return uploads
}

func selectAIEditBaseArtifact(artifacts []model.Artifact, state domain.RunState, templateID uint) (model.Artifact, bool) {
	if state.RunID == 0 || templateID == 0 {
		return model.Artifact{}, false
	}
	candidates := make([]model.Artifact, 0)
	for _, artifact := range artifacts {
		if artifact.AgentRunID != state.RunID || artifact.ParentArtifactID != templateID || !isUsableImageArtifact(artifact) {
			continue
		}
		if strings.Contains(artifact.ArtifactGroupID, "ai-edits") {
			candidates = append(candidates, artifact)
		}
	}
	if len(candidates) == 0 {
		return model.Artifact{}, false
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		if candidates[i].RankScore == candidates[j].RankScore {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].RankScore > candidates[j].RankScore
	})
	return candidates[0], true
}

func isUsableImageArtifact(artifact model.Artifact) bool {
	return strings.EqualFold(artifact.Kind, "image") && strings.TrimSpace(artifact.ObjectKey) != ""
}

func inputArtifactIDs(metadata map[string]string) []uint {
	if metadata == nil {
		return nil
	}
	raw := strings.TrimSpace(metadata["input_artifact_ids"])
	if raw == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	cleaned := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			cleaned = append(cleaned, id)
		}
	}
	return cleaned
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

type exactTemplateLayout struct {
	IsExact     bool
	TopLeft     horizontalTextSpec
	TopRight    horizontalTextSpec
	LeftColumns []verticalTextSpec
	RightLeft   verticalTextSpec
	RightRight  verticalTextSpec
	Logo        logoSpec
}

type horizontalTextSpec struct {
	Text       string
	X          int
	Y          int
	RightEdge  int
	Size       float64
	Color      color.RGBA
	RightAlign bool
}

type verticalTextSpec struct {
	Text       string
	X          int
	Y          int
	Size       float64
	Color      color.RGBA
	SlashColor color.RGBA
	Bold       bool
}

type logoSpec struct {
	X      int
	Bottom int
	Scale  float64
}

func drawBaseImage(canvas *image.RGBA, base image.Image) {
	if base.Bounds().Dx() == canvas.Bounds().Dx() && base.Bounds().Dy() == canvas.Bounds().Dy() {
		draw.Draw(canvas, canvas.Bounds(), base, base.Bounds().Min, draw.Src)
		return
	}
	resized := imaging.Fill(base, canvas.Bounds().Dx(), canvas.Bounds().Dy(), imaging.Center, imaging.Lanczos)
	draw.Draw(canvas, canvas.Bounds(), resized, resized.Bounds().Min, draw.Src)
}

func sameImageAspect(left image.Rectangle, right image.Rectangle) bool {
	if left.Dx() <= 0 || left.Dy() <= 0 || right.Dx() <= 0 || right.Dy() <= 0 {
		return false
	}
	leftRatio := float64(left.Dx()) / float64(left.Dy())
	rightRatio := float64(right.Dx()) / float64(right.Dy())
	diff := leftRatio - rightRatio
	if diff < 0 {
		diff = -diff
	}
	return diff <= leftRatio*0.08
}

func parseExactTemplateLayout(source string) exactTemplateLayout {
	source = strings.TrimSpace(source)
	hasTextSpecs := strings.Contains(source, "文字内容：") ||
		strings.Contains(source, "① 文字：") ||
		strings.Contains(source, "右侧文字：") ||
		strings.Contains(source, "左侧文字：")
	if !hasTextSpecs || !strings.Contains(source, "边缘") {
		return exactTemplateLayout{}
	}
	section1 := sectionBetween(source, "1.", "2.")
	section2 := sectionBetween(source, "2.", "3.")
	section3 := sectionBetween(source, "3.", "4.")
	section4 := sectionBetween(source, "4.", "5.")
	section5 := sectionAfter(source, "5.")
	layout := exactTemplateLayout{
		IsExact: true,
		TopLeft: horizontalTextSpec{
			Text:       fieldLine(section1, "文字内容："),
			X:          intOrDefault(distanceValue(section1, "左"), 50),
			Y:          intOrDefault(distanceValue(section1, "上"), 50),
			Size:       floatOrDefault(fontSizeValue(section1), 16),
			Color:      colorOrDefault(firstHexColor(section1), color.RGBA{182, 182, 180, 255}),
			RightAlign: false,
		},
		TopRight: horizontalTextSpec{
			Text:       fieldLine(section2, "文字内容："),
			RightEdge:  intOrDefault(distanceValue(section2, "右"), 50),
			Y:          intOrDefault(distanceValue(section2, "上"), 50),
			Size:       floatOrDefault(fontSizeValue(section2), 16),
			Color:      colorOrDefault(firstHexColor(section2), color.RGBA{182, 182, 180, 255}),
			RightAlign: true,
		},
		Logo: logoSpec{
			X:      intOrDefault(distanceValue(section5, "左"), 50),
			Bottom: intOrDefault(distanceValue(section5, "下"), 100),
		},
	}
	leftX := intOrDefault(distanceValue(section3, "左"), 80)
	leftY := intOrDefault(distanceValue(section3, "上"), 250)
	leftOne := textBeforeMarker(section3, "① 文字：", "字体")
	leftTwo := textBeforeMarker(section3, "② 文字：", "字体")
	if leftOne != "" {
		layout.LeftColumns = append(layout.LeftColumns, verticalTextSpec{
			Text:       leftOne,
			X:          leftX,
			Y:          leftY,
			Size:       floatOrDefault(fontSizeNear(section3, leftOne), 28),
			Color:      colorOrDefault(firstHexColorAfter(section3, leftOne), color.RGBA{0, 181, 226, 255}),
			SlashColor: color.RGBA{3, 179, 226, 255},
			Bold:       strings.Contains(section3, "加粗"),
		})
	}
	if leftTwo != "" {
		layout.LeftColumns = append(layout.LeftColumns, verticalTextSpec{
			Text:       leftTwo,
			X:          leftX,
			Y:          leftY,
			Size:       floatOrDefault(fontSizeNear(section3, leftTwo), 26),
			Color:      colorOrDefault(firstHexColorAfter(section3, leftTwo), color.RGBA{36, 36, 37, 255}),
			SlashColor: color.RGBA{3, 179, 226, 255},
		})
	}
	rightTop := intOrDefault(distanceValue(section4, "上"), 320)
	rightMargin := intOrDefault(distanceValue(section4, "右"), 80)
	rightOne := textBeforeMarker(section4, "① 右侧文字：", "字体")
	rightTwo := textBeforeMarker(section4, "② 左侧文字：", "字体")
	slashColor := colorOrDefault(lastHexColor(section4), color.RGBA{3, 179, 226, 255})
	layout.RightRight = verticalTextSpec{
		Text:       rightOne,
		X:          rightMargin,
		Y:          rightTop,
		Size:       floatOrDefault(fontSizeNear(section4, rightOne), 20),
		Color:      colorOrDefault(firstHexColorAfter(section4, rightOne), color.RGBA{78, 78, 79, 255}),
		SlashColor: slashColor,
	}
	layout.RightLeft = verticalTextSpec{
		Text:       rightTwo,
		X:          rightMargin,
		Y:          rightTop,
		Size:       floatOrDefault(fontSizeNear(section4, rightTwo), 20),
		Color:      colorOrDefault(firstHexColorAfter(section4, rightTwo), color.RGBA{78, 78, 79, 255}),
		SlashColor: slashColor,
	}
	return layout
}

func (layout exactTemplateLayout) Texts() []string {
	values := []string{layout.TopLeft.Text, layout.TopRight.Text}
	for _, spec := range layout.LeftColumns {
		values = append(values, spec.Text)
	}
	values = append(values, layout.RightRight.Text, layout.RightLeft.Text)
	return nonEmptyStrings(values)
}

func scaleExactTemplateLayoutForCanvas(layout exactTemplateLayout, bounds image.Rectangle) exactTemplateLayout {
	if !layout.IsExact {
		return layout
	}
	scale := exactLayoutScale(bounds)
	if scale <= 1 {
		layout.Logo.Scale = 1
		return layout
	}
	scaleInt := func(value int) int {
		if value <= 0 {
			return value
		}
		return int(float64(value)*scale + 0.5)
	}
	scaleSize := func(value float64) float64 {
		if value <= 0 {
			return value
		}
		return value * scale
	}
	layout.TopLeft.X = scaleInt(layout.TopLeft.X)
	layout.TopLeft.Y = scaleInt(layout.TopLeft.Y)
	layout.TopLeft.Size = scaleSize(layout.TopLeft.Size)
	layout.TopRight.RightEdge = scaleInt(layout.TopRight.RightEdge)
	layout.TopRight.Y = scaleInt(layout.TopRight.Y)
	layout.TopRight.Size = scaleSize(layout.TopRight.Size)
	for index := range layout.LeftColumns {
		layout.LeftColumns[index].X = scaleInt(layout.LeftColumns[index].X)
		layout.LeftColumns[index].Y = scaleInt(layout.LeftColumns[index].Y)
		layout.LeftColumns[index].Size = scaleSize(layout.LeftColumns[index].Size)
	}
	layout.RightLeft.X = scaleInt(layout.RightLeft.X)
	layout.RightLeft.Y = scaleInt(layout.RightLeft.Y)
	layout.RightLeft.Size = scaleSize(layout.RightLeft.Size)
	layout.RightRight.X = scaleInt(layout.RightRight.X)
	layout.RightRight.Y = scaleInt(layout.RightRight.Y)
	layout.RightRight.Size = scaleSize(layout.RightRight.Size)
	layout.Logo.X = scaleInt(layout.Logo.X)
	layout.Logo.Bottom = scaleInt(layout.Logo.Bottom)
	layout.Logo.Scale = scale
	return layout
}

func exactLayoutScale(bounds image.Rectangle) float64 {
	longEdge := bounds.Dx()
	if bounds.Dy() > longEdge {
		longEdge = bounds.Dy()
	}
	scale := float64(longEdge) / 1600
	if scale < 1 {
		return 1
	}
	if scale > 5 {
		return 5
	}
	return scale
}

func drawExactTemplateLayout(canvas *image.RGBA, objectStore agent_svc.ObjectStore, icons []model.Artifact, layout exactTemplateLayout) int {
	drawHorizontalText(canvas, layout.TopLeft)
	drawHorizontalText(canvas, layout.TopRight)
	drawLeftVerticalColumns(canvas, layout.LeftColumns)
	drawRightVerticalColumns(canvas, layout)
	iconCount := 0
	if len(icons) > 0 {
		if iconImage, err := loadImage(objectStore.Path(icons[0].ObjectKey)); err == nil {
			drawLogoAt(canvas, iconImage, layout.Logo)
			iconCount = 1
		}
	}
	return iconCount
}

func drawHorizontalText(canvas *image.RGBA, spec horizontalTextSpec) {
	text := strings.TrimSpace(spec.Text)
	if text == "" {
		return
	}
	face := loadFontFace(spec.Size)
	x := spec.X
	if spec.RightAlign {
		width := font.MeasureString(face, text).Ceil()
		x = canvas.Bounds().Dx() - spec.RightEdge - width
	}
	y := spec.Y + face.Metrics().Ascent.Ceil()
	drawStringWithColor(canvas, face, x, y, text, spec.Color)
}

func drawLeftVerticalColumns(canvas *image.RGBA, specs []verticalTextSpec) {
	x := 0
	for index, spec := range specs {
		if index == 0 {
			x = spec.X
		} else {
			x += verticalColumnWidth(specs[index-1]) + int(spec.Size)
		}
		spec.X = x
		drawVerticalText(canvas, spec)
	}
}

func drawRightVerticalColumns(canvas *image.RGBA, layout exactTemplateLayout) {
	if strings.TrimSpace(layout.RightRight.Text) == "" && strings.TrimSpace(layout.RightLeft.Text) == "" {
		return
	}
	rightMargin := 80
	if layout.RightRight.X > 0 {
		rightMargin = layout.RightRight.X
	}
	rightWidth := verticalColumnWidth(layout.RightRight)
	leftWidth := verticalColumnWidth(layout.RightLeft)
	gap := int(layout.RightRight.Size * 1.4)
	rightX := canvas.Bounds().Dx() - rightMargin - rightWidth
	leftX := rightX - gap - leftWidth
	layout.RightLeft.X = leftX
	layout.RightRight.X = rightX
	drawVerticalText(canvas, layout.RightLeft)
	drawVerticalText(canvas, layout.RightRight)
}

func drawLogoAt(canvas *image.RGBA, icon image.Image, spec logoSpec) {
	iconBounds := icon.Bounds()
	if spec.Scale > 1 {
		targetWidth := int(float64(iconBounds.Dx())*spec.Scale + 0.5)
		targetHeight := int(float64(iconBounds.Dy())*spec.Scale + 0.5)
		maxWidth := canvas.Bounds().Dx() / 4
		maxHeight := canvas.Bounds().Dy() / 6
		if targetWidth > maxWidth {
			targetWidth = maxWidth
		}
		if targetHeight > maxHeight {
			targetHeight = maxHeight
		}
		if targetWidth > 0 && targetHeight > 0 {
			resized := imaging.Fit(icon, targetWidth, targetHeight, imaging.Lanczos)
			icon = resized
			iconBounds = resized.Bounds()
		}
	}
	x := spec.X
	y := canvas.Bounds().Dy() - spec.Bottom - iconBounds.Dy()
	if y < spec.Bottom {
		maxHeight := canvas.Bounds().Dy() / 8
		resized := imaging.Fit(icon, canvas.Bounds().Dx()/8, maxHeight, imaging.Lanczos)
		icon = resized
		iconBounds = resized.Bounds()
		y = canvas.Bounds().Dy() - spec.Bottom - iconBounds.Dy()
	}
	if y < 0 {
		y = 0
	}
	draw.Draw(canvas, iconBounds.Add(image.Pt(x, y)), icon, iconBounds.Min, draw.Over)
}

func drawVerticalText(canvas *image.RGBA, spec verticalTextSpec) {
	text := strings.TrimSpace(spec.Text)
	if text == "" {
		return
	}
	face := loadFontFace(spec.Size)
	units := verticalUnits(text, spec.Color, spec.SlashColor)
	y := spec.Y + face.Metrics().Ascent.Ceil()
	step := int(spec.Size * 1.28)
	columnWidth := verticalColumnWidth(spec)
	for _, unit := range units {
		width := font.MeasureString(face, unit.Text).Ceil()
		x := spec.X + (columnWidth-width)/2
		drawStringWithColor(canvas, face, x, y, unit.Text, unit.Color)
		if spec.Bold {
			drawStringWithColor(canvas, face, x+1, y, unit.Text, unit.Color)
		}
		y += step
	}
}

type verticalUnit struct {
	Text  string
	Color color.RGBA
}

func verticalUnits(text string, textColor color.RGBA, slashColor color.RGBA) []verticalUnit {
	runes := []rune(strings.TrimSpace(text))
	units := make([]verticalUnit, 0, len(runes))
	for index := 0; index < len(runes); {
		r := runes[index]
		if r == ' ' || r == '\t' {
			index++
			continue
		}
		if r == '\\' || r == '/' {
			units = append(units, verticalUnit{Text: string(r), Color: slashColor})
			index++
			continue
		}
		if isASCIIWordRune(r) {
			start := index
			for index < len(runes) && isASCIIWordRune(runes[index]) {
				index++
			}
			units = append(units, verticalUnit{Text: string(runes[start:index]), Color: textColor})
			continue
		}
		units = append(units, verticalUnit{Text: string(r), Color: textColor})
		index++
	}
	return units
}

func isASCIIWordRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func verticalColumnWidth(spec verticalTextSpec) int {
	face := loadFontFace(spec.Size)
	maxWidth := int(spec.Size)
	for _, unit := range verticalUnits(spec.Text, spec.Color, spec.SlashColor) {
		if width := font.MeasureString(face, unit.Text).Ceil(); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func encodeComposedImage(canvas image.Image, template model.Artifact) ([]byte, string, string, error) {
	mimeType := strings.ToLower(strings.TrimSpace(template.MimeType))
	if mimeType == "" {
		mimeType = mimeTypeForName(template.Name)
	}
	var buffer bytes.Buffer
	switch mimeType {
	case "image/jpeg", "image/jpg":
		if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 95}); err != nil {
			return nil, "", "", fmt.Errorf("encode composed jpeg image: %w", err)
		}
		return buffer.Bytes(), "image/jpeg", "composed-image.jpg", nil
	default:
		if err := png.Encode(&buffer, canvas); err != nil {
			return nil, "", "", fmt.Errorf("encode composed png image: %w", err)
		}
		return buffer.Bytes(), "image/png", "composed-image.png", nil
	}
}

func mimeTypeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "image/png"
	}
}

func sectionBetween(source string, startMarker string, endMarker string) string {
	start := strings.Index(source, startMarker)
	if start < 0 {
		return ""
	}
	remaining := source[start+len(startMarker):]
	end := strings.Index(remaining, endMarker)
	if end < 0 {
		return strings.TrimSpace(remaining)
	}
	return strings.TrimSpace(remaining[:end])
}

func sectionAfter(source string, marker string) string {
	start := strings.Index(source, marker)
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(source[start+len(marker):])
}

func fieldLine(section string, marker string) string {
	start := strings.Index(section, marker)
	if start < 0 {
		return ""
	}
	remaining := section[start+len(marker):]
	return strings.TrimSpace(strings.Split(remaining, "\n")[0])
}

func textBeforeMarker(section string, startMarker string, endMarker string) string {
	start := strings.Index(section, startMarker)
	if start < 0 {
		return ""
	}
	remaining := section[start+len(startMarker):]
	end := strings.Index(remaining, endMarker)
	if end < 0 {
		return strings.TrimSpace(strings.Split(remaining, "\n")[0])
	}
	return cleanInstructionText(remaining[:end])
}

func cleanInstructionText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "：:，,。.;； \t\r\n")
	return value
}

func distanceValue(section string, direction string) int {
	pattern := fmt.Sprintf(`距%s边缘\s*([0-9]+)\s*px`, regexp.QuoteMeta(direction))
	matches := regexp.MustCompile(pattern).FindStringSubmatch(section)
	if len(matches) == 2 {
		value, _ := strconv.Atoi(matches[1])
		return value
	}
	return 0
}

func fontSizeValue(section string) int {
	matches := regexp.MustCompile(`字号[:：]?\s*([0-9]+)`).FindStringSubmatch(section)
	if len(matches) == 2 {
		value, _ := strconv.Atoi(matches[1])
		return value
	}
	return 0
}

func fontSizeNear(section string, text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return fontSizeValue(section)
	}
	index := strings.Index(section, text)
	if index < 0 {
		return fontSizeValue(section)
	}
	fragment := section[index:]
	if next := strings.Index(fragment, "\n"); next >= 0 {
		fragment = fragment[:next]
	}
	if value := fontSizeValue(fragment); value > 0 {
		return value
	}
	return fontSizeValue(section)
}

func firstHexColor(section string) string {
	colors := hexColors(section)
	if len(colors) == 0 {
		return ""
	}
	return colors[0]
}

func lastHexColor(section string) string {
	colors := hexColors(section)
	if len(colors) == 0 {
		return ""
	}
	return colors[len(colors)-1]
}

func firstHexColorAfter(section string, marker string) string {
	index := strings.Index(section, marker)
	if index < 0 {
		return firstHexColor(section)
	}
	fragment := section[index:]
	if next := strings.Index(fragment, "\n"); next >= 0 {
		fragment = fragment[:next]
	}
	if value := firstHexColor(fragment); value != "" {
		return value
	}
	return firstHexColor(section)
}

func hexColors(section string) []string {
	matches := regexp.MustCompile(`#([0-9a-fA-F]{6})`).FindAllString(section, -1)
	return matches
}

func colorOrDefault(hex string, fallback color.RGBA) color.RGBA {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return fallback
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return fallback
	}
	return color.RGBA{
		R: uint8(value >> 16),
		G: uint8((value >> 8) & 0xff),
		B: uint8(value & 0xff),
		A: 255,
	}
}

func intOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func floatOrDefault(value int, fallback float64) float64 {
	if value > 0 {
		return float64(value)
	}
	return fallback
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
	drawStringWithColor(canvas, face, x, y, text, color.RGBA{255, 250, 240, 255})
}

func drawStringWithColor(canvas *image.RGBA, face font.Face, x int, y int, text string, color color.RGBA) {
	drawer := font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(color),
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
		parsed, err := parseOpenTypeFont(data)
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

func parseOpenTypeFont(data []byte) (*opentype.Font, error) {
	parsed, err := opentype.Parse(data)
	if err == nil {
		return parsed, nil
	}
	collection, collectionErr := opentype.ParseCollection(data)
	if collectionErr != nil {
		return nil, err
	}
	for index := 0; index < collection.NumFonts(); index++ {
		parsed, fontErr := collection.Font(index)
		if fontErr == nil {
			return parsed, nil
		}
	}
	return nil, err
}

func fontCandidates() []string {
	return []string{
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "simhei.ttf"),
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "msyh.ttc"),
		filepath.Join(os.Getenv("WINDIR"), "Fonts", "simsun.ttc"),
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
