package agent_svc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/internal/service/model_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"
)

const imagePromptTargetLength = 750
const imagePromptTargetBytes = 780
const PromptTooLongMessage = "提示词太长，请重新输入或使用智能优化功能"

// executeChatTurn 执行文本聊天轮次。
func (svc *AgentService) executeChatTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "text", request.TextModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	run.TaskType = "text_chat"
	run.TextModelName = runtimeTextModelName(config)
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{
		"task_type":       run.TaskType,
		"text_model_name": run.TextModelName,
	})

	messages, err := svc.buildChatMessages(userID, conversation.ID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}

	systemPrompt := fmt.Sprintf(
		"You are the chat assistant in an image AI Agent workspace. Answer the user directly. The active backend model configuration is provider=%s, chat_model=%s, anthropic_model=%s. If the user asks which model you use, answer from this configuration. Do not claim artifacts were generated unless the backend artifact flow has completed.",
		config.Provider,
		config.ChatModel,
		config.AnthropicModel,
	)
	stream := true
	returnReasoning := true
	_ = svc.createStepWithThinking(run.ID, "model_config_agent", config.Provider+"/"+config.ChatModel, "loaded model config", "读取当前用户绑定的文本对话模型配置。", "")
	_ = svc.createStepWithThinking(run.ID, "external_model_request_agent", modelCallTarget(config), modelRequestPayloadSummary(stream, returnReasoning, config.Temperature), "准备以流式方式调用模型，并要求返回 reasoning 内容。", "")
	chatResult, err := NewProviderWithConfig(config).Chat(ChatRequest{
		System:          systemPrompt,
		Messages:        messages,
		ModelConfig:     config,
		Stream:          stream,
		ReturnReasoning: returnReasoning,
	})
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	reply := strings.TrimSpace(chatResult.Content)
	_ = svc.createStepWithThinking(run.ID, "model_chat_agent", userMessage.Content, reply, "结合最近会话上下文生成本轮文本回复。", chatResult.ReasoningContent)

	assistantMessage := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "assistant",
		InputType:      "normal",
		Content:        reply,
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&assistantMessage); err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}

	run.Status = "completed"
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": run.Status})
	steps, _ := svc.dao.ListAgentSteps(userID, run.ID)

	return map[string]interface{}{
		"user_message":        userMessage,
		"assistant_message":   assistantMessage,
		"follow_up_questions": []model.FollowUpQuestion{},
		"agent_run":           run,
		"agent_steps":         steps,
		"artifacts":           []model.Artifact{},
		"model_output": map[string]interface{}{
			"content":          reply,
			"thinking_content": chatResult.ReasoningContent,
			"finish_reason":    "stop",
			"usage":            model.TokenUsage{},
		},
		"conversation": conversation,
	}, nil
}

func (svc *AgentService) buildChatMessages(userID uint, conversationID uint) ([]ChatMessage, error) {
	storedMessages, err := svc.dao.ListMessages(userID, conversationID)
	if err != nil {
		return nil, err
	}
	start := 0
	if len(storedMessages) > 20 {
		start = len(storedMessages) - 20
	}
	messages := make([]ChatMessage, 0, len(storedMessages)-start)
	for _, message := range storedMessages[start:] {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}
	return messages, nil
}

