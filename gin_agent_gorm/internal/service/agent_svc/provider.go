package agent_svc

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"gin-biz-web-api/internal/service/model_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/logger"
)

type GenerationRequest struct {
	Prompt          string
	Intent          string
	TaskType        string
	Stream          bool
	ReturnReasoning bool
	Temperature     string
	ModelConfig     model.UserModelConfig
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	System          string
	Messages        []ChatMessage
	ModelConfig     model.UserModelConfig
	Stream          bool
	ReturnReasoning bool
}

type ChatResult struct {
	Content          string
	ReasoningContent string
}

type GeneratedFile struct {
	Name     string
	Kind     string
	MimeType string
	Content  []byte
}

type Provider interface {
	Chat(request ChatRequest) (ChatResult, error)
	Generate(request GenerationRequest) ([]GeneratedFile, error)
}

type HTTPProvider struct {
	config model.UserModelConfig
	client *http.Client
}

func NewProvider() Provider {
	return &HTTPProvider{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func NewProviderWithConfig(config model.UserModelConfig) Provider {
	return &HTTPProvider{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (provider *HTTPProvider) Chat(request ChatRequest) (ChatResult, error) {
	providerName := strings.ToLower(strings.TrimSpace(provider.config.Provider))
	baseURL := strings.TrimSpace(provider.config.BaseURL)
	if providerName == "" || providerName == "mock" {
		return ChatResult{}, errors.New("未配置真实文本模型，请在全局模型配置中选择 deepseek/openai/anthropic 等真实模型")
	}
	if isDeepseekChatProvider(providerName, baseURL) {
		return provider.chatDeepseek(request, baseURL)
	}
	if isAnthropicProvider(providerName, provider.config) {
		if strings.TrimSpace(provider.config.AnthropicBaseURL) != "" {
			baseURL = strings.TrimSpace(provider.config.AnthropicBaseURL)
		}
		return provider.chatAnthropic(baseURL, request)
	}
	return provider.chatOpenAICompatible(baseURL, request)
}

func (provider *HTTPProvider) chatDeepseek(request ChatRequest, baseURL string) (ChatResult, error) {
	apiKey := strings.TrimSpace(provider.config.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(provider.config.AnthropicAuthToken)
	}
	modelName := strings.TrimSpace(provider.config.ChatModel)
	if modelName == "" {
		modelName = strings.TrimSpace(provider.config.AnthropicModel)
	}
	messages := make([]model_request.DeepseekMessage, 0, len(request.Messages))
	for _, message := range request.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, model_request.DeepseekMessage{
			Role:    normalizeChatRole(message.Role),
			Content: content,
		})
	}
	result, err := model_request.SendDeepseekRequest(baseURL, apiKey, modelName, model_request.DeepseekChatRequest{
		System:          request.System,
		Messages:        messages,
		Stream:          request.Stream,
		ReturnReasoning: request.ReturnReasoning,
		Temperature:     parseTemperature(provider.config.Temperature),
		MaxTokens:       parseMaxTokens(provider.config.ClaudeCodeMaxOutputTokens, 4096),
	})
	if err != nil {
		return ChatResult{}, err
	}
	return ChatResult{
		Content:          result.Content,
		ReasoningContent: result.ReasoningContent,
	}, nil
}

func (provider *HTTPProvider) chatAnthropic(baseURL string, request ChatRequest) (ChatResult, error) {
	apiKey := strings.TrimSpace(provider.config.AnthropicAuthToken)
	if apiKey == "" {
		apiKey = strings.TrimSpace(provider.config.APIKey)
	}
	if apiKey == "" {
		return ChatResult{}, errors.New("model api key is empty")
	}

	modelName := strings.TrimSpace(provider.config.AnthropicModel)
	if modelName == "" {
		modelName = strings.TrimSpace(provider.config.ChatModel)
	}
	if modelName == "" {
		return ChatResult{}, errors.New("chat model is empty")
	}
	if strings.TrimSpace(baseURL) == "" {
		return ChatResult{}, errors.New("anthropic base url is empty")
	}

	payloadMessages := make([]map[string]interface{}, 0, len(request.Messages))
	for _, message := range request.Messages {
		role := normalizeChatRole(message.Role)
		if role == "system" {
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		payloadMessages = append(payloadMessages, map[string]interface{}{
			"role":    role,
			"content": content,
		})
	}
	if len(payloadMessages) == 0 {
		return ChatResult{}, errors.New("chat messages are empty")
	}

	payload := map[string]interface{}{
		"model":            modelName,
		"max_tokens":       parseMaxTokens(provider.config.ClaudeCodeMaxOutputTokens, 4096),
		"temperature":      parseTemperature(provider.config.Temperature),
		"system":           request.System,
		"messages":         payloadMessages,
		"stream":           request.Stream,
		"return_reasoning": request.ReturnReasoning,
	}

	endpoint := joinBaseURL(baseURL, "v1/messages")
	body, err := provider.doJSON("POST", endpoint, payload, modelName, "anthropic", func(req *http.Request) {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		if strings.Contains(strings.ToLower(baseURL), "deepseek") {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	})
	if err != nil {
		return ChatResult{}, err
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		ReasoningContent string `json:"reasoning_content"`
		Error            *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		streamResult := parseStreamingChatResult(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return ChatResult{}, errors.Wrap(err, "decode anthropic response")
	}
	if response.Error != nil {
		return ChatResult{}, errors.Errorf("model api error: %s", response.Error.Message)
	}
	parts := make([]string, 0, len(response.Content))
	for _, item := range response.Content {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, item.Text)
		}
	}
	content := strings.TrimSpace(strings.Join(parts, "\n"))
	if content == "" {
		streamResult := parseStreamingChatResult(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return ChatResult{}, errors.New("model returned empty response")
	}
	return ChatResult{Content: content, ReasoningContent: strings.TrimSpace(response.ReasoningContent)}, nil
}

func (provider *HTTPProvider) chatOpenAICompatible(baseURL string, request ChatRequest) (ChatResult, error) {
	apiKey := strings.TrimSpace(provider.config.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(provider.config.AnthropicAuthToken)
	}
	if apiKey == "" {
		return ChatResult{}, errors.New("model api key is empty")
	}

	modelName := strings.TrimSpace(provider.config.ChatModel)
	if modelName == "" {
		modelName = strings.TrimSpace(provider.config.AnthropicModel)
	}
	if modelName == "" {
		return ChatResult{}, errors.New("chat model is empty")
	}
	if strings.TrimSpace(baseURL) == "" {
		return ChatResult{}, errors.New("model base url is empty")
	}

	payloadMessages := make([]map[string]string, 0, len(request.Messages)+1)
	if strings.TrimSpace(request.System) != "" {
		payloadMessages = append(payloadMessages, map[string]string{"role": "system", "content": request.System})
	}
	for _, message := range request.Messages {
		role := normalizeChatRole(message.Role)
		if role != "system" && role != "assistant" {
			role = "user"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		payloadMessages = append(payloadMessages, map[string]string{"role": role, "content": content})
	}
	if len(payloadMessages) == 0 {
		return ChatResult{}, errors.New("chat messages are empty")
	}

	payload := map[string]interface{}{
		"model":            modelName,
		"messages":         payloadMessages,
		"temperature":      parseTemperature(provider.config.Temperature),
		"stream":           request.Stream,
		"return_reasoning": request.ReturnReasoning,
	}

	endpoint := joinBaseURL(baseURL, "chat/completions")
	body, err := provider.doJSON("POST", endpoint, payload, modelName, "openai-compatible", func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	})
	if err != nil {
		return ChatResult{}, err
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		streamResult := parseStreamingChatResult(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return ChatResult{}, errors.Wrap(err, "decode openai-compatible response")
	}
	if response.Error != nil {
		return ChatResult{}, errors.Errorf("model api error: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		streamResult := parseStreamingChatResult(body)
		if strings.TrimSpace(streamResult.Content) != "" {
			return streamResult, nil
		}
		return ChatResult{}, errors.New("model returned empty response")
	}
	return ChatResult{
		Content:          strings.TrimSpace(response.Choices[0].Message.Content),
		ReasoningContent: strings.TrimSpace(response.Choices[0].Message.ReasoningContent),
	}, nil
}

func (provider *HTTPProvider) doJSON(method string, endpoint string, payload interface{}, modelName string, apiType string, applyHeaders func(*http.Request)) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "encode model request")
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, errors.Wrap(err, "create model request")
	}
	req.Header.Set("Content-Type", "application/json")
	applyHeaders(req)

	start := time.Now()
	logModelRequestStart(method, endpoint, provider.config.Provider, modelName, apiType)
	resp, err := provider.client.Do(req)
	if err != nil {
		logModelRequestError(method, endpoint, provider.config.Provider, modelName, apiType, time.Since(start), err)
		return nil, errors.Wrap(err, "call model api")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logModelRequestError(method, endpoint, provider.config.Provider, modelName, apiType, time.Since(start), err)
		return nil, errors.Wrap(err, "read model response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logModelRequestDone(method, endpoint, provider.config.Provider, modelName, apiType, resp.StatusCode, time.Since(start), len(body))
		return nil, errors.Errorf("model api http %d: %s", resp.StatusCode, truncateForError(string(body), 500))
	}
	logModelRequestDone(method, endpoint, provider.config.Provider, modelName, apiType, resp.StatusCode, time.Since(start), len(body))
	return body, nil
}

func logModelRequestStart(method string, endpoint string, providerName string, modelName string, apiType string) {
	if logger.Logger == nil {
		return
	}
	logger.Info("External Model Request Started",
		zap.String("method", method),
		zap.String("endpoint", endpoint),
		zap.String("provider", providerName),
		zap.String("model", modelName),
		zap.String("api_type", apiType),
	)
}

func logModelRequestDone(method string, endpoint string, providerName string, modelName string, apiType string, statusCode int, took time.Duration, responseBytes int) {
	if logger.Logger == nil {
		return
	}
	logger.Info("External Model Request Finished",
		zap.String("method", method),
		zap.String("endpoint", endpoint),
		zap.String("provider", providerName),
		zap.String("model", modelName),
		zap.String("api_type", apiType),
		zap.Int("status_code", statusCode),
		zap.Duration("took", took),
		zap.Int("response_bytes", responseBytes),
	)
}

func logModelRequestError(method string, endpoint string, providerName string, modelName string, apiType string, took time.Duration, err error) {
	if logger.Logger == nil {
		return
	}
	logger.Error("External Model Request Failed",
		zap.String("method", method),
		zap.String("endpoint", endpoint),
		zap.String("provider", providerName),
		zap.String("model", modelName),
		zap.String("api_type", apiType),
		zap.Duration("took", took),
		zap.Error(err),
	)
}

func isAnthropicProvider(providerName string, config model.UserModelConfig) bool {
	if strings.Contains(providerName, "anthropic") || strings.Contains(providerName, "claude") {
		return true
	}
	if providerName == "deepseek" {
		return strings.Contains(strings.ToLower(config.BaseURL), "anthropic") ||
			strings.Contains(strings.ToLower(config.AnthropicBaseURL), "anthropic")
	}
	if providerName != "" {
		return false
	}
	return strings.TrimSpace(config.AnthropicBaseURL) != "" && strings.TrimSpace(config.AnthropicModel) != ""
}

func isDeepseekChatProvider(providerName string, baseURL string) bool {
	if !strings.Contains(providerName, "deepseek") {
		return false
	}
	lowerBaseURL := strings.ToLower(baseURL)
	return !strings.Contains(lowerBaseURL, "anthropic")
}

func normalizeChatRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

func parseStreamingChatResult(body []byte) ChatResult {
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
		// 兼容 OpenAI/DeepSeek 流式返回：choices[].delta.content / reasoning_content。
		if choices, ok := event["choices"].([]interface{}); ok {
			for _, choice := range choices {
				choiceMap, _ := choice.(map[string]interface{})
				delta, _ := choiceMap["delta"].(map[string]interface{})
				contentParts = appendIfPresent(contentParts, delta["content"])
				reasoningParts = appendIfPresent(reasoningParts, delta["reasoning_content"])
			}
		}
		// 兼容 Anthropic 流式返回：content_block_delta.delta.text / thinking。
		if delta, ok := event["delta"].(map[string]interface{}); ok {
			contentParts = appendIfPresent(contentParts, delta["text"])
			reasoningParts = appendIfPresent(reasoningParts, delta["thinking"])
			reasoningParts = appendIfPresent(reasoningParts, delta["reasoning_content"])
		}
		contentParts = appendIfPresent(contentParts, event["content"])
		reasoningParts = appendIfPresent(reasoningParts, event["reasoning_content"])
	}
	return ChatResult{
		Content:          strings.TrimSpace(strings.Join(contentParts, "")),
		ReasoningContent: strings.TrimSpace(strings.Join(reasoningParts, "")),
	}
}

func appendIfPresent(parts []string, value interface{}) []string {
	text, ok := value.(string)
	if !ok || text == "" {
		return parts
	}
	return append(parts, text)
}

func parseTemperature(value string) float64 {
	temperature, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0.7
	}
	return temperature
}

func parseMaxTokens(value string, fallback int) int {
	maxTokens, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || maxTokens <= 0 {
		return fallback
	}
	return maxTokens
}

func joinBaseURL(baseURL string, endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpoint = strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(endpoint, "v1/") {
		endpoint = strings.TrimPrefix(endpoint, "v1/")
	}
	if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/openai") {
		return base + "/" + endpoint
	}
	return base + "/" + endpoint
}

func truncateForError(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func (provider *HTTPProvider) Generate(request GenerationRequest) ([]GeneratedFile, error) {
	providerName := strings.ToLower(strings.TrimSpace(provider.config.Provider))
	modelName := strings.TrimSpace(provider.config.ImageModel)
	apiKey := strings.TrimSpace(provider.config.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(provider.config.AnthropicAuthToken)
	}
	baseURL := strings.TrimSpace(provider.config.BaseURL)
	if modelName == "" || baseURL == "" {
		return nil, errors.New("未配置真实图片模型名称或请求地址")
	}

	if isJimengProvider(providerName, baseURL) {
		return provider.generateJimengImage(request, baseURL, modelName)
	}
	if apiKey == "" {
		return nil, errors.New("未配置真实图片模型 API Key 或鉴权信息")
	}
	if isDashScopeProvider(providerName, baseURL) {
		return provider.generateDashScopeImage(request, baseURL, apiKey, modelName)
	}

	payload := map[string]interface{}{
		"model":           modelName,
		"prompt":          request.Prompt,
		"n":               1,
		"size":            "1024x1024",
		"response_format": "b64_json",
	}
	endpoint := joinBaseURL(baseURL, "images/generations")
	body, err := provider.doJSON("POST", endpoint, payload, modelName, "image-openai-compatible", func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	})
	if err != nil {
		return nil, err
	}

	file, err := provider.parseOpenAIImageFile(body, request.Prompt)
	if err != nil {
		return nil, err
	}
	return []GeneratedFile{file}, nil
}

func (provider *HTTPProvider) generateJimengImage(
	request GenerationRequest,
	baseURL string,
	modelName string,
) ([]GeneratedFile, error) {
	// 记录即将发送的请求
	logger.Info("[Jimeng Provider] 准备发送图片生成请求",
		zap.String("baseURL", baseURL),
		zap.String("model_name", modelName),
		zap.String("prompt", request.Prompt),
		zap.Int("prompt_length", len([]rune(request.Prompt))),
	)

	config := model_request.JimengConfig{
		AccessKeyID:     runtimeConfigString(provider.config, "access_key_id", "ak", "access_key", "accesskey"),
		SecretAccessKey: runtimeConfigString(provider.config, "secret_access_key", "sk", "secret_key", "secretkey"),
		Region:          runtimeConfigString(provider.config, "region"),
		Service:         runtimeConfigString(provider.config, "service"),
	}

	// 记录配置信息（敏感信息已掩码）
	logger.Info("[Jimeng Provider] 配置信息",
		zap.String("access_key_id", maskString(config.AccessKeyID)),
		zap.String("region", config.Region),
		zap.String("service", config.Service),
	)

	if config.Region == "" {
		config.Region = "cn-north-1"
		logger.Info("[Jimeng Provider] 使用默认区域", zap.String("region", config.Region))
	}
	if config.Service == "" {
		config.Service = "cv"
		logger.Info("[Jimeng Provider] 使用默认服务", zap.String("service", config.Service))
	}
	reqKey := runtimeConfigString(provider.config, "req_key")
	if reqKey == "" {
		reqKey = model_request.JimengReqKey
	}

	// 准备最终的 prompt
	finalPrompt, err := normalizeJimengPrompt(request.Prompt)
	if err != nil {
		return nil, err
	}
	logger.Info("[Jimeng Provider] 最终发送的 prompt",
		zap.String("final_prompt", finalPrompt),
		zap.Int("final_prompt_length", len([]rune(finalPrompt))),
	)

	jimengRequest := model_request.JimengImageRequest{
		ReqKey:      reqKey,
		Prompt:      finalPrompt,
		Width:       runtimeConfigInt(provider.config, "width"),
		Height:      runtimeConfigInt(provider.config, "height"),
		Scale:       runtimeConfigInt(provider.config, "scale"),
		ForceSingle: runtimeConfigBool(provider.config, "force_single"),
		ReturnURL:   true,
	}

	// 记录完整的请求对象
	logger.Debug("[Jimeng Provider] 完整请求对象", zap.Any("request", jimengRequest))

	// 1. 提交任务
	submitResponse, err := model_request.SendJimengImageRequest(baseURL, config, jimengRequest)
	if err != nil {
		logger.Error("[Jimeng Provider] 提交任务失败", zap.Error(err))
		return nil, err
	}
	taskID := submitResponse.Data.TaskID
	logger.Info("[Jimeng Provider] 任务提交成功，开始轮询", zap.String("task_id", taskID))

	// 2. 轮询任务状态直到完成
	queryResponse, err := model_request.PollJimengTaskUntilComplete(baseURL, config, taskID, 150, 2*time.Second)
	if err != nil {
		logger.Error("[Jimeng Provider] 轮询任务失败", zap.Error(err))
		return nil, err
	}

	// 3. 处理生成的图片
	var files []GeneratedFile

	// 优先使用 image_urls
	if len(queryResponse.Data.ImageURLs) > 0 {
		logger.Info("[Jimeng Provider] 从URL下载图片", zap.Int("count", len(queryResponse.Data.ImageURLs)))
		for i, imgURL := range queryResponse.Data.ImageURLs {
			downloadedFile, err := provider.downloadGeneratedImage(imgURL)
			if err != nil {
				logger.Warn("[Jimeng Provider] 下载图片失败", zap.Int("index", i), zap.Error(err))
				continue
			}
			downloadedFile.Name = fmt.Sprintf("generated-image-%d.png", i+1)
			files = append(files, downloadedFile)
		}
	}

	// 如果没有 URL，尝试使用 base64
	if len(files) == 0 && len(queryResponse.Data.BinaryDataBase64) > 0 {
		logger.Info("[Jimeng Provider] 使用Base64解码图片", zap.Int("count", len(queryResponse.Data.BinaryDataBase64)))
		for i, base64Data := range queryResponse.Data.BinaryDataBase64 {
			imgData, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				logger.Warn("[Jimeng Provider] Base64解码失败", zap.Int("index", i), zap.Error(err))
				continue
			}
			files = append(files, GeneratedFile{
				Name:     fmt.Sprintf("generated-image-%d.png", i+1),
				Kind:     "image",
				MimeType: "image/png",
				Content:  imgData,
			})
		}
	}

	// 如果还是没有图片，返回任务信息作为备选
	if len(files) == 0 {
		logger.Warn("[Jimeng Provider] 未获取到图片，返回任务信息")
		payload := map[string]interface{}{
			"provider":     provider.config.Provider,
			"model":        modelName,
			"task_id":      taskID,
			"request_id":   queryResponse.RequestID,
			"time_elapsed": queryResponse.TimeElapsed,
			"message":      queryResponse.Message,
			"status":       queryResponse.Data.Status,
		}
		content, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, errors.Wrap(err, "encode jimeng task artifact")
		}
		files = append(files, GeneratedFile{
			Name:     "jimeng-task.json",
			Kind:     "json",
			MimeType: "application/json; charset=utf-8",
			Content:  content,
		})
	} else {
		logger.Info("[Jimeng Provider] 成功获取图片", zap.Int("count", len(files)))
	}

	return files, nil
}

