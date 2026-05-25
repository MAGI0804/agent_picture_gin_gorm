package workflow

import "gin-biz-web-api/internal/service/agent_v2/agents"

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