func (svc *AgentService) latestChatMessages(userID uint, conversationID uint, limit int) []ChatMessage {
	messages, err := svc.buildChatMessages(userID, conversationID)
	if err != nil {
		return []ChatMessage{}
	}
	if len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func (svc *AgentService) resolveRuntimeModelConfig(userID uint, modelKind string, selectedModelConfigID uint) (model.UserModelConfig, error) {
	config := model.UserModelConfig{}
	selected, ok := svc.permittedGlobalModelConfig(userID, modelKind, selectedModelConfigID)
	if !ok {
		return config, errors.Errorf("未选择真实%s模型，请先在输入框或设置页选择模型", modelKindLabel(modelKind))
	}
	config = mergeUserConfigWithGlobalModel(model.UserModelConfig{
		UserID:      userID,
		Temperature: "0.7",
	}, selected, modelKind)

	if strings.TrimSpace(config.Provider) == "" || strings.EqualFold(config.Provider, "mock") {
		return config, errors.Errorf("所选%s模型不是有效真实模型，请检查全局模型配置", modelKindLabel(modelKind))
	}
	if strings.TrimSpace(config.Temperature) == "" {
		config.Temperature = "0.7"
	}
	if strings.TrimSpace(config.AnthropicAuthToken) == "" {
		config.AnthropicAuthToken = strings.TrimSpace(config.APIKey)
	}
	if strings.TrimSpace(config.APIKey) == "" {
		config.APIKey = strings.TrimSpace(config.AnthropicAuthToken)
	}
	if strings.TrimSpace(config.AnthropicBaseURL) == "" {
		config.AnthropicBaseURL = strings.TrimSpace(config.BaseURL)
	}
	if strings.TrimSpace(config.AnthropicModel) == "" {
		config.AnthropicModel = strings.TrimSpace(config.ChatModel)
	}
	return config, nil
}

func (svc *AgentService) firstAvailableUserGlobalModelConfig(userID uint, modelKind string) (model.ModelConfig, bool) {
	var configs []model.ModelConfig
	var err error
	if modelKind == "image" {
		configs, err = svc.ListUserImageModels(userID)
	} else {
		configs, err = svc.ListUserTextModels(userID)
	}
	if err != nil || len(configs) == 0 {
		return model.ModelConfig{}, false
	}

	// 对于文本模型，优先查找 deepseek-v4-pro
	if modelKind != "image" {
		for _, cfg := range configs {
			modelName := strings.ToLower(strings.TrimSpace(cfg.ModelName))
			if strings.Contains(modelName, "deepseek-v4-pro") || strings.Contains(modelName, "deepseek_v4_pro") {
				return cfg, true
			}
		}
	}

	// 如果没找到，返回第一个可用的
	return configs[0], true
}

func modelKindLabel(modelKind string) string {
	if modelKind == "image" {
		return "图片"
	}
	return "文本"
}

func (svc *AgentService) permittedGlobalModelConfig(
	userID uint,
	modelKind string,
	selectedID uint,
) (model.ModelConfig, bool) {
	if selectedID == 0 {
		return svc.firstAvailableUserGlobalModelConfig(userID, modelKind)
	}
	config, err := svc.dao.FindPermittedModelConfig(userID, selectedID)
	if err != nil {
		return model.ModelConfig{}, false
	}
	if isMockModelConfig(config) {
		return model.ModelConfig{}, false
	}
	if modelKind == "image" && !config.IsImageModel {
		return model.ModelConfig{}, false
	}
	if modelKind != "image" && !config.IsTextModel {
		return model.ModelConfig{}, false
	}
	return config, true
}

func modelConfigProvider(config model.ModelConfig) string {
	provider := strings.ToLower(configInfoFirstString(config.ConfigInfo, "provider", "vendor", "api_type", "type", "provider name", "api type"))
	if provider != "" {
		return provider
	}

	source := strings.ToLower(strings.TrimSpace(config.RequestURL) + " " + strings.TrimSpace(config.ModelName))
	switch {
	case strings.Contains(source, "deepseek"):
		return "deepseek"
	case strings.Contains(source, "anthropic") || strings.Contains(source, "claude"):
		return "anthropic"
	case strings.Contains(source, "dashscope") || strings.Contains(source, "qwen") || strings.Contains(source, "aliyun"):
		return "dashscope"
	case strings.Contains(source, "jimeng") || strings.Contains(source, "doubao") || strings.Contains(source, "volc") || strings.Contains(source, "seedream"):
		return "jimeng"
	case strings.Contains(source, "openai") || strings.Contains(source, "gpt"):
		return "openai"
	case strings.TrimSpace(config.RequestURL) != "":
		return "openai-compatible"
	default:
		return ""
	}
}

func mergeUserConfigWithGlobalModel(
	userConfig model.UserModelConfig,
	globalConfig model.ModelConfig,
	modelKind string,
) model.UserModelConfig {
	config := userConfig
	config.RuntimeConfig = globalConfig.ConfigInfo
	provider := modelConfigProvider(globalConfig)
	baseURL := configInfoFirstString(globalConfig.ConfigInfo, "base_url", "baseURL", "url", "endpoint", "base url")
	if baseURL == "" {
		baseURL = globalConfig.RequestURL
	}
	apiKey := configInfoFirstString(globalConfig.ConfigInfo,
		"api_key", "apiKey", "key", "token", "access_token", "auth_token", "anthropic_auth_token",
		"api key", "access key", "secret access key", "secret key")
	temperature := configInfoFirstString(globalConfig.ConfigInfo, "temperature", "temp")
	if temperature == "" {
		temperature = config.Temperature
	}
	if temperature == "" {
		temperature = "0.7"
	}

	config.Provider = provider
	config.BaseURL = baseURL
	config.APIKey = apiKey
	config.Temperature = temperature
	if modelKind == "image" {
		config.ImageModel = globalConfig.ModelName
	} else {
		config.ChatModel = globalConfig.ModelName
		config.AnthropicModel = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_model", "anthropicModel", "anthropic model")
		if config.AnthropicModel == "" {
			config.AnthropicModel = globalConfig.ModelName
		}
	}

	config.AnthropicAuthToken = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_auth_token", "anthropicAuthToken", "anthropic auth token")
	if config.AnthropicAuthToken == "" {
		config.AnthropicAuthToken = apiKey
	}
	config.AnthropicBaseURL = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_base_url", "anthropicBaseURL", "anthropic base url")
	if config.AnthropicBaseURL == "" {
		config.AnthropicBaseURL = baseURL
	}
	config.AnthropicDefaultOpusModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_opus_model")
	config.AnthropicDefaultSonnetModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_sonnet_model")
	config.AnthropicDefaultHaikuModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_haiku_model")
	config.ClaudeCodeSubagentModel = configInfoString(globalConfig.ConfigInfo, "claude_code_subagent_model")
	config.ClaudeCodeMaxOutputTokens = configInfoString(globalConfig.ConfigInfo, "claude_code_max_output_tokens")
	if config.ClaudeCodeMaxOutputTokens == "" {
		config.ClaudeCodeMaxOutputTokens = "4096"
	}
	return config
}

func configInfoString(values model.JSONMap, key string) string {
	if len(values) == 0 {
		return ""
	}

	// 先尝试精确匹配
	if value, ok := values[key]; ok && value != nil {
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		default:
			return strings.TrimSpace(fmt.Sprint(typed))
		}
	}

	// 尝试大小写不敏感匹配（包括空格处理）
	targetKeyLower := strings.ToLower(strings.ReplaceAll(key, " ", ""))
	for k, value := range values {
		if value == nil {
			continue
		}
		kLower := strings.ToLower(strings.ReplaceAll(k, " ", ""))
		if kLower == targetKeyLower {
			switch typed := value.(type) {
			case string:
				return strings.TrimSpace(typed)
			default:
				return strings.TrimSpace(fmt.Sprint(typed))
			}
		}
	}

	return ""
}

