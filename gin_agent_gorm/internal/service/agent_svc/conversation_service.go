package agent_svc

import (
	"path/filepath"
	"strings"

	"gin-biz-web-api/model"
)

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

// SafeDownloadName 清理下载文件名，避免路径穿越。
func SafeDownloadName(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return "artifact"
	}
	return name
}