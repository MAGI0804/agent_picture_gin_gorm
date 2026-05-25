package tools

import (
	"context"
	"errors"
	"fmt"
)

const (
	KindText            = "text"
	KindImageGeneration = "image_generation"
	KindImageEdit       = "image_edit"
	KindVision          = "vision"
	KindOCR             = "ocr"
	KindSegmentation    = "segmentation"
	KindSafety          = "safety"
)

// Capability describes a tool's limits and supported features.
type Capability struct {
	MaxPromptChars     int
	SupportedRatios    []string
	SupportsImageInput bool
	SupportsMask       bool
	MaxCandidates      int
	CostPolicy         string
}

// Tool is a registered model/provider capability.
type Tool struct {
	Name                    string
	Kind                    string
	Provider                string
	Model                   string
	ModelConfigID           uint
	Capability              Capability
	TextProvider            TextProvider
	ImageGenerationProvider ImageGenerationProvider
	ImageEditProvider       ImageEditProvider
	VisionProvider          VisionProvider
	OCRProvider             OCRProvider
	SegmentationProvider    SegmentationProvider
	SafetyProvider          SafetyProvider
}

// FindToolRequest describes a capability lookup.
type FindToolRequest struct {
	Kind          string
	UserID        uint
	ModelConfigID uint
}

// Registry stores available V2 tools by capability.
type Registry struct {
	tools []Tool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: []Tool{}}
}

// Register adds a tool after validating it has the provider required by its kind.
func (registry *Registry) Register(tool Tool) error {
	if tool.Name == "" {
		return errors.New("tool name is required")
	}
	if tool.Kind == "" {
		return errors.New("tool kind is required")
	}
	if !tool.hasProviderForKind() {
		return fmt.Errorf("tool %q has no provider for kind %q", tool.Name, tool.Kind)
	}
	registry.tools = append(registry.tools, tool)
	return nil
}

// FindTool returns the first tool matching kind and optional model config.
func (registry *Registry) FindTool(request FindToolRequest) (Tool, error) {
	if request.Kind == "" {
		return Tool{}, errors.New("tool kind is required")
	}
	for _, tool := range registry.tools {
		if tool.Kind != request.Kind {
			continue
		}
		if request.ModelConfigID > 0 && tool.ModelConfigID != request.ModelConfigID {
			continue
		}
		return tool, nil
	}
	return Tool{}, fmt.Errorf("tool kind %q was not found", request.Kind)
}

func (tool Tool) hasProviderForKind() bool {
	switch tool.Kind {
	case KindText:
		return tool.TextProvider != nil
	case KindImageGeneration:
		return tool.ImageGenerationProvider != nil
	case KindImageEdit:
		return tool.ImageEditProvider != nil
	case KindVision:
		return tool.VisionProvider != nil
	case KindOCR:
		return tool.OCRProvider != nil
	case KindSegmentation:
		return tool.SegmentationProvider != nil
	case KindSafety:
		return tool.SafetyProvider != nil
	default:
		return false
	}
}

// TextProvider is the V2 text generation provider boundary.
type TextProvider interface {
	GenerateText(ctx context.Context, request TextRequest) (TextResult, error)
}

// ImageGenerationProvider is the V2 text-to-image provider boundary.
type ImageGenerationProvider interface {
	GenerateImage(ctx context.Context, request ImageGenerationRequest) (ImageGenerationResult, error)
}

// ImageEditProvider is the V2 image editing provider boundary.
type ImageEditProvider interface {
	EditImage(ctx context.Context, request ImageEditRequest) (ImageEditResult, error)
}

// VisionProvider is the V2 visual understanding provider boundary.
type VisionProvider interface {
	AnalyzeImage(ctx context.Context, request VisionRequest) (VisionResult, error)
}

// OCRProvider is the V2 OCR provider boundary.
type OCRProvider interface {
	ExtractText(ctx context.Context, request OCRRequest) (OCRResult, error)
}

// SegmentationProvider is the V2 segmentation provider boundary.
type SegmentationProvider interface {
	SegmentImage(ctx context.Context, request SegmentationRequest) (SegmentationResult, error)
}

// SafetyProvider is the V2 content safety provider boundary.
type SafetyProvider interface {
	CheckContent(ctx context.Context, request SafetyRequest) (SafetyResult, error)
}

type TextRequest struct {
	Prompt string
}

type TextResult struct {
	Text string
}

type ImageGenerationRequest struct {
	Prompt         string
	NegativePrompt string
	AspectRatio    string
	CandidateCount int
}

type ImageGenerationResult struct {
	Images []GeneratedImage
}

type GeneratedImage struct {
	ObjectKey  string
	PreviewURL string
	Hash       string
}

type ImageEditRequest struct {
	Prompt    string
	ImageRefs []string
	MaskRef   string
}

type ImageEditResult struct {
	Images []GeneratedImage
}

type VisionRequest struct {
	ImageRef string
	Prompt   string
}

type VisionResult struct {
	Summary string
	Scores  map[string]float64
}

type OCRRequest struct {
	ImageRef string
}

type OCRResult struct {
	Text string
}

type SegmentationRequest struct {
	ImageRef string
	Prompt   string
}

type SegmentationResult struct {
	MaskObjectKey string
}

type SafetyRequest struct {
	Text     string
	ImageRef string
}

type SafetyResult struct {
	Allowed bool
	Reason  string
}
