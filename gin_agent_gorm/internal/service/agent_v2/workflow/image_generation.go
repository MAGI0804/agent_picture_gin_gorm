package workflow

import (
	"gin-biz-web-api/internal/service/agent_v2/agents"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/internal/service/agent_v2/tools"
)

// ImageGenerationWorkflowOptions wires the first real text-to-image workflow.
type ImageGenerationWorkflowOptions struct {
	Registry            *tools.Registry
	ArtifactWriter      agents.ArtifactWriter
	ImageModelConfigID  uint
	VisionModelConfigID uint
	CandidateCount      int
	ModelProvider       string
	ModelName           string
}

// MockImageGenerationWorkflow 创建一个模拟的图片生成工作流
func MockImageGenerationWorkflow() Workflow {
	return Sequential(
		"image_generation_v2",
		"0.1.0",
		agents.NewMockAgent("intent_router", "classified request as image_generation", map[string]interface{}{
			"task_type": "image_generation",
			"intent":    "mock_image_generation",
		}),
		agents.NewMockAgent("requirement_agent", "extracted first-pass image requirements", map[string]interface{}{
			"need_clarification": false,
		}),
		agents.NewMockAgent("memory_agent", "loaded placeholder memory context", map[string]interface{}{
			"memory_count": 0,
		}),
		agents.NewMockAgent("prompt_agent", "prepared placeholder prompt bundle", map[string]interface{}{
			"positive_prompt": "mock prompt for first-day v2 runtime skeleton",
		}),
	)
}

// ImageGenerationWorkflow creates the first real V2 image generation workflow.
func ImageGenerationWorkflow(options ImageGenerationWorkflowOptions) Workflow {
	var reviewNode domain.AgentNode = agents.NewMockVisionReviewAgent(0.7)
	if options.Registry != nil && options.VisionModelConfigID > 0 {
		reviewNode = agents.NewVisionReviewAgent(options.Registry, agents.VisionReviewAgentOptions{
			VisionModelConfigID: options.VisionModelConfigID,
			MinPassingScore:     0.7,
		})
	}
	return Sequential(
		"image_generation_v2",
		"0.3.0",
		agents.NewIntentRouterAgent(),
		agents.NewRequirementAgent(),
		agents.NewMemoryAgent(),
		agents.NewPromptAgent(),
		agents.NewImageGenerationAgent(options.Registry, agents.ImageGenerationAgentOptions{
			ImageModelConfigID: options.ImageModelConfigID,
			CandidateCount:     options.CandidateCount,
		}),
		agents.NewArtifactAgent(options.ArtifactWriter, agents.ArtifactAgentOptions{
			ModelProvider: options.ModelProvider,
			ModelName:     options.ModelName,
		}),
		reviewNode,
	)
}