func configInfoFirstString(values model.JSONMap, keys ...string) string {
	for _, key := range keys {
		if value := configInfoString(values, key); value != "" {
			return value
		}
	}
	return ""
}

func modelCallTarget(config model.UserModelConfig) string {
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	if strings.Contains(provider, "anthropic") || strings.Contains(provider, "claude") ||
		(provider == "deepseek" && strings.Contains(strings.ToLower(config.AnthropicBaseURL), "anthropic")) {
		return strings.TrimRight(config.AnthropicBaseURL, "/") + "/v1/messages"
	}
	return strings.TrimRight(config.BaseURL, "/") + "/chat/completions"
}

func runtimeTextModelName(config model.UserModelConfig) string {
	if strings.TrimSpace(config.ChatModel) != "" {
		return strings.TrimSpace(config.ChatModel)
	}
	if strings.TrimSpace(config.AnthropicModel) != "" {
		return strings.TrimSpace(config.AnthropicModel)
	}
	return strings.TrimSpace(config.Provider)
}

func runtimeImageModelName(config model.UserModelConfig) string {
	if strings.TrimSpace(config.ImageModel) != "" {
		return strings.TrimSpace(config.ImageModel)
	}
	return strings.TrimSpace(config.Provider)
}

// createClarifyingTurn 生成补充问题轮次。
func (svc *AgentService) createClarifyingTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string, textModelConfigID uint) (map[string]interface{}, error) {
	questionResult, textModelName, err := svc.generateFollowUpQuestions(userID, content, textModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	run.TaskType = "image_generation"
	run.TextModelName = textModelName
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{
		"task_type":       run.TaskType,
		"text_model_name": run.TextModelName,
	})
	questionTexts := parseQuestionLines(questionResult.Content)
	_ = svc.createStepWithThinking(
		run.ID,
		"planner_agent",
		content,
		strings.Join(questionTexts, "\n"),
		"调用当前文本模型判断图片任务还缺少哪些关键信息，并生成有针对性的追问。",
		questionResult.ReasoningContent,
	)
	assistantMessage := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "assistant",
		InputType:      "follow_up_questions",
		Content:        "我需要先确认几个细节，然后再进入多 Agent 生成流程。\n" + strings.Join(questionTexts, "\n"),
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&assistantMessage); err != nil {
		return nil, err
	}

	questions := make([]model.FollowUpQuestion, 0, len(questionTexts))
	for _, questionText := range questionTexts {
		questions = append(questions, model.FollowUpQuestion{
			ConversationID: conversation.ID,
			MessageID:      assistantMessage.ID,
			UserID:         userID,
			Question:       questionText,
			Status:         "pending",
		})
	}
	if err := svc.dao.CreateFollowUpQuestions(questions); err != nil {
		return nil, err
	}
	run.Status = "waiting_questions"
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": run.Status})
	steps, _ := svc.dao.ListAgentSteps(userID, run.ID)

	return map[string]interface{}{
		"user_message":        userMessage,
		"assistant_message":   assistantMessage,
		"follow_up_questions": questions,
		"agent_run":           run,
		"agent_steps":         steps,
		"model_output": map[string]interface{}{
			"content":          assistantMessage.Content,
			"thinking_content": questionResult.ReasoningContent,
			"finish_reason":    "stop",
			"usage":            model.TokenUsage{},
		},
		"conversation": conversation,
	}, nil
}

