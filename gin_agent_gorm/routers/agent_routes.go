package routers

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller/agent_ctrl"
	"gin-biz-web-api/internal/middleware"
)

// registerAgentRoutes 注册图片 AI Agent 工作台所需的业务路由。
// 所有路由均需要 JWT 认证。
func registerAgentRoutes(api *gin.RouterGroup) {
	agentCtrl := new(agent_ctrl.AgentController)
	modelConfigCtrl := new(agent_ctrl.ModelConfigController)
	agentGroup := api.Group("")
	agentGroup.Use(middleware.AuthJWT())
	{
		// 会话管理路由
		agentGroup.GET("/conversations", agentCtrl.ListConversations)   // GET /api/conversations - 获取当前用户的会话列表
		agentGroup.POST("/conversations", agentCtrl.CreateConversation) // POST /api/conversations - 创建新的 AI Agent 会话

		// 消息路由
		agentGroup.GET("/conversations/:id/messages", agentCtrl.ListMessages) // GET /api/conversations/:id/messages - 获取指定会话的消息列表
		agentGroup.POST("/conversations/:id/messages", agentCtrl.SendMessage) // POST /api/conversations/:id/messages - 发送消息（普通对话或补充问题回答）

		// 产物路由
		agentGroup.GET("/conversations/:id/artifacts", agentCtrl.ListArtifacts) // GET /api/conversations/:id/artifacts - 获取会话的生成产物列表
		agentGroup.GET("/artifacts/:id/download", agentCtrl.DownloadArtifact)   // GET /api/artifacts/:id/download - 下载指定产物文件

		// Agent Run 事件路由
		agentGroup.GET("/runs/:id/events", agentCtrl.RunEvents)   // GET /api/runs/:id/events - SSE 流式返回 Agent 执行步骤事件
		agentGroup.GET("/runs/:id/steps", agentCtrl.ListRunSteps) // GET /api/runs/:id/steps - 获取 Agent Run 的步骤列表

		// 用户模型配置路由
		agentGroup.GET("/settings/model-config", agentCtrl.GetModelConfig)  // GET /api/settings/model-config - 获取当前用户的模型配置
		agentGroup.PUT("/settings/model-config", agentCtrl.SaveModelConfig) // PUT /api/settings/model-config - 保存用户的模型配置

		// 全局模型配置路由（管理员功能）
		agentGroup.GET("/model-configs", modelConfigCtrl.ListModelConfigs)             // GET /api/model-configs - 获取全局模型配置列表
		agentGroup.GET("/model-configs/:id", modelConfigCtrl.GetModelConfig)           // GET /api/model-configs/:id - 获取单个模型配置详情
		agentGroup.POST("/model-configs", modelConfigCtrl.CreateModelConfig)           // POST /api/model-configs - 创建新的模型配置
		agentGroup.PUT("/model-configs/:id", modelConfigCtrl.UpdateModelConfig)        // PUT /api/model-configs/:id - 更新模型配置
		agentGroup.DELETE("/model-configs/:id", modelConfigCtrl.DeleteModelConfig)     // DELETE /api/model-configs/:id - 删除模型配置
		agentGroup.GET("/model-configs/text-models", modelConfigCtrl.ListTextModels)   // GET /api/model-configs/text-models - 获取所有文本模型配置
		agentGroup.GET("/model-configs/image-models", modelConfigCtrl.ListImageModels) // GET /api/model-configs/image-models - 获取所有图片模型配置
	}
}
