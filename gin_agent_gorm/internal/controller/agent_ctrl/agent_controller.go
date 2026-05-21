package agent_ctrl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

// AgentController 处理图片 AI Agent 工作台相关 HTTP 请求。
// 提供会话管理、消息处理、产物管理、模型配置等功能。
type AgentController struct {
}

// GetModelConfig 获取当前用户绑定的模型配置。
// GET /api/settings/model-config
//
// 返回数据:
//   - model_config: 用户模型配置对象
func (ctrl *AgentController) GetModelConfig(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	config, err := agent_svc.NewAgentService().GetModelConfig(userID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(gin.H{"model_config": config})
}

// SaveModelConfig 保存当前用户绑定的模型配置。
// PUT /api/settings/model-config
//
// 请求参数:
//   - Provider: 模型供应商（如 deepseek-anthropic）
//   - ChatModel: 对话模型名称
//   - ImageModel: 图片模型名称
//   - BaseURL: API 基础地址
//   - APIKey: API 密钥
//   - Temperature: 温度参数
//   - AnthropicAuthToken: Anthropic 鉴权令牌
//   - AnthropicBaseURL: Anthropic API 地址
//   - AnthropicModel: Anthropic 默认模型
//   - ... 其他 Anthropic 相关配置
//
// 返回数据:
//   - model_config: 更新后的用户模型配置对象
func (ctrl *AgentController) SaveModelConfig(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	var request agent_request.SaveModelConfigRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}
	config, err := agent_svc.NewAgentService().SaveModelConfig(userID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(gin.H{"model_config": config})
}

// ListConversations 获取当前用户的会话列表。
// GET /api/conversations
//
// 返回数据:
//   - conversations: 会话列表（按更新时间倒序）
func (ctrl *AgentController) ListConversations(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversations, err := agent_svc.NewAgentService().ListConversations(userID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(gin.H{"conversations": conversations})
}

// CreateConversation 创建一个新的 AI Agent 会话。
// POST /api/conversations
//
// 请求参数:
//   - Title: 会话标题（可选，默认自动生成）
//
// 返回数据:
//   - conversation: 创建的会话对象
func (ctrl *AgentController) CreateConversation(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	var request agent_request.CreateConversationRequest
	_ = c.ShouldBind(&request)
	conversation, err := agent_svc.NewAgentService().CreateConversation(userID, request.Title)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}
	responses.New(c).ToResponse(gin.H{"conversation": conversation})
}

// ListMessages 获取指定会话下的消息列表。
// GET /api/conversations/:id/messages
//
// 路径参数:
//   - id: 会话 ID
//
// 返回数据:
//   - messages: 消息列表（按时间顺序）
func (ctrl *AgentController) ListMessages(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	messages, err := agent_svc.NewAgentService().ListMessages(userID, conversationID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "会话不存在")
		return
	}
	responses.New(c).ToResponse(gin.H{"messages": messages})
}

// SendMessage 发送消息（普通对话或补充问题回答）。
// POST /api/conversations/:id/messages
//
// 路径参数:
//   - id: 会话 ID
//
// 请求参数:
//   - input_type: 输入类型（normal 或 answer_to_questions）
//   - task_type: 任务类型（text_chat 或 image_generation）
//   - content: 消息内容
//   - model_config: 模型配置（可选，覆盖默认配置）
//   - answered_question_ids: 回答的问题 ID 列表（仅 answer_to_questions 类型需要）
//
// 返回数据:
//   - user_message: 用户消息
//   - assistant_message: 助手回复
//   - follow_up_questions: 补充问题列表（如有）
//   - artifacts: 生成产物列表（如有）
//   - agent_run: Agent 任务信息
//   - agent_steps: Agent 执行步骤
//   - conversation: 会话信息
func (ctrl *AgentController) SendMessage(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}

	var request agent_request.SendMessageRequest
	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}
	result, err := agent_svc.NewAgentService().SendMessage(userID, conversationID, request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), err.Error())
		return
	}
	responses.New(c).ToResponse(result)
}

// ListArtifacts 获取指定会话下的生成产物列表。
// GET /api/conversations/:id/artifacts
//
// 路径参数:
//   - id: 会话 ID
//
// 返回数据:
//   - artifacts: 产物列表
func (ctrl *AgentController) ListArtifacts(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	conversationID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	artifacts, err := agent_svc.NewAgentService().ListArtifacts(userID, conversationID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "会话不存在")
		return
	}
	responses.New(c).ToResponse(gin.H{"artifacts": artifacts})
}

// DownloadArtifact 下载指定产物文件。
// GET /api/artifacts/:id/download
//
// 路径参数:
//   - id: 产物 ID
//
// 返回: 文件下载流
func (ctrl *AgentController) DownloadArtifact(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	artifactID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	artifact, filePath, err := agent_svc.NewAgentService().FindArtifact(userID, artifactID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "产物不存在")
		return
	}
	c.Header("Content-Type", artifact.MimeType)
	c.FileAttachment(filePath, agent_svc.SafeDownloadName(artifact.Name))
}

// RunEvents 以 SSE（Server-Sent Events）格式返回 Agent Run 的步骤事件。
// GET /api/runs/:id/events
//
// 路径参数:
//   - id: Agent Run ID
//
// 返回: SSE 流，包含 agent_step 事件和 done 事件
func (ctrl *AgentController) RunEvents(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	steps, err := agent_svc.NewAgentService().ListRunEvents(userID, runID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "任务不存在")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	for _, step := range steps {
		payload, _ := json.Marshal(step)
		_, _ = fmt.Fprintf(c.Writer, "event: agent_step\ndata: %s\n\n", string(payload))
		c.Writer.Flush()
	}
	_, _ = fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
	c.Writer.Flush()
}

// ListRunSteps 获取指定 Agent Run 的步骤列表。
// GET /api/runs/:id/steps
//
// 路径参数:
//   - id: Agent Run ID
//
// 返回数据:
//   - steps: Agent 步骤列表
func (ctrl *AgentController) ListRunSteps(c *gin.Context) {
	userID := auth.CurrentUserID(c)
	runID, ok := ctrl.parseID(c, "id")
	if !ok {
		return
	}
	steps, err := agent_svc.NewAgentService().ListRunEvents(userID, runID)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "任务不存在")
		return
	}
	responses.New(c).ToResponse(gin.H{"steps": steps})
}

// parseID 从路由参数中解析 uint 类型的 ID。
// 参数:
//   - c: Gin 上下文
//   - key: 参数名
//
// 返回:
//   - uint: 解析后的 ID
//   - bool: 解析是否成功
func (ctrl *AgentController) parseID(c *gin.Context, key string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || id == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "ID 参数错误")
		return 0, false
	}
	return uint(id), true
}