func (svc *AgentService) generateFollowUpQuestions(userID uint, content string, textModelConfigID uint) (ChatResult, string, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "text", textModelConfigID)
	if err != nil {
		return ChatResult{}, "", err
	}
	systemPrompt := strings.Join([]string{
		"You are the planner agent for an image generation workflow.",
		"Ask up to 3 targeted Chinese follow-up questions before generation.",
		"Questions must focus on goal, aspect ratio or size, style, required elements, and avoided elements.",
		"Return one question per line. Do not add numbering explanations or markdown.",
	}, " ")
	result, err := NewProviderWithConfig(config).Chat(ChatRequest{
		System: systemPrompt,
		Messages: []ChatMessage{
			{Role: "user", Content: content},
		},
		ModelConfig:     config,
		Stream:          true,
		ReturnReasoning: true,
	})
	return result, runtimeTextModelName(config), err
}

func parseQuestionLines(content string) []string {
	lines := strings.Split(content, "\n")
	questions := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*0123456789.、)） ")
		if line == "" {
			continue
		}
		questions = append(questions, line)
		if len(questions) == 3 {
			break
		}
	}
	if len(questions) > 0 {
		return questions
	}
	return []string{
		"这张图的核心用途是什么，例如头像、海报、产品图还是页面配图？",
		"希望使用什么尺寸或比例，例如 1:1、16:9、9:16？",
		"必须出现或必须避免哪些元素、文字、风格？",
	}
}

