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
		group.POST("/conversations/:id/runs/async", ctrl.CreateRunAsync)
		group.GET("/conversations/:id/artifacts", ctrl.ListArtifacts)
		group.POST("/conversations/:id/artifacts/upload", ctrl.UploadArtifact)
		group.GET("/runs/:id", ctrl.GetRun)
		group.POST("/runs/:id/cancel", ctrl.CancelRun)
		group.POST("/runs/:id/resume", ctrl.ResumeRun)
		group.GET("/runs/:id/events", ctrl.RunEvents)
		group.GET("/memories", ctrl.SearchMemories)
		group.POST("/memories/search", ctrl.SearchMemories)
		group.POST("/memories/:id/promote", ctrl.PromoteMemoryProposal)
		group.PATCH("/memories/:id", ctrl.UpdateMemory)
		group.DELETE("/memories/:id", ctrl.DeleteMemory)
		group.GET("/evolution/summary", ctrl.EvolutionSummary)
		group.GET("/evolution/prompt-versions", ctrl.ListPromptVersions)
		group.POST("/evolution/prompt-versions/draft", ctrl.DraftPromptVersion)
		group.POST("/evolution/prompt-versions/:id/review", ctrl.MovePromptVersionToReview)
		group.POST("/evolution/prompt-versions/:id/activate", ctrl.ActivatePromptVersion)
		group.POST("/evolution/prompt-versions/:id/archive", ctrl.ArchivePromptVersion)
		group.GET("/eval/cases", ctrl.ListEvalCases)
		group.POST("/eval/cases", ctrl.CreateEvalCase)
		group.GET("/eval/runs", ctrl.ListEvalRuns)
		group.POST("/eval/runs", ctrl.CreateEvalRun)
		group.GET("/artifacts/:id/versions", ctrl.ListArtifactVersions)
		group.GET("/artifacts/:id/preview", ctrl.PreviewArtifact)
		group.GET("/artifacts/:id/download", ctrl.DownloadArtifact)
		group.POST("/artifacts/:id/edit", ctrl.EditArtifact)
		group.POST("/artifacts/:id/render-text", ctrl.RenderArtifactText)
		group.POST("/artifacts/:id/feedback", ctrl.RecordArtifactFeedback)
		group.POST("/artifacts/:id/select", ctrl.SelectArtifact)
	}
}
