package config

import "gin-biz-web-api/pkg/config"

func init() {
	// 注册 AI Agent 配置，支持模型 Provider 和产物存储的后续替换。
	config.Add("cfg.ai_agent", func() map[string]interface{} {
		return map[string]interface{}{
			"auto_migrate": config.Get("AIAgent.AutoMigrate", true),
			"provider": map[string]interface{}{
				"name": config.Get("AIAgent.Provider.Name", "mock"),
			},
			"storage": map[string]interface{}{
				"driver":      config.Get("AIAgent.Storage.Driver", "local"),
				"local_path":  config.Get("AIAgent.Storage.LocalPath", "public/artifacts"),
				"public_path": config.Get("AIAgent.Storage.PublicPath", "/artifacts"),
			},
		}
	})
}