func (svc *AgentService) prepareGenerationPromptInput(
	userID uint,
	userMessage model.Message,
	run model.AgentRun,
	content string,
	request agent_request.SendMessageRequest,
) (string, string, string, error) {
	optimizedPrompt := strings.TrimSpace(request.OptimizedPrompt)
	if request.IsOptimized && optimizedPrompt != "" {
		if !promptFitsImageLimits(optimizedPrompt) {
			return "", "", "", errors.New("智能优化后的提示词仍超过图片模型限制，请重新优化或缩短输入")
		}
		return optimizedPrompt, optimizedPrompt, "用户确认使用智能优化后的提示词；后端已校验图片模型长度限制。", nil
	}

	if promptFitsImageLimits(content) {
		return content, "", "", nil
	}

	logger.Warn("[Prompt Agent] 图片提示词过长，自动触发智能优化",
		zap.Uint("user_id", userID),
		zap.Uint("message_id", userMessage.ID),
		zap.Uint("run_id", run.ID),
		zap.Int("original_length", len([]rune(content))),
	)

	optimized, err := svc.optimizePromptWithDeepseek(userID, content, imagePromptTargetLength, "shorten")
	if err != nil {
		logger.Warn("[Prompt Agent] 自动优化失败，终止图片生成",
			zap.Uint("user_id", userID),
			zap.Uint("message_id", userMessage.ID),
			zap.Uint("run_id", run.ID),
			zap.Error(err),
		)
		_ = svc.createStepWithThinking(
			run.ID,
			"auto_prompt_optimize_agent",
			content,
			"",
			"图片提示词超过限制，自动调用 deepseek-v4-pro 优化失败，已终止生成；未截断用户输入。",
			err.Error(),
		)
		return "", "", "", errors.Wrap(err, "智能优化失败，请重试或缩短提示词")
	}

	optimized = strings.TrimSpace(optimized)
	if optimized == "" {
		return "", "", "", errors.New("智能优化失败：优化结果为空")
	}
	if !promptFitsImageLimits(optimized) {
		secondPass, secondErr := svc.optimizePromptWithDeepseek(
			userID,
			optimized,
			imagePromptTargetLength,
			"shorten",
		)
		if secondErr == nil && strings.TrimSpace(secondPass) != "" {
			optimized = strings.TrimSpace(secondPass)
		}
	}
	if !promptFitsImageLimits(optimized) {
		_ = svc.createStepWithThinking(
			run.ID,
			"auto_prompt_optimize_agent",
			content,
			optimized,
			fmt.Sprintf("图片提示词超过限制，自动优化后仍为 %d 字符、%d 字节，已终止生成；未截断用户输入。", len([]rune(optimized)), len(optimized)),
			"",
		)
		return "", "", "", errors.New("智能优化后的提示词仍超过图片模型限制，请缩短内容后重试")
	}

	reason := fmt.Sprintf(
		"图片提示词超过限制，已自动调用 deepseek-v4-pro 优化；最终长度 %d 字符、%d 字节。",
		len([]rune(optimized)),
		len(optimized),
	)
	_ = svc.createStepWithThinking(
		run.ID,
		"auto_prompt_optimize_agent",
		content,
		optimized,
		reason,
		"",
	)
	return optimized, optimized, reason, nil
}

func (svc *AgentService) ensureImagePromptLength(
	userID uint,
	userMessage model.Message,
	run model.AgentRun,
	prompt string,
) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New(PromptTooLongMessage)
	}
	if promptFitsImageLimits(prompt) {
		return prompt, nil
	}

	logger.Warn("[Prompt Agent] 最终图片提示词过长，终止图片生成",
		zap.Uint("user_id", userID),
		zap.Uint("message_id", userMessage.ID),
		zap.Uint("run_id", run.ID),
		zap.Int("prompt_length", len([]rune(prompt))),
		zap.Int("prompt_bytes", len(prompt)),
	)
	_ = svc.createStepWithThinking(
		run.ID,
		"image_prompt_length_check_agent",
		"",
		"",
		fmt.Sprintf("最终图片提示词超过限制：%d 字符、%d 字节，已终止生成；未截断用户输入。", len([]rune(prompt)), len(prompt)),
		"",
	)
	return "", errors.New("最终图片提示词仍超过图片模型限制，请重新优化或缩短输入")
}

func promptFitsImageLimits(prompt string) bool {
	prompt = strings.TrimSpace(prompt)
	return len([]rune(prompt)) <= imagePromptTargetLength && len(prompt) <= imagePromptTargetBytes
}

func normalizePromptTargetLength(targetLength int) int {
	if targetLength <= 0 {
		return imagePromptTargetLength
	}
	if targetLength < 100 {
		return 100
	}
	if targetLength > imagePromptTargetLength {
		return imagePromptTargetLength
	}
	return targetLength
}