func (provider *HTTPProvider) generateDashScopeImage(
	request GenerationRequest,
	baseURL string,
	apiKey string,
	modelName string,
) ([]GeneratedFile, error) {
	payload := map[string]interface{}{
		"model": modelName,
		"input": map[string]interface{}{
			"prompt": request.Prompt,
		},
		"parameters": map[string]interface{}{
			"size": "1024*1024",
			"n":    1,
		},
	}
	endpoint := joinBaseURL(baseURL, "services/aigc/text2image/image-synthesis")
	body, err := provider.doJSON("POST", endpoint, payload, modelName, "image-dashscope", func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("X-DashScope-Async", "disable")
	})
	if err != nil {
		return nil, err
	}

	file, err := provider.parseDashScopeImageFile(body)
	if err != nil {
		return nil, err
	}
	return []GeneratedFile{file}, nil
}

func isDashScopeProvider(providerName string, baseURL string) bool {
	return strings.Contains(providerName, "dashscope") ||
		strings.Contains(providerName, "qwen") ||
		strings.Contains(strings.ToLower(baseURL), "dashscope")
}

func isJimengProvider(providerName string, baseURL string) bool {
	lowerBaseURL := strings.ToLower(baseURL)
	return strings.Contains(providerName, "jimeng") ||
		strings.Contains(providerName, "doubao") ||
		strings.Contains(providerName, "volc") ||
		strings.Contains(lowerBaseURL, "volcengine") ||
		strings.Contains(lowerBaseURL, "visual.volcengineapi")
}

