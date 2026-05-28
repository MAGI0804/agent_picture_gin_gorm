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

// Capability 描述工具的限制和支持的功能。
type Capability struct {
	MaxPromptChars     int
	SupportedRatios    []string
	SupportsImageInput bool
	SupportsMask       bool
	MaxCandidates      int
	CostPolicy         string
}

// Tool 是一个已注册的模型/提供商能力。
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

// FindToolRequest 描述一个能力查找请求。
type FindToolRequest struct {
	Kind          string
	UserID        uint
	ModelConfigID uint
}

// Registry 按能力存储可用的 V2 工具。
type Registry struct {
	tools []Tool
}

// NewRegistry 创建一个空的注册表。
func NewRegistry() *Registry {
	return &Registry{tools: []Tool{}}
}

// Register 在验证工具具有其类型所需的提供商后添加工具。
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

// FindTool 返回匹配类型和可选模型配置的第一个工具。
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

// TextProvider 是 V2 文本生成提供商接口。
type TextProvider interface {
	GenerateText(ctx context.Context, request TextRequest) (TextResult, error)
}

// ImageGenerationProvider 是 V2 文本到图像提供商接口。
type ImageGenerationProvider interface {
	GenerateImage(ctx context.Context, request ImageGenerationRequest) (ImageGenerationResult, error)
}

// ImageEditProvider 是 V2 图像编辑提供商接口。
type ImageEditProvider interface {
	EditImage(ctx context.Context, request ImageEditRequest) (ImageEditResult, error)
}

// VisionProvider 是 V2 视觉理解提供商接口。
type VisionProvider interface {
	AnalyzeImage(ctx context.Context, request VisionRequest) (VisionResult, error)
}

// OCRProvider 是 V2 OCR 提供商接口。
type OCRProvider interface {
	ExtractText(ctx context.Context, request OCRRequest) (OCRResult, error)
}

// SegmentationProvider 是 V2 分割提供商接口。
type SegmentationProvider interface {
	SegmentImage(ctx context.Context, request SegmentationRequest) (SegmentationResult, error)
}

// SafetyProvider 是 V2 内容安全提供商接口。
type SafetyProvider interface {
	CheckContent(ctx context.Context, request SafetyRequest) (SafetyResult, error)
}

type TextRequest struct {
	UserID   uint
	RunID    uint
	StepID   uint
	System   string
	Prompt   string
	Messages []TextMessage
}

type TextMessage struct {
	Role    string
	Content string
}

type TextResult struct {
	Text      string
	Reasoning string
}

type ImageGenerationRequest struct {
	UserID              uint
	ConversationID      uint
	RunID               uint
	StepID              uint
	TaskType            string
	Intent              string
	Prompt              string
	NegativePrompt      string
	AspectRatio         string
	CandidateCount      int
	CandidateStartIndex int
	Temperature         string
}

type ImageGenerationResult struct {
	Images []GeneratedImage
}

type GeneratedImage struct {
	Name       string
	Kind       string
	MimeType   string
	ObjectKey  string
	PreviewURL string
	SizeBytes  int64
	Hash       string
}

type ImageEditRequest struct {
	UserID         uint
	ConversationID uint
	RunID          uint
	StepID         uint
	TaskType       string
	Prompt         string
	ImageRefs      []string
	MaskRef        string
	CandidateCount int
}

type ImageEditResult struct {
	Images []GeneratedImage
}

type VisionRequest struct {
	UserID   uint
	RunID    uint
	StepID   uint
	ImageRef string
	Prompt   string
}

type VisionResult struct {
	Summary      string
	Scores       map[string]float64
	Issues       []string
	ShouldRefine bool
}

type OCRRequest struct {
	UserID   uint
	RunID    uint
	StepID   uint
	ImageRef string
	Prompt   string
}

type OCRResult struct {
	Text            string
	TextReadability float64
	LayoutScore     float64
	Issues          []string
	ShouldRefine    bool
}

type SegmentationRequest struct {
	ImageRef string
	Prompt   string
}

type SegmentationResult struct {
	MaskObjectKey string
}

type SafetyRequest struct {
	UserID   uint
	RunID    uint
	StepID   uint
	Text     string
	ImageRef string
}

type SafetyResult struct {
	Allowed bool
	Reason  string
}
