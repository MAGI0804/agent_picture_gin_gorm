package routers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/middleware"
	"gin-biz-web-api/pkg/config"
)

// RegisterAPIRoutes 注册所有 API 路由入口。
//
// 这里只保留全局路由组和公共中间件，具体业务路由拆分到
// auth_routes.go、agent_routes.go、example_routes.go、test_routes.go。
func RegisterAPIRoutes(r *gin.Engine) {
	setStaticURL(r)

	api := r.Group("/api")
	api.Use(middleware.LimitIP("2000-H"))

	registerTestRoutes(api)
	registerAuthRoutes(api)
	registerAgentRoutes(api)
	registerAgentV2Routes(api)
	registerExampleRoutes(api)
}

// setStaticURL 注册静态资源访问路径。
//
// upload 路径兼容脚手架原有上传能力，artifacts 路径用于 AI Agent 生成产物预览。
func setStaticURL(r *gin.Engine) {
	r.StaticFS(config.GetString("cfg.upload.static_fs_relative_path"), http.Dir(config.GetString("cfg.upload.save_path")))
	if config.GetBool("cfg.ai_agent.storage.static_enabled", false) {
		r.StaticFS(config.GetString("cfg.ai_agent.storage.public_path", "/artifacts"), http.Dir(resolveLocalPath(config.GetString("cfg.ai_agent.storage.local_path", "public/artifacts"))))
	}
}

func resolveLocalPath(localPath string) string {
	if filepath.IsAbs(localPath) {
		return localPath
	}
	return filepath.Join(global.RootPath, localPath)
}
