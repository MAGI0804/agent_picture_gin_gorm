package agent_svc

import (
	"strings"

	"github.com/pkg/errors"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
)

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
	}
	if err := svc.dao.CreateAgentRun(&run); err != nil {
		return nil, err
	}

	if inputType == "normal" && taskType == "text_chat" {
		return svc.executeChatTurn(userID, conversation, userMessage, run, request)
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