package agent_svc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"gin-biz-web-api/internal/dao/agent_dao"
	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// AgentService 封装 AI Agent 会话、消息、编排和产物生成业务。
type AgentService struct {
	dao   *agent_dao.AgentDAO // 数据访问对象。
	store ObjectStore         // 产物对象存储，首版使用本地文件实现。
}

// NewAgentService 创建 AI Agent 业务服务。
func NewAgentService() *AgentService {
	return &AgentService{
		dao:   agent_dao.NewAgentDAO(),
		store: NewObjectStore(),
	}
}

// ListConversations 查询当前用户的会话列表。
func (svc *AgentService) ListConversations(userID uint) ([]model.Conversation, error) {
	return svc.dao.ListConversations(userID)
}

// CreateConversation 创建一个新的 AI Agent 会话。
func (svc *AgentService) CreateConversation(userID uint, title string) (model.Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "新的图片 Agent 会话"
	}
	conversation := model.Conversation{UserID: userID, Title: title, Status: "active"}
	err := svc.dao.CreateConversation(&conversation)
	return conversation, err
}

// ListMessages 查询指定会话下的消息列表，并校验用户归属。
func (svc *AgentService) ListMessages(userID uint, conversationID uint) ([]model.Message, error) {
	if _, err := svc.dao.FindConversation(userID, conversationID); err != nil {
		return nil, err
	}
	return svc.dao.ListMessages(userID, conversationID)
}

// SendMessage 根据输入类型处理普通对话或补充问题回答。
func (svc *AgentService) SendMessage(userID uint, conversationID uint, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	conversation, err := svc.dao.FindConversation(userID, conversationID)
	if err != nil {
		return nil, err
	}
	messageCount, err := svc.dao.CountMessages(userID, conversationID)
	if err != nil {
		return nil, err
	}

	inputType := strings.TrimSpace(request.InputType)
	if inputType == "" {
		inputType = "normal"
	}
	if inputType != "normal" && inputType != "answer_to_questions" {
		return nil, errors.New("input_type must be normal or answer_to_questions")
	}
	taskType := normalizeTaskType(request.TaskType)
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return nil, errors.New("content cannot be empty")
	}

	userMessage := model.Message{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           "user",
		InputType:      inputType,
		Content:        content,
	}
	if err := svc.dao.CreateMessage(&userMessage); err != nil {
		return nil, err
	}
	if messageCount == 0 {
		conversation.Title = makeConversationTitle(content)
		_ = svc.dao.UpdateConversationTitle(userID, conversationID, conversation.Title)
	}

	run := model.AgentRun{
		ConversationID:   conversationID,
		UserID:           userID,
		TriggerMessageID: userMessage.ID,
		Status:           "running",
		Intent:           svc.detectIntent(content, taskType),
	}
	if err := svc.dao.CreateAgentRun(&run); err != nil {
		return nil, err
	}
	_ = svc.dao.UpdateMessageAgentRunID(userMessage.ID, run.ID)

	if inputType == "normal" && taskType == "text_chat" {
		return svc.executeChatTurn(userID, conversation, userMessage, run, request)
	}
	if inputType == "normal" {
		return svc.createClarifyingTurn(userID, conversation, userMessage, run, content)
	}

	if err := svc.dao.AnswerFollowUpQuestions(userID, request.AnsweredQuestionIDs, content); err != nil {
		return nil, err
	}
	return svc.executeGeneration(userID, conversation, userMessage, run, content, request)
}

// ListArtifacts 查询会话产物列表，并校验用户归属。
func (svc *AgentService) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	if _, err := svc.dao.FindConversation(userID, conversationID); err != nil {
		return nil, err
	}
	return svc.dao.ListArtifacts(userID, conversationID)
}

// FindArtifact 查询产物并返回本地下载路径。
func (svc *AgentService) FindArtifact(userID uint, artifactID uint) (model.Artifact, string, error) {
	artifact, err := svc.dao.FindArtifact(userID, artifactID)
	if err != nil {
		return artifact, "", err
	}
	return artifact, svc.store.Path(artifact.ObjectKey), nil
}

