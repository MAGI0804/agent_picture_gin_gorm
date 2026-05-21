package agent_svc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
)

// executeChatTurn 执行文本聊天轮次。
func (svc *AgentService) executeChatTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "text", request.TextModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}

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
	provider := strings.ToLower(configInfoFirstString(config.ConfigInfo, "provider", "vendor", "api_type", "type"))
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
	baseURL := configInfoFirstString(globalConfig.ConfigInfo, "base_url", "baseURL", "url", "endpoint")
	if baseURL == "" {
		baseURL = globalConfig.RequestURL
	}
	apiKey := configInfoFirstString(globalConfig.ConfigInfo, "api_key", "apiKey", "key", "token", "access_token", "auth_token", "anthropic_auth_token")
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
		config.AnthropicModel = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_model", "anthropicModel")
		if config.AnthropicModel == "" {
			config.AnthropicModel = globalConfig.ModelName
		}
	}

	config.AnthropicAuthToken = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_auth_token", "anthropicAuthToken")
	if config.AnthropicAuthToken == "" {
		config.AnthropicAuthToken = apiKey
	}
	config.AnthropicBaseURL = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_base_url", "anthropicBaseURL")
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
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
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

// createClarifyingTurn 生成补充问题轮次。
func (svc *AgentService) createClarifyingTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string, textModelConfigID uint) (map[string]interface{}, error) {
	questionResult, err := svc.generateFollowUpQuestions(userID, content, textModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
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

func (svc *AgentService) generateFollowUpQuestions(userID uint, content string, textModelConfigID uint) (ChatResult, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "text", textModelConfigID)
	if err != nil {
		return ChatResult{}, err
	}
	systemPrompt := strings.Join([]string{
		"You are the planner agent for an image generation workflow.",
		"Ask up to 3 targeted Chinese follow-up questions before generation.",
		"Questions must focus on goal, aspect ratio or size, style, required elements, and avoided elements.",
		"Return one question per line. Do not add numbering explanations or markdown.",
	}, " ")
	return NewProviderWithConfig(config).Chat(ChatRequest{
		System: systemPrompt,
		Messages: []ChatMessage{
			{Role: "user", Content: content},
		},
		ModelConfig:     config,
		Stream:          true,
		ReturnReasoning: true,
	})
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

// executeGeneration 执行固定多 Agent DAG，并保存生成产物。
func (svc *AgentService) executeGeneration(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveRuntimeModelConfig(userID, "image", request.ImageModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
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

	rawPrompt := svc.composePrompt(conversation.ID, content, memories, questions)
	promptResult, err := svc.refineGenerationPrompt(userID, normalizeTaskType(request.TaskType), rawPrompt, request.TextModelConfigID)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	prompt := promptResult.Content
	if strings.TrimSpace(prompt) == "" {
		prompt = rawPrompt
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
	config, err := svc.resolveRuntimeModelConfig(userID, "text", textModelConfigID)
	if err != nil {
		return ChatResult{}, err
	}
	systemPrompt := strings.Join([]string{
		"You are the prompt agent for an image AI Agent workflow.",
		"Convert the user's Chinese requirements, follow-up answers, and context into a concise final generation prompt.",
		"Return only the final prompt. Include subject, composition, style, color, aspect ratio, required elements, and avoid-list when present.",
		"Do not say you cannot generate images. Do not include markdown fences.",
	}, " ")
	userPrompt := fmt.Sprintf("task_type=%s\n\n%s", taskType, rawPrompt)
	return NewProviderWithConfig(config).Chat(ChatRequest{
		System: systemPrompt,
		Messages: []ChatMessage{
			{Role: "user", Content: userPrompt},
		},
		ModelConfig:     config,
		Stream:          true,
		ReturnReasoning: true,
	})
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
