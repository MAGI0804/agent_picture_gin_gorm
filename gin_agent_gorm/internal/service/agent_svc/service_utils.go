package agent_svc

import (
	"encoding/json"

	"gin-biz-web-api/model"
)

// createStep 记录一个 Agent 子步骤。
func (svc *AgentService) createStep(runID uint, name string, input string, output string) error {
	return svc.createStepWithThinking(runID, name, input, output, defaultStepThinkContent(name), "")
}

// createStepWithThinking 记录一个带业务思考和模型推理内容的 Agent 子步骤。
func (svc *AgentService) createStepWithThinking(runID uint, name string, input string, output string, thinkContent string, reasoningContent string) error {
	step := model.AgentStep{
		AgentRunID:       runID,
		Name:             name,
		Status:           "completed",
		Input:            input,
		Output:           output,
		ThinkContent:     thinkContent,
		ReasoningContent: reasoningContent,
	}
	return svc.dao.CreateAgentStep(&step)
}

func (svc *AgentService) createStepsWithThinking(runID uint, steps []stepRecord) error {
	agentSteps := make([]model.AgentStep, 0, len(steps))
	for _, step := range steps {
		agentSteps = append(agentSteps, model.AgentStep{
			AgentRunID:       runID,
			Name:             step.name,
			Status:           "completed",
			Input:            step.input,
			Output:           step.output,
			ThinkContent:     step.thinkContent,
			ReasoningContent: step.reasoningContent,
		})
	}
	return svc.dao.CreateAgentSteps(agentSteps)
}

type stepRecord struct {
	name             string
	input            string
	output           string
	thinkContent     string
	reasoningContent string
}

func defaultStepThinkContent(name string) string {
	switch name {
	case "planner_agent":
		return "分析用户意图并决定是否需要补充问题。"
	case "context_agent":
		return "读取上下文记忆，为后续 Agent 提供背景。"
	case "prompt_agent":
		return "整理任务输入并生成模型提示词。"
	case "image_agent":
		return "调用图片生成能力并准备图片产物。"
	case "html_agent":
		return "调用 HTML 生成能力并准备页面产物。"
	case "review_agent":
		return "检查产物质量和可用性。"
	case "artifact_agent":
		return "保存产物文件并写入下载元数据。"
	default:
		return "执行当前 Agent 子步骤。"
	}
}

func modelRequestPayloadSummary(stream bool, returnReasoning bool, temperature string) string {
	payload := map[string]interface{}{
		"stream":           stream,
		"temperature":      parseTemperature(temperature),
		"return_reasoning": returnReasoning,
	}
	body, _ := json.Marshal(payload)
	return string(body)
}
