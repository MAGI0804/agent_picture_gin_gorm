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
				"driver":         config.Get("AIAgent.Storage.Driver", "local"),
				"local_path":     config.Get("AIAgent.Storage.LocalPath", "public/artifacts"),
				"public_path":    config.Get("AIAgent.Storage.PublicPath", "/artifacts"),
				"static_enabled": config.Get("AIAgent.Storage.StaticEnabled", false),
			},
			"safety": map[string]interface{}{
				"enabled":       config.Get("AIAgent.Safety.Enabled", true),
				"fail_closed":   config.Get("AIAgent.Safety.FailClosed", true),
				"blocked_terms": config.Get("AIAgent.Safety.BlockedTerms", []string{}),
			},
			"proxy": map[string]interface{}{
				"enabled": config.Get("AIAgent.Proxy.Enabled", false),
				"http":    config.Get("AIAgent.Proxy.HTTP", "http://127.0.0.1:22307"),
				"https":   config.Get("AIAgent.Proxy.HTTPS", "http://127.0.0.1:22307"),
			},
		}
	})
}
