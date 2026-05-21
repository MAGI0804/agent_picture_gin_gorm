package model_request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"gin-biz-web-api/model"
)

var deepseekHTTPClient = &http.Client{Timeout: 120 * time.Second}

// DeepseekChatRequest Deepseek 模型聊天请求参数。
type DeepseekChatRequest struct {
	System          string            `json:"system"`
	Messages        []DeepseekMessage `json:"messages"`
	Stream          bool              `json:"stream"`
	ReturnReasoning bool              `json:"return_reasoning"`
	Temperature     float64           `json:"temperature"`
	MaxTokens       int               `json:"max_tokens"`
}

// DeepseekMessage Deepseek 消息结构。
type DeepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepseekChatResult Deepseek 模型聊天响应结果。
// 封装了文本内容和思考过程。
type DeepseekChatResult struct {
	Content          string `json:"content"`           // 模型返回的文本内容
	ReasoningContent string `json:"reasoning_content"` // 模型的思考过程
	FinishReason     string `json:"finish_reason"`     // 结束原因
}

// SendDeepseekRequest 发送 Deepseek-v4-pro 模型请求。
//
// 参数:
//   - url: Deepseek API 的基础地址
//   - apiKey: API 密钥
//   - modelName: 模型名称（如 "deepseek-v4-pro"）
//   - request: 聊天请求参数
//
// 返回:
//   - DeepseekChatResult: 包含文本内容和思考过程的结果对象
//   - error: 错误信息
func SendDeepseekRequest(url, apiKey, modelName string, request DeepseekChatRequest) (DeepseekChatResult, error) {
	if strings.TrimSpace(url) == "" {
		return DeepseekChatResult{}, errors.New("url cannot be empty")
	}
	if strings.TrimSpace(apiKey) == "" {
		return DeepseekChatResult{}, errors.New("apiKey cannot be empty")
	}
	if strings.TrimSpace(modelName) == "" {
		return DeepseekChatResult{}, errors.New("modelName cannot be empty")
	}

	endpoint := buildEndpoint(url)

	payload := map[string]interface{}{
		"model":            modelName,
		"stream":           request.Stream,
		"return_reasoning": request.ReturnReasoning,
		"temperature":      request.Temperature,
	}

	if request.MaxTokens > 0 {
		payload["max_tokens"] = request.MaxTokens
	}

	if strings.TrimSpace(request.System) != "" {
		payload["system"] = request.System
	}

	if len(request.Messages) > 0 {
		messages := make([]map[string]string, 0, len(request.Messages))
		for _, msg := range request.Messages {
			role := normalizeRole(msg.Role)
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			messages = append(messages, map[string]string{
				"role":    role,
				"content": content,
			})
		}
		if len(messages) == 0 {
			return DeepseekChatResult{}, errors.New("no valid messages in request")
		}
		payload["messages"] = messages
	} else {
		return DeepseekChatResult{}, errors.New("messages cannot be empty")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return DeepseekChatResult{}, errors.Wrap(err, "failed to marshal payload")
	}

	httpRequest, err := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return DeepseekChatResult{}, errors.Wrap(err, "failed to create request")
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := deepseekHTTPClient.Do(httpRequest)
	if err != nil {
		return DeepseekChatResult{}, errors.Wrap(err, "failed to send request")
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return DeepseekChatResult{}, errors.Wrap(err, "failed to read response")
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return DeepseekChatResult{}, errors.Errorf("API request failed with status %d: %s", response.StatusCode, truncate(string(body), 500))
	}

	return parseDeepseekResponse(body)
}

// SendDeepseekSimple 发送简化的 Deepseek-v4-pro 请求。
// 适用于简单的文本对话场景，自动构建请求结构。
//
// 参数:
//   - url: Deepseek API 的基础地址
//   - apiKey: API 密钥
//   - modelName: 模型名称（如 "deepseek-v4-pro"）
//   - userPrompt: 用户输入的提示词
//   - returnReasoning: 是否返回思考过程
//
// 返回:
//   - model.TextThinkingModelOutput: 封装的文本思考模型输出对象
//   - error: 错误信息
func SendDeepseekSimple(url, apiKey, modelName, userPrompt string, returnReasoning bool) (model.TextThinkingModelOutput, error) {
	request := DeepseekChatRequest{
		Messages: []DeepseekMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream:          false,
		ReturnReasoning: returnReasoning,
		Temperature:     0.7,
		MaxTokens:       4096,
	}

	result, err := SendDeepseekRequest(url, apiKey, modelName, request)
	if err != nil {
		return model.TextThinkingModelOutput{}, err
	}

	return model.TextThinkingModelOutput{
		ModelName:       modelName,
		Content:         result.Content,
		ThinkingContent: result.ReasoningContent,
		FinishReason:    result.FinishReason,
		Usage: model.TokenUsage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
		IsStreaming: false,
	}, nil
}

