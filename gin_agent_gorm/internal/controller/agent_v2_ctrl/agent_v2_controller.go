package agent_v2_ctrl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/internal/service/agent_v2/app"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

// AgentV2Controller Agent V2 控制器，处理 Agent V2 相关的 HTTP 请求
type AgentV2Controller struct{}

// CreateRun 创建一个新的 Agent 运行
func (ctrl *AgentV2Controller) CreateRun(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}

	var request app.CreateRunRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}

	result, err := app.NewService().CreateRun(c.Request.Context(), userID, conversationID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(result)
}

// CreateRunAsync 创建一个异步 Agent Run，并立即返回 queued 状态。
func (ctrl *AgentV2Controller) CreateRunAsync(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}

	var request app.CreateRunRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}

	result, err := app.NewService().CreateRunAsync(c.Request.Context(), userID, conversationID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(result)
}

// GetRun 获取指定的 Agent 运行信息
func (ctrl *AgentV2Controller) GetRun(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	result, err := app.NewService().GetRun(userID, runID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "run not found")
		return
	}
	responses.New(c).ToResponse(result)
}

// CancelRun 取消 queued/running 状态的异步 Agent Run。
func (ctrl *AgentV2Controller) CancelRun(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	result, err := app.NewService().CancelRun(userID, runID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "run not found")
		return
	}
	responses.New(c).ToResponse(result)
}

// RunEvents 获取 Agent 运行的事件流（SSE）
func (ctrl *AgentV2Controller) RunEvents(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	cursor := 0
	if rawCursor := c.Query("cursor"); rawCursor != "" {
		parsed, err := strconv.Atoi(rawCursor)
		if err != nil || parsed < 0 {
			responses.New(c).ToErrorResponse(errcode.BadRequest, "cursor params error")
			return
		}
		cursor = parsed
	}
	events, err := app.NewService().ListRunEvents(userID, runID, cursor)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "run not found")
		return
	}
	if c.Query("format") == "json" || c.Query("cursor") != "" {
		responses.New(c).ToResponse(events)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	for _, event := range events.Events {
		payload, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(payload))
		c.Writer.Flush()
	}
	donePayload, _ := json.Marshal(gin.H{"cursor": events.Cursor})
	_, _ = fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(donePayload))
	c.Writer.Flush()
}

// SearchMemories 查询当前用户的 V2 记忆。
func (ctrl *AgentV2Controller) SearchMemories(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	var request app.MemorySearchRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	memories, err := app.NewService().SearchMemories(userID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"memories": memories})
}

// DeleteMemory 删除当前用户的一条 V2 记忆。
func (ctrl *AgentV2Controller) DeleteMemory(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	memoryID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	if err := app.NewService().DeleteMemory(userID, memoryID); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"deleted": true})
}

// UpdateMemory edits or disables one V2 memory.
func (ctrl *AgentV2Controller) UpdateMemory(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	memoryID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.UpdateMemoryRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	memory, err := app.NewService().UpdateMemory(userID, memoryID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"memory": memory})
}

// PromoteMemoryProposal 将一条候选记忆确认为稳定记忆。
func (ctrl *AgentV2Controller) PromoteMemoryProposal(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	memoryID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.PromoteMemoryRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	memory, promoted, err := app.NewService().PromoteMemoryProposal(userID, memoryID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"memory": memory, "promoted": promoted})
}

// SelectArtifact 选择一个候选产物。
func (ctrl *AgentV2Controller) SelectArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.SelectArtifactRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	if err := app.NewService().SelectArtifact(userID, artifactID, request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"selected": true})
}

// ListArtifacts 列出当前会话的 V2 产物。
func (ctrl *AgentV2Controller) ListArtifacts(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	artifacts, err := app.NewService().ListArtifacts(userID, conversationID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "conversation not found")
		return
	}
	responses.New(c).ToResponse(gin.H{"artifacts": artifacts})
}

// UploadArtifact stores a user uploaded image as a V2 artifact/version.
func (ctrl *AgentV2Controller) UploadArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "upload file is required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, app.MaxImageUploadBytes+1))
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "read upload file failed")
		return
	}
	artifact, version, err := app.NewService().UploadArtifact(userID, app.UploadArtifactInput{
		ConversationID: conversationID,
		FileName:       header.Filename,
		ContentType:    header.Header.Get("Content-Type"),
		Content:        content,
	})
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"artifact": artifact, "version": version})
}

// ListArtifactVersions 列出当前用户有权访问的产物版本。
func (ctrl *AgentV2Controller) ListArtifactVersions(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	versions, err := app.NewService().ListArtifactVersions(userID, artifactID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "artifact not found")
		return
	}
	responses.New(c).ToResponse(gin.H{"versions": versions})
}

// EditArtifact appends a provider-generated edit as a child artifact version.
func (ctrl *AgentV2Controller) EditArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.EditArtifactRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	version, err := app.NewService().EditArtifact(c.Request.Context(), userID, artifactID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"version": version})
}

// DownloadArtifact 下载当前用户有权访问的 V2 产物。
func (ctrl *AgentV2Controller) DownloadArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	artifact, filePath, err := app.NewService().DownloadArtifact(userID, artifactID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "artifact not found")
		return
	}
	c.Header("Content-Type", artifact.MimeType)
	c.FileAttachment(filePath, agent_svc.SafeDownloadName(artifact.Name))
}

// PreviewArtifact 内联预览当前用户有权访问的 V2 产物。
func (ctrl *AgentV2Controller) PreviewArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	artifact, filePath, err := app.NewService().PreviewArtifact(userID, artifactID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "artifact not found")
		return
	}
	c.Header("Content-Type", artifact.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", agent_svc.SafeDownloadName(artifact.Name)))
	c.File(filePath)
}

// RecordArtifactFeedback 写入当前用户对 V2 产物的反馈。
func (ctrl *AgentV2Controller) RecordArtifactFeedback(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.ArtifactFeedbackRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	if err := app.NewService().RecordArtifactFeedback(userID, artifactID, request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(gin.H{"recorded": true})
}

// parseID 解析 URL 参数中的 ID
func (ctrl *AgentV2Controller) parseID(c *gin.Context, key string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || id == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "ID params error")
		return 0, false
	}
	return uint(id), true
}

// ResumeRun submits a clarification answer and requeues the same Agent V2 run.
func (ctrl *AgentV2Controller) ResumeRun(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	var request app.ResumeRunRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "request params error")
		return
	}
	result, err := app.NewService().ResumeRun(c.Request.Context(), userID, runID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(result)
}
