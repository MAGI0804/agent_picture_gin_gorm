package workflow

import (
	"gin-biz-web-api/internal/service/agent_v2/agents"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

// ImageGenerationArtifactStore is the persistence surface shared by AI edit and final bitmap composition.
type ImageGenerationArtifactStore interface {
	agents.ImageEditStore
	agents.CompositionStore
}

// ImageGenerationWorkflowOptions wires the first real text-to-image workflow.
type ImageGenerationWorkflowOptions struct {
	Registry            *tools.Registry
	ArtifactWriter      ImageGenerationArtifactStore
	TextModelConfigID   uint
	ImageModelConfigID  uint
	VisionModelConfigID uint
	OCRModelConfigID    uint
	CandidateCount      int
	ModelProvider       string
	ModelName           string
}

// MockImageGenerationWorkflow 创建一个模拟的图片生成工作流
func MockImageGenerationWorkflow() Workflow {
	return Sequential(
		"image_generation_v2",
		"0.1.0",
		agents.NewMockAgent("intent_router", "已识别为图片生成任务", map[string]interface{}{
			"task_type": "image_generation",
			"intent":    "mock_image_generation",
		}),
		agents.NewMockAgent("requirement_agent", "已提取首版图片需求", map[string]interface{}{
			"need_clarification": false,
		}),
		agents.NewMockAgent("memory_agent", "已加载占位记忆上下文", map[string]interface{}{
			"memory_count": 0,
		}),
		agents.NewMockAgent("prompt_agent", "已生成占位提示词包", map[string]interface{}{
			"positive_prompt": "mock prompt for first-day v2 runtime skeleton",
		}),
	)
}

// ImageGenerationWorkflow creates the first real V2 image generation workflow.
func ImageGenerationWorkflow(options ImageGenerationWorkflowOptions) Workflow {
	return Sequential(
		"image_generation_v2",
		"0.7.0",
		agents.NewIntentRouterAgent(),
		agents.NewRequirementAgentWithRequiredText(options.Registry, options.TextModelConfigID),
		agents.NewPromptAgentWithRequiredText(options.Registry, options.TextModelConfigID),
		agents.NewImageEditAgent(options.Registry, options.ArtifactWriter, agents.ImageEditAgentOptions{
			ImageModelConfigID: options.ImageModelConfigID,
			CandidateCount:     options.CandidateCount,
			ModelProvider:      options.ModelProvider,
			ModelName:          options.ModelName,
		}),
		agents.NewImageCompositionAgent(options.ArtifactWriter),
	)
}
