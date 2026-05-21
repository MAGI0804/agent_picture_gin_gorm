package agent_svc

import (
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/logger"
)

// ListMessages 查询指定会话下的消息列表，并校验用户归属。
func (svc *AgentService) ListMessages(userID uint, conversationID uint) ([]model.Message, error) {
	if _, err := svc.dao.FindConversation(userID, conversationID); err != nil {
		return nil, err
	}
	return svc.dao.ListMessages(userID, conversationID)
}

// OptimizePrompt 使用 deepseek-v4-pro 对用户输入的提示词进行智能优化。
func (svc *AgentService) OptimizePrompt(userID uint, request agent_request.OptimizePromptRequest) (map[string]interface{}, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return nil, errors.New("content cannot be empty")
	}
	targetLength := normalizePromptTargetLength(request.TargetLength)
	
	logger.Info("[OptimizePrompt] 开始优化提示词",
		zap.Uint("user_id", userID),
		zap.Int("original_length", len([]rune(content))),
		zap.Int("target_length", targetLength),
	)
	
	// 决定优化类型
	optimizationType := "enhance"
	if len([]rune(content)) > targetLength {
		optimizationType = "shorten"
		logger.Info("[OptimizePrompt] 提示词较长，使用缩短模式",
			zap.String("optimization_type", optimizationType),
		)
	} else {
		logger.Info("[OptimizePrompt] 提示词较短，使用增强模式",
			zap.String("optimization_type", optimizationType),
		)
	}
	
	optimizedPrompt, err := svc.optimizePromptWithDeepseek(userID, content, targetLength, optimizationType)
	if err != nil {
		logger.Error("[OptimizePrompt] deepseek-v4-pro 优化失败", zap.Error(err))
		return nil, errors.Wrap(err, "智能优化失败，请重试")
	}
	
	optimizedPrompt = strings.TrimSpace(optimizedPrompt)
	if optimizedPrompt == "" {
		logger.Error("[OptimizePrompt] 优化结果为空")
		return nil, errors.New("优化结果为空，请重试")
	}
	
	finalLength := len([]rune(optimizedPrompt))
	logger.Info("[OptimizePrompt] 优化完成",
		zap.Int("original_length", len([]rune(content))),
		zap.Int("optimized_length", finalLength),
		zap.String("optimized_prompt", optimizedPrompt),
	)
	
	if finalLength > targetLength+100 {
		logger.Warn("[OptimizePrompt] 优化结果仍较长，但继续使用",
			zap.Int("length", finalLength),
			zap.Int("target_length", targetLength),
		)
	}
	
	return map[string]interface{}{
		"original_prompt":  content,
		"optimized_prompt": optimizedPrompt,
		"target_length":    targetLength,
		"original_length":  len([]rune(content)),
		"optimized_length": finalLength,
	}, nil
}

// SendMessage 根据输入类型处理普通对话或补充问题回答。
func (svc *AgentService) SendMessage(userID uint, conversationID uint, request agent_request.SendMessageRequest) (map[string]interface{}, error) {
	conversation, err := svc.dao.FindConversation(userID, conversationID)
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
	optimizedPrompt := strings.TrimSpace(request.OptimizedPrompt)
	isOptimized := request.IsOptimized && optimizedPrompt != ""

	userMessage := model.Message{
		ConversationID:  conversationID,
		UserID:          userID,
		Role:            "user",
		InputType:       inputType,
		Content:         content,
		IsOptimized:     isOptimized,
		OptimizedPrompt: optimizedPrompt,
	}
	if err := svc.dao.CreateMessage(&userMessage); err != nil {
		return nil, err
	}
	if shouldRefreshConversationTitle(conversation.Title) {
		conversation.Title = makeConversationTitle(content)
		_ = svc.dao.UpdateConversationTitle(userID, conversationID, conversation.Title)
	}

	run := model.AgentRun{
		ConversationID:   conversationID,
		UserID:           userID,
		TriggerMessageID: userMessage.ID,
		Status:           "running",
		Intent:           svc.detectIntent(content, taskType),
		IsOptimized:      isOptimized,
		OptimizedPrompt:  optimizedPrompt,
	}
	if err := svc.dao.CreateAgentRun(&run); err != nil {
		return nil, err
	}

	if inputType == "normal" && taskType == "text_chat" {
		return svc.executeChatTurn(userID, conversation, userMessage, run, request)
	}
	if inputType == "normal" && taskType == "image_generation" {
		return svc.executeGeneration(userID, conversation, userMessage, run, content, request)
	}
	if inputType == "normal" {
		return svc.createClarifyingTurn(userID, conversation, userMessage, run, content, request.TextModelConfigID)
	}

	if err := svc.dao.AnswerFollowUpQuestions(userID, request.AnsweredQuestionIDs, content); err != nil {
		return nil, err
	}
	return svc.executeGeneration(userID, conversation, userMessage, run, content, request)
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

func shouldRefreshConversationTitle(title string) bool {
	title = strings.TrimSpace(title)
	return title == "" || title == "新的图片 Agent 会话"
}

// detectIntent 根据用户输入粗略识别任务类型。
func (svc *AgentService) detectIntent(content string, taskType string) string {
	if taskType == "text_chat" {
		return "text_chat"
	}
	if taskType == "image_generation" {
		return "image_generation"
	}
	if taskType == "html_generation" {
		return "html_generation"
	}
	if taskType == "mixed_generation" {
		return "mixed_generation"
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "html") || strings.Contains(content, "页面") {
		return "html_generation"
	}
	if strings.Contains(content, "图") || strings.Contains(lower, "image") {
		return "image_generation"
	}
	return "mixed_generation"
}

func normalizeTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "image_generation":
		return "image_generation"
	case "html_generation":
		return "html_generation"
	case "mixed_generation":
		return "mixed_generation"
	default:
		return "text_chat"
	}
}