// executeGeneration 执行固定多 Agent DAG，并保存生成产物。
func (svc *AgentService) executeGeneration(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "image", request.ImageModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	run.TaskType = normalizeTaskType(request.TaskType)
	run.ImageModelName = runtimeImageModelName(config)
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{
		"task_type":        run.TaskType,
		"image_model_name": run.ImageModelName,
	})
	generationInput, optimizedForGeneration, optimizeReason, err := svc.prepareGenerationPromptInput(userID, userMessage, run, content, request)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": PromptTooLongMessage})
		return nil, err
	}
	if optimizedForGeneration != "" {
		content = generationInput
		userMessage.IsOptimized = true
		userMessage.OptimizedPrompt = optimizedForGeneration
		run.IsOptimized = true
		run.OptimizedPrompt = optimizedForGeneration
		_ = svc.dao.UpdateMessageOptimization(userMessage.ID, true, optimizedForGeneration)
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{
			"is_optimized":     true,
			"optimized_prompt": optimizedForGeneration,
		})
	}
	memories, _ := svc.dao.ListContextMemories(userID, conversation.ID, 5)
	questions, _ := svc.dao.ListConversationQuestions(userID, conversation.ID, 20)
	contextPackage := map[string]interface{}{
		"short_messages": svc.latestChatMessages(userID, conversation.ID, 20),
		"task": map[string]interface{}{
			"task_type":              normalizeTaskType(request.TaskType),
			"intent":                 run.Intent,
			"answered_question_ids":  request.AnsweredQuestionIDs,
			"current_agent_run_id":   run.ID,
			"trigger_message_id":     userMessage.ID,
			"follow_up_question_set": questions,
		},
		"long_term_memories": memories,
	}
	contextBytes, _ := json.Marshal(contextPackage)

	rawPrompt := svc.composePrompt(conversation.ID, generationInput, memories, questions)

	// 准备一个简单的默认提示词作为备用
	simpleFallbackPrompt := generationInput
	if strings.TrimSpace(simpleFallbackPrompt) == "" {
		simpleFallbackPrompt = "Beautiful landscape scenery, natural environment"
	}

	// 尝试使用 prompt agent 生成提示词
	var prompt string
	var promptResult ChatResult
	if optimizedForGeneration != "" {
		prompt = optimizedForGeneration
		promptResult = ChatResult{
			Content:          optimizedForGeneration,
			ReasoningContent: optimizeReason,
		}
	} else {
		promptResult, err = svc.refineGenerationPrompt(userID, normalizeTaskType(request.TaskType), rawPrompt, request.TextModelConfigID)
		if err == nil && strings.TrimSpace(promptResult.Content) != "" {
			prompt = promptResult.Content

			// 检查生成的提示词是否合理（不是对话回复）
			lowerPrompt := strings.ToLower(prompt)
			if strings.Contains(lowerPrompt, "无法") || strings.Contains(lowerPrompt, "不能") ||
				strings.Contains(lowerPrompt, "抱歉") || strings.Contains(lowerPrompt, "but") ||
				strings.Contains(lowerPrompt, "i cannot") || strings.Contains(lowerPrompt, "i can't") {
				// 如果提示词有问题，使用备用
				prompt = simpleFallbackPrompt
			}
		} else {
			if err != nil && err.Error() == PromptTooLongMessage {
				_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": PromptTooLongMessage})
				return nil, err
			}
			// 如果 prompt agent 失败，使用备用
			prompt = simpleFallbackPrompt
		}
	}
	promptBeforeLengthCheck := prompt
	prompt, err = svc.ensureImagePromptLength(userID, userMessage, run, prompt)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": PromptTooLongMessage})
		return nil, err
	}
	if prompt != strings.TrimSpace(promptBeforeLengthCheck) {
		userMessage.IsOptimized = true
		userMessage.OptimizedPrompt = prompt
		run.IsOptimized = true
		run.OptimizedPrompt = prompt
	}
	_ = svc.createStepsWithThinking(run.ID, []stepRecord{
		{
			name:         "context_agent",
			input:        string(contextBytes),
			output:       "已读取最近上下文和长期记忆。",
			thinkContent: "检索当前会话记忆、用户偏好和相关历史任务。",
		},
		{
			name:             "prompt_agent",
			input:            rawPrompt,
			output:           prompt,
			thinkContent:     "调用当前文本模型，把用户需求、追问回答和上下文整理成图片模型可执行提示词。",
			reasoningContent: promptResult.ReasoningContent,
		},
	})

	files, err := NewProviderWithConfig(config).Generate(GenerationRequest{
		Prompt:          prompt,
		Intent:          run.Intent,
		TaskType:        normalizeTaskType(request.TaskType),
		Stream:          true,
		ReturnReasoning: true,
		Temperature:     config.Temperature,
		ModelConfig:     config,
	})
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	artifacts := make([]model.Artifact, 0, len(files))
	for _, file := range files {
		objectKey := fmt.Sprintf("user-%d/conversation-%d/run-%d/%s", userID, conversation.ID, run.ID, file.Name)
		stored, err := svc.store.Save(objectKey, file.Content)
		if err != nil {
			_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
			return nil, err
		}
		artifact := model.Artifact{
			ConversationID: conversation.ID,
			UserID:         userID,
			AgentRunID:     run.ID,
			Name:           file.Name,
			Kind:           file.Kind,
			MimeType:       file.MimeType,
			ObjectKey:      stored.ObjectKey,
			PreviewURL:     stored.PreviewURL,
			SizeBytes:      stored.SizeBytes,
			Hash:           stored.Hash,
		}
		artifacts = append(artifacts, artifact)
	}
	artifacts, err = svc.dao.CreateArtifacts(artifacts)
	if err != nil {
		return nil, err
	}
	_ = svc.createStepsWithThinking(run.ID, []stepRecord{
		{
			name:             "image_agent",
			input:            prompt,
			output:           "已提交图片模型并生成产物。",
			thinkContent:     "根据图片生成模式选择图片模型并生成可预览产物。",
			reasoningContent: "图片模型由所选全局图片模型配置决定；即梦模型会返回 task_id JSON 产物。",
		},
		{
			name:             "html_agent",
			input:            prompt,
			output:           "已整理可预览产物。",
			thinkContent:     "将生成结果组织成右侧可预览或可下载的产物。",
			reasoningContent: "如果图片模型返回异步任务，则以 JSON 任务产物呈现。",
		},
		{
			name:         "review_agent",
			input:        prompt,
			output:       "review passed",
			thinkContent: "检查产物是否可预览、可下载，并确认元数据完整。",
		},
		{
			name:         "artifact_agent",
			input:        prompt,
			output:       "产物已保存并完成元数据入库。",
			thinkContent: "保存对象存储文件并把预览地址、下载元数据写入数据库。",
		},
	})

	assistantMessage := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "assistant",
		InputType:      "agent_result",
		Content:        "已完成多 Agent 协作生成，右侧可以预览图片和 HTML 产物。",
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&assistantMessage); err != nil {
		return nil, err
	}

	memoriesToCreate := []model.ContextMemory{
		{
			ConversationID: conversation.ID,
			UserID:         userID,
			Kind:           "summary",
			Content:        "用户补充需求：" + content,
			Score:          10,
		},
		{
			ConversationID: conversation.ID,
			UserID:         userID,
			Kind:           "artifact_requirement",
			Content:        "生成提示词：" + prompt,
			Score:          8,
		},
	}
	if preference := extractPreference(content); preference != "" {
		memoriesToCreate = append(memoriesToCreate, model.ContextMemory{
			ConversationID: conversation.ID,
			UserID:         userID,
			Kind:           "preference",
			Content:        preference,
			Score:          12,
		})
	}
	_ = svc.dao.CreateContextMemories(memoriesToCreate)
	run.Status = "completed"
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": run.Status})
	steps, _ := svc.dao.ListAgentSteps(userID, run.ID)

	return map[string]interface{}{
		"user_message":        userMessage,
		"assistant_message":   assistantMessage,
		"follow_up_questions": []model.FollowUpQuestion{},
		"artifacts":           artifacts,
		"agent_run":           run,
		"agent_steps":         steps,
		"model_output": map[string]interface{}{
			"artifacts":        artifacts,
			"thinking_content": promptResult.ReasoningContent,
			"finish_reason":    "stop",
			"image_width":      0,
			"image_height":     0,
			"image_url":        "",
			"image_base64":     "",
		},
		"conversation": conversation,
	}, nil
}