// ListRunEvents 查询 Agent Run 的步骤事件。
func (svc *AgentService) ListRunEvents(userID uint, runID uint) ([]model.AgentStep, error) {
	return svc.dao.ListAgentSteps(userID, runID)
}

// GetModelConfig 查询当前用户绑定的模型配置，不存在时返回默认配置。
func (svc *AgentService) GetModelConfig(userID uint) (model.UserModelConfig, error) {
	config, err := svc.dao.FindUserModelConfig(userID)
	if err == nil {
		return config, nil
	}

	return model.UserModelConfig{
		UserID:                      userID,
		Provider:                    "deepseek-anthropic",
		ChatModel:                   "deepseek-v4-pro",
		ImageModel:                  "",
		BaseURL:                     "https://api.deepseek.com/anthropic",
		APIKey:                      "",
		Temperature:                 "0.7",
		AnthropicAuthToken:          "",
		AnthropicBaseURL:            "https://api.deepseek.com/anthropic",
		AnthropicModel:              "deepseek-v4-pro",
		AnthropicDefaultOpusModel:   "deepseek-v4-pro",
		AnthropicDefaultSonnetModel: "deepseek-v4-pro",
		AnthropicDefaultHaikuModel:  "deepseek-v4-pro",
		ClaudeCodeSubagentModel:     "deepseek-v4-pro",
		ClaudeCodeMaxOutputTokens:   "32000",
	}, nil
}

// SaveModelConfig 保存当前用户绑定的模型配置。
func (svc *AgentService) SaveModelConfig(userID uint, request agent_request.SaveModelConfigRequest) (model.UserModelConfig, error) {
	config := model.UserModelConfig{
		UserID:      userID,
		Provider:    strings.TrimSpace(request.Provider),
		ChatModel:   strings.TrimSpace(request.ChatModel),
		ImageModel:  strings.TrimSpace(request.ImageModel),
		BaseURL:     strings.TrimSpace(request.BaseURL),
		APIKey:      strings.TrimSpace(request.APIKey),
		Temperature: strings.TrimSpace(request.Temperature),

		AnthropicAuthToken:          strings.TrimSpace(request.AnthropicAuthToken),
		AnthropicBaseURL:            strings.TrimSpace(request.AnthropicBaseURL),
		AnthropicModel:              strings.TrimSpace(request.AnthropicModel),
		AnthropicDefaultOpusModel:   strings.TrimSpace(request.AnthropicDefaultOpusModel),
		AnthropicDefaultSonnetModel: strings.TrimSpace(request.AnthropicDefaultSonnetModel),
		AnthropicDefaultHaikuModel:  strings.TrimSpace(request.AnthropicDefaultHaikuModel),
		ClaudeCodeSubagentModel:     strings.TrimSpace(request.ClaudeCodeSubagentModel),
		ClaudeCodeMaxOutputTokens:   strings.TrimSpace(request.ClaudeCodeMaxOutputTokens),
	}
	if config.Provider == "" {
		config.Provider = "deepseek-anthropic"
	}
	if config.ChatModel == "" {
		config.ChatModel = "deepseek-v4-pro"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com/anthropic"
	}
	if config.Temperature == "" {
		config.Temperature = "0.7"
	}
	if config.AnthropicAuthToken == "" {
		config.AnthropicAuthToken = config.APIKey
	}
	if config.APIKey == "" {
		config.APIKey = config.AnthropicAuthToken
	}
	if config.AnthropicBaseURL == "" {
		config.AnthropicBaseURL = config.BaseURL
	}
	if config.AnthropicModel == "" {
		config.AnthropicModel = config.ChatModel
	}
	if config.AnthropicDefaultOpusModel == "" {
		config.AnthropicDefaultOpusModel = config.AnthropicModel
	}
	if config.AnthropicDefaultSonnetModel == "" {
		config.AnthropicDefaultSonnetModel = config.AnthropicModel
	}
	if config.AnthropicDefaultHaikuModel == "" {
		config.AnthropicDefaultHaikuModel = config.AnthropicModel
	}
	if config.ClaudeCodeSubagentModel == "" {
		config.ClaudeCodeSubagentModel = config.AnthropicModel
	}
	if config.ClaudeCodeMaxOutputTokens == "" {
		config.ClaudeCodeMaxOutputTokens = "32000"
	}

	if err := svc.dao.SaveUserModelConfig(&config); err != nil {
		return config, err
	}
	return svc.GetModelConfig(userID)
}

