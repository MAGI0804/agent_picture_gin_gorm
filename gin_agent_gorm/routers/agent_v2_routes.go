package routers

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller/agent_v2_ctrl"
	"gin-biz-web-api/internal/middleware"
)

// registerAgentV2Routes 注册 Agent V2 相关路由
func registerAgentV2Routes(api *gin.RouterGroup) {
	ctrl := new(agent_v2_ctrl.AgentV2Controller)
	group := api.Group("/v2")
	group.Use(middleware.AuthJWT())
	{
		group.POST("/conversations/:id/runs", ctrl.CreateRun)
		group.GET("/runs/:id", ctrl.GetRun)
		group.GET("/runs/:id/events", ctrl.RunEvents)
		group.GET("/memories", ctrl.SearchMemories)
		group.POST("/memories/search", ctrl.SearchMemories)
		group.DELETE("/memories/:id", ctrl.DeleteMemory)
		group.POST("/artifacts/:id/select", ctrl.SelectArtifact)
	}
}