func (svc *AgentService) refineGenerationPrompt(
	userID uint,
	taskType string,
	rawPrompt string,
	textModelConfigID uint,
) (ChatResult, error) {
	// 检查原始提示词长度
	rawLength := len([]rune(rawPrompt))
	if rawLength > imagePromptTargetLength {
		logger.Warn("[Prompt Agent] 提示词过长，终止生成流程",
			zap.Int("raw_length", rawLength),
			zap.Int("target_length", imagePromptTargetLength),
		)
		return ChatResult{}, errors.New(PromptTooLongMessage)
	}

	// 直接使用原始提示词，不做自动优化
	logger.Info("[Prompt Agent] 使用原始提示词",
		zap.String("raw_prompt", rawPrompt),
		zap.Int("raw_length", rawLength),
	)

	return ChatResult{
		Content:          rawPrompt,
		ReasoningContent: "直接使用用户输入的提示词",
	}, nil
}

// 保留 optimizePromptWithDeepseek 函数，供单独调用智能优化使用
func (svc *AgentService) optimizePromptWithDeepseek(userID uint, prompt string, maxLength int, optimizationType string) (string, error) {
	// 首先查找用户有权使用的 deepseek-v4-pro 模型配置
	deepseekConfig, err := svc.findDeepseekV4ProConfig(userID)
	if err != nil {
		return "", errors.Wrap(err, "failed to find deepseek-v4-pro config")
	}

	// 提取 URL 和 API Key
	apiURL, apiKey, err := extractDeepseekCredentials(deepseekConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to extract deepseek credentials")
	}

	logger.Info("[Prompt Agent] 找到 deepseek-v4-pro 配置，准备调用专门的提示词优化方法",
		zap.String("api_url", apiURL),
		zap.String("model_name", deepseekConfig.ModelName),
		zap.Int("user_id", int(userID)),
	)

	// 调用专门的 deepseek 提示词缩短方法，使用配置中的实际模型名称
	return model_request.OptimizePromptWithDeepseek(apiURL, apiKey, deepseekConfig.ModelName, prompt, maxLength, optimizationType)
}