func runtimeConfigString(config model.UserModelConfig, keys ...string) string {
	// 对每个提供的键名，尝试多种变体形式进行查找
	for _, key := range keys {
		value := findRuntimeConfigValue(config.RuntimeConfig, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func findRuntimeConfigValue(runtimeConfig model.JSONMap, key string) string {
	// 先尝试精确匹配
	if value, ok := runtimeConfig[key]; ok && value != nil {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" {
				return text
			}
		}
	}

	// 尝试大小写不敏感匹配和空格/下划线替换
	targetKeyLower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, " ", ""), "_", ""))
	for k, value := range runtimeConfig {
		if value == nil {
			continue
		}
		kLower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, " ", ""), "_", ""))
		if kLower == targetKeyLower {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			default:
				text := strings.TrimSpace(fmt.Sprint(typed))
				if text != "" {
					return text
				}
			}
		}
	}

	return ""
}

func runtimeConfigInt(config model.UserModelConfig, key string) int {
	value := runtimeConfigString(config, key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func runtimeConfigBool(config model.UserModelConfig, key string) bool {
	switch strings.ToLower(runtimeConfigString(config, key)) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}

func normalizeJimengPrompt(prompt string) (string, error) {
	// 先去除 markdown 格式
	cleaned := strings.TrimSpace(prompt)

	if promptFitsImageLimits(cleaned) {
		return cleaned, nil
	}
	return "", errors.New("图片提示词仍超过即梦模型限制，已终止生成；未截断用户输入")
}

func (provider *HTTPProvider) parseOpenAIImageFile(body []byte, prompt string) (GeneratedFile, error) {
	var response struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return GeneratedFile{}, errors.Wrap(err, "decode image response")
	}
	if response.Error != nil {
		return GeneratedFile{}, errors.Errorf("image model api error: %s", response.Error.Message)
	}
	if len(response.Data) == 0 {
		return GeneratedFile{}, errors.New("image model returned empty data")
	}
	if strings.TrimSpace(response.Data[0].B64JSON) != "" {
		content, err := base64.StdEncoding.DecodeString(response.Data[0].B64JSON)
		if err != nil {
			return GeneratedFile{}, errors.Wrap(err, "decode generated image")
		}
		return GeneratedFile{
			Name:     "generated-image.png",
			Kind:     "image",
			MimeType: "image/png",
			Content:  content,
		}, nil
	}
	if strings.TrimSpace(response.Data[0].URL) != "" {
		return provider.downloadGeneratedImage(response.Data[0].URL)
	}
	return GeneratedFile{}, errors.New("image model returned no image payload")
}

func (provider *HTTPProvider) parseDashScopeImageFile(body []byte) (GeneratedFile, error) {
	var response struct {
		Output struct {
			Results []struct {
				URL string `json:"url"`
			} `json:"results"`
			TaskStatus string `json:"task_status"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return GeneratedFile{}, errors.Wrap(err, "decode dashscope image response")
	}
	if response.Code != "" {
		return GeneratedFile{}, errors.Errorf("dashscope image model api error: %s", response.Message)
	}
	if len(response.Output.Results) == 0 || strings.TrimSpace(response.Output.Results[0].URL) == "" {
		return GeneratedFile{}, errors.New("dashscope image model returned no image url")
	}
	return provider.downloadGeneratedImage(response.Output.Results[0].URL)
}

func (provider *HTTPProvider) downloadGeneratedImage(rawURL string) (GeneratedFile, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return GeneratedFile{}, errors.Wrap(err, "create image download request")
	}
	resp, err := provider.client.Do(req)
	if err != nil {
		return GeneratedFile{}, errors.Wrap(err, "download generated image")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GeneratedFile{}, errors.Errorf("download generated image http %d", resp.StatusCode)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return GeneratedFile{}, errors.Wrap(err, "read generated image")
	}
	mimeType := resp.Header.Get("Content-Type")
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	name := "generated-image" + imageExtension(mimeType)
	return GeneratedFile{
		Name:     name,
		Kind:     "image",
		MimeType: mimeType,
		Content:  content,
	}, nil
}

func imageExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".png"
	}
}

// maskString 对敏感信息进行掩码处理
func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