func buildEndpoint(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

func parseDeepseekResponse(body []byte) (DeepseekChatResult, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		streamResult := parseStreamingResponse(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return DeepseekChatResult{}, errors.Wrap(err, "failed to unmarshal response")
	}

	if response.Error != nil {
		return DeepseekChatResult{}, errors.Errorf("API error: %s", response.Error.Message)
	}

	if len(response.Choices) == 0 {
		streamResult := parseStreamingResponse(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return DeepseekChatResult{}, errors.New("no choices in response")
	}

	return DeepseekChatResult{
		Content:          strings.TrimSpace(response.Choices[0].Message.Content),
		ReasoningContent: strings.TrimSpace(response.Choices[0].Message.ReasoningContent),
		FinishReason:     response.Choices[0].FinishReason,
	}, nil
}

func parseStreamingResponse(body []byte) DeepseekChatResult {
	lines := strings.Split(string(body), "\n")
	var contentParts []string
	var reasoningParts []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		if choices, ok := event["choices"].([]interface{}); ok {
			for _, choice := range choices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					if delta, ok := choiceMap["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							contentParts = append(contentParts, content)
						}
						if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
							reasoningParts = append(reasoningParts, reasoning)
						}
					}
				}
			}
		}
	}

	return DeepseekChatResult{
		Content:          strings.Join(contentParts, ""),
		ReasoningContent: strings.Join(reasoningParts, ""),
	}
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

// OptimizePromptWithDeepseek 使用 deepseek-v4-pro 智能优化提示词
// 功能：智能优化图片生成提示词，保持关键信息的同时使其更专业、更有效
//
// 参数:
//   - url: Deepseek API 的基础地址
//   - apiKey: API 密钥
//   - originalPrompt: 原始提示词
//   - targetLength: 目标最大字符数（建议 800）
//   - optimizationType: 优化类型："shorten"（缩短）或 "enhance"（增强）
//
// 返回:
//   - string: 优化后的提示词
//   - error: 错误信息
func OptimizePromptWithDeepseek(url, apiKey, originalPrompt string, targetLength int, optimizationType string) (string, error) {
	if strings.TrimSpace(url) == "" {
		return "", errors.New("url cannot be empty")
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("apiKey cannot be empty")
	}
	if strings.TrimSpace(originalPrompt) == "" {
		return "", errors.New("originalPrompt cannot be empty")
	}

	var systemPrompt string
	var userPrompt string

	if optimizationType == "shorten" {
		// 缩短模式：在保持关键元素的情况下缩短提示词
		systemPrompt = strings.Join([]string{
			"You are an expert prompt engineer specialized in optimizing image generation prompts.",
			"Your task: shorten the given prompt while preserving ALL critical visual information.",
			fmt.Sprintf("CRITICAL: Try your best to make the prompt under %d characters long.", targetLength),
			"Keep ALL key elements: subject, composition, style, colors, lighting, text placements, dimensions, etc.",
			"Make it concise but keep full visual meaning.",
			"Return ONLY the optimized prompt, no explanations.",
		}, " ")
		userPrompt = fmt.Sprintf("Please shorten this image generation prompt while keeping ALL important details. Try to make it under %d characters:\n\n%s",
			targetLength, originalPrompt)
	} else {
		// 增强模式：优化和增强提示词
		systemPrompt = strings.Join([]string{
			"You are an expert prompt engineer specialized in optimizing image generation prompts.",
			"Your task: enhance and optimize the given prompt for better image generation results.",
			fmt.Sprintf("CRITICAL: Try your best to make the prompt under %d characters long.", targetLength),
			"Improve clarity, add useful artistic/style details where appropriate, but keep the core intent.",
			"Return ONLY the optimized prompt, no explanations.",
		}, " ")
		userPrompt = fmt.Sprintf("Please optimize and enhance this image generation prompt. Try to make it under %d characters:\n\n%s",
			targetLength, originalPrompt)
	}

	request := DeepseekChatRequest{
		System: systemPrompt,
		Messages: []DeepseekMessage{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream:          false,
		ReturnReasoning: false,
		Temperature:     0.7, // 使用适中的温度
		MaxTokens:       1000,
	}

	result, err := SendDeepseekRequest(url, apiKey, "deepseek-v4-pro", request)
	if err != nil {
		return "", errors.Wrap(err, "failed to optimize prompt with Deepseek")
	}

	optimizedPrompt := strings.TrimSpace(result.Content)
	if optimizedPrompt == "" {
		return "", errors.New("optimized prompt is empty")
	}

	return optimizedPrompt, nil
}

// ShortenImagePrompt 专用方法：智能缩短图片生成提示词到目标长度
// 这是 OptimizePromptWithDeepseek 的便捷包装方法
func ShortenImagePrompt(url, apiKey, originalPrompt string, maxLength int) (string, error) {
	return OptimizePromptWithDeepseek(url, apiKey, originalPrompt, maxLength, "shorten")
}