// findDeepseekV4ProConfig 查找 deepseek-v4-pro 的模型配置
func (svc *AgentService) findDeepseekV4ProConfig(userID uint) (model.ModelConfig, error) {
	// 查找用户有权使用的模型配置
	configs, err := svc.dao.ListPermittedModelConfigs(userID, true, false)
	if err != nil {
		return model.ModelConfig{}, err
	}

	// 在用户可使用的模型中寻找 deepseek-v4-pro
	for _, cfg := range configs {
		modelName := strings.ToLower(strings.TrimSpace(cfg.ModelName))
		if strings.Contains(modelName, "deepseek-v4-pro") || strings.Contains(modelName, "deepseek_v4_pro") {
			logger.Info("[Prompt Agent] 找到用户可用的 deepseek-v4-pro 模型配置",
				zap.String("model_name", cfg.ModelName),
				zap.Uint("config_id", cfg.ID),
			)
			return cfg, nil
		}
	}

	// 如果用户没有 deepseek-v4-pro 配置，查找全局配置
	var globalConfig model.ModelConfig
	err = database.DB.Where("model_name LIKE ?", "%deepseek-v4-pro%").Or("model_name LIKE ?", "%deepseek_v4_pro%").First(&globalConfig).Error
	if err != nil {
		return model.ModelConfig{}, errors.Wrap(err, "deepseek-v4-pro config not found globally")
	}

	logger.Info("[Prompt Agent] 找到全局 deepseek-v4-pro 模型配置",
		zap.String("model_name", globalConfig.ModelName),
		zap.Uint("config_id", globalConfig.ID),
	)
	return globalConfig, nil
}

// extractDeepseekCredentials 从 ModelConfig 中提取 URL 和 API Key
func extractDeepseekCredentials(config model.ModelConfig) (string, string, error) {
	// 从 ConfigInfo 中提取必要的参数
	apiURL := config.RequestURL
	if apiURL == "" {
		apiURL = configInfoFirstString(config.ConfigInfo, "base_url", "request_url", "url", "api_url", "endpoint")
	}
	if apiURL == "" {
		// 默认 deepseek API URL
		apiURL = "https://api.deepseek.com"
	}

	apiKey := configInfoFirstString(config.ConfigInfo, "api_key", "apikey", "api_key_secret", "secret", "token", "auth_token", "authorization")
	if apiKey == "" {
		return "", "", errors.New("api key not found in config")
	}

	return apiURL, apiKey, nil
}

// composePrompt 将当前输入和历史记忆组合为 Provider 提示词。
func (svc *AgentService) composePrompt(
	conversationID uint,
	content string,
	memories []model.ContextMemory,
	questions []model.FollowUpQuestion,
) string {
	parts := []string{fmt.Sprintf("conversation:%d", conversationID), content}
	for _, question := range questions {
		if strings.TrimSpace(question.Answer) == "" {
			continue
		}
		parts = append(parts, "补充问题："+question.Question)
		parts = append(parts, "用户回答："+question.Answer)
	}
	for _, memory := range memories {
		parts = append(parts, memory.Content)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractPreference(content string) string {
	keywords := []string{"喜欢", "偏好", "风格", "不要", "必须", "希望"}
	for _, keyword := range keywords {
		if strings.Contains(content, keyword) {
			return "用户偏好：" + strings.TrimSpace(content)
		}
	}
	return ""
}