func (svc *AgentService) executeChatTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveModelConfig(userID, request.ModelConfig)
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
		"conversation":        conversation,
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

func (svc *AgentService) resolveModelConfig(userID uint, requestConfig map[string]interface{}) (model.UserModelConfig, error) {
	config, err := svc.GetModelConfig(userID)
	if err != nil {
		return config, err
	}
	applyConfigOverride := func(field *string, key string) {
		if value := stringConfigValue(requestConfig, key); value != "" {
			*field = value
		}
	}
	applyConfigOverride(&config.Provider, "provider")
	applyConfigOverride(&config.ChatModel, "chat_model")
	applyConfigOverride(&config.ImageModel, "image_model")
	applyConfigOverride(&config.BaseURL, "base_url")
	applyConfigOverride(&config.APIKey, "api_key")
	applyConfigOverride(&config.Temperature, "temperature")
	applyConfigOverride(&config.AnthropicAuthToken, "anthropic_auth_token")
	applyConfigOverride(&config.AnthropicBaseURL, "anthropic_base_url")
	applyConfigOverride(&config.AnthropicModel, "anthropic_model")
	applyConfigOverride(&config.AnthropicDefaultOpusModel, "anthropic_default_opus_model")
	applyConfigOverride(&config.AnthropicDefaultSonnetModel, "anthropic_default_sonnet_model")
	applyConfigOverride(&config.AnthropicDefaultHaikuModel, "anthropic_default_haiku_model")
	applyConfigOverride(&config.ClaudeCodeSubagentModel, "claude_code_subagent_model")
	applyConfigOverride(&config.ClaudeCodeMaxOutputTokens, "claude_code_max_output_tokens")

	if strings.TrimSpace(config.Provider) == "" {
		config.Provider = "deepseek-anthropic"
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

func stringConfigValue(values map[string]interface{}, key string) string {
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

func modelCallTarget(config model.UserModelConfig) string {
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	if strings.Contains(provider, "anthropic") || strings.Contains(provider, "claude") ||
		(provider == "deepseek" && strings.Contains(strings.ToLower(config.AnthropicBaseURL), "anthropic")) {
		return strings.TrimRight(config.AnthropicBaseURL, "/") + "/v1/messages"
	}
	return strings.TrimRight(config.BaseURL, "/") + "/chat/completions"
}

// createClarifyingTurn 生成补充问题轮次。
func (svc *AgentService) createClarifyingTurn(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string) (map[string]interface{}, error) {
	_ = svc.createStepWithThinking(run.ID, "planner_agent", content, "需要先确认图片或 HTML 产物的目标、风格和尺寸。", "判断当前是图片生成任务，信息不足时先生成补充问题。", "Planner 未调用外部模型，reasoning_content 为空。")
	assistantMessage := model.Message{
		ConversationID: conversation.ID,
		UserID:         userID,
		Role:           "assistant",
		InputType:      "follow_up_questions",
		Content:        "我需要先确认几个细节，然后再进入多 Agent 生成流程。",
		AgentRunID:     run.ID,
	}
	if err := svc.dao.CreateMessage(&assistantMessage); err != nil {
		return nil, err
	}

	questions := []model.FollowUpQuestion{
		{ConversationID: conversation.ID, MessageID: assistantMessage.ID, UserID: userID, Question: "希望生成图片、HTML 页面，还是两者都生成？", Status: "pending"},
		{ConversationID: conversation.ID, MessageID: assistantMessage.ID, UserID: userID, Question: "请补充风格、尺寸、用途或必须出现的元素。", Status: "pending"},
	}
	if err := svc.dao.CreateFollowUpQuestions(questions); err != nil {
		return nil, err
	}
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "waiting_questions"})
	steps, _ := svc.dao.ListAgentSteps(userID, run.ID)

	return map[string]interface{}{
		"user_message":        userMessage,
		"assistant_message":   assistantMessage,
		"follow_up_questions": questions,
		"agent_run":           run,
		"agent_steps":         steps,
		"conversation":        conversation,
	}, nil
}

// executeGeneration 执行固定多 Agent DAG，并保存生成产物。
func (svc *AgentService) executeGeneration(userID uint, conversation model.Conversation, userMessage model.Message, run model.AgentRun, content string, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	config, err := svc.resolveModelConfig(userID, request.ModelConfig)
	if err != nil {
		_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "failed", "error_message": err.Error()})
		return nil, err
	}
	memories, _ := svc.dao.ListContextMemories(userID, conversation.ID, 5)
	contextBytes, _ := json.Marshal(memories)
	_ = svc.createStepWithThinking(run.ID, "context_agent", string(contextBytes), "已读取最近上下文和长期记忆。", "检索当前会话记忆、用户偏好和相关历史任务。", "")

	prompt := svc.composePrompt(conversation.ID, content, memories)
	_ = svc.createStepWithThinking(run.ID, "prompt_agent", content, prompt, "把补充问题答案、历史上下文和任务类型整理为生成提示词。", "")

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
	_ = svc.createStepWithThinking(run.ID, "image_agent", prompt, "已生成图片候选。", "根据图片生成模式选择图片模型并生成可预览产物。", "return_reasoning=true；图片生成模型的 reasoning 内容会写入该字段。")
	_ = svc.createStepWithThinking(run.ID, "html_agent", prompt, "已生成 HTML 预览文件。", "将生成结果组织成可在右侧 iframe 预览的 HTML 文件。", "return_reasoning=true；HTML 生成模型的 reasoning 内容会写入该字段。")
	_ = svc.createStepWithThinking(run.ID, "review_agent", prompt, "mock review passed", "检查产物是否可预览、可下载，并确认元数据完整。", "")

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
		if err := svc.dao.CreateArtifact(&artifact); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	_ = svc.createStepWithThinking(run.ID, "artifact_agent", prompt, "产物已保存并完成元数据入库。", "保存对象存储文件并把预览地址、下载元数据写入数据库。", "")

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

	memory := model.ContextMemory{
		ConversationID: conversation.ID,
		UserID:         userID,
		Kind:           "summary",
		Content:        "用户补充需求：" + content,
		Score:          10,
	}
	_ = svc.dao.CreateContextMemory(&memory)
	_ = svc.dao.UpdateAgentRun(run.ID, map[string]interface{}{"status": "completed"})
	steps, _ := svc.dao.ListAgentSteps(userID, run.ID)

	return map[string]interface{}{
		"user_message":      userMessage,
		"assistant_message": assistantMessage,
		"artifacts":         artifacts,
		"agent_run":         run,
		"agent_steps":       steps,
		"conversation":      conversation,
	}, nil
}

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

func normalizeTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "image_generation":
		return "image_generation"
	default:
		return "text_chat"
	}
}

func makeConversationTitle(content string) string {
	title := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if title == "" {
		return "新的图片 Agent 会话"
	}
	runes := []rune(title)
	if len(runes) > 18 {
		return string(runes[:18]) + "..."
	}
	return title
}

// detectIntent 根据用户输入粗略识别任务类型。
func (svc *AgentService) detectIntent(content string, taskType string) string {
	if taskType == "text_chat" {
		return "text_chat"
	}
	if taskType == "image_generation" {
		return "image"
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "html") || strings.Contains(content, "页面") {
		return "html"
	}
	if strings.Contains(content, "图") || strings.Contains(lower, "image") {
		return "image"
	}
	return "mixed"
}

// composePrompt 将当前输入和历史记忆组合为 Provider 提示词。
func (svc *AgentService) composePrompt(conversationID uint, content string, memories []model.ContextMemory) string {
	parts := []string{fmt.Sprintf("conversation:%d", conversationID), content}
	for _, memory := range memories {
		parts = append(parts, memory.Content)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// IsNotFound 判断错误是否为 GORM 记录不存在。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// SafeDownloadName 清理下载文件名，避免路径穿越。
func SafeDownloadName(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return "artifact"
	}
	return name
}

// ListModelConfigs 获取全局模型配置列表。
func (svc *AgentService) ListModelConfigs(page, perPage int, isTextModel, isImageModel string) ([]model.ModelConfig, int64, error) {
	var configs []model.ModelConfig
	query := database.DB.Model(&model.ModelConfig{})

	if isTextModel != "" {
		query = query.Where("is_text_model = ?", isTextModel == "true")
	}
	if isImageModel != "" {
		query = query.Where("is_image_model = ?", isImageModel == "true")
	}

	var total int64
	query.Count(&total)

	offset := (page - 1) * perPage
	err := query.Order("id desc").Offset(offset).Limit(perPage).Find(&configs).Error
	return configs, total, err
}

// GetModelConfigByID 根据 ID 获取模型配置。
func (svc *AgentService) GetModelConfigByID(id uint) (model.ModelConfig, error) {
	var config model.ModelConfig
	err := database.DB.Where("id = ?", id).First(&config).Error
	return config, err
}

// CreateModelConfig 创建新的模型配置。
func (svc *AgentService) CreateModelConfig(request struct {
	ModelName       string                 `json:"model_name" form:"model_name"`
	RequestURL      string                 `json:"request_url" form:"request_url"`
	IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
	IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
	SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
	ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
}) (model.ModelConfig, error) {
	config := model.ModelConfig{
		ModelName:       request.ModelName,
		RequestURL:      request.RequestURL,
		IsTextModel:     request.IsTextModel,
		IsImageModel:    request.IsImageModel,
		SupportThinking: request.SupportThinking,
	}

	if request.ConfigInfo != nil {
		config.ConfigInfo = model.JSONMap(request.ConfigInfo)
	}

	err := database.DB.Create(&config).Error
	return config, err
}

// UpdateModelConfig 更新模型配置。
func (svc *AgentService) UpdateModelConfig(id uint, request struct {
	ModelName       string                 `json:"model_name" form:"model_name"`
	RequestURL      string                 `json:"request_url" form:"request_url"`
	IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
	IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
	SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
	ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
}) (model.ModelConfig, error) {
	var config model.ModelConfig
	err := database.DB.Where("id = ?", id).First(&config).Error
	if err != nil {
		return config, err
	}

	if request.ModelName != "" {
		config.ModelName = request.ModelName
	}
	if request.RequestURL != "" {
		config.RequestURL = request.RequestURL
	}
	config.IsTextModel = request.IsTextModel
	config.IsImageModel = request.IsImageModel
	config.SupportThinking = request.SupportThinking

	if request.ConfigInfo != nil {
		config.ConfigInfo = model.JSONMap(request.ConfigInfo)
	}

	err = database.DB.Save(&config).Error
	return config, err
}

// DeleteModelConfig 删除模型配置。
func (svc *AgentService) DeleteModelConfig(id uint) error {
	return database.DB.Delete(&model.ModelConfig{}, id).Error
}

// ListTextModels 获取所有文本模型配置。
func (svc *AgentService) ListTextModels() ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	err := database.DB.Where("is_text_model = ?", true).Order("id desc").Find(&configs).Error
	return configs, err
}

// ListImageModels 获取所有图片模型配置。
func (svc *AgentService) ListImageModels() ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	err := database.DB.Where("is_image_model = ?", true).Order("id desc").Find(&configs).Error
	return configs, err
}
