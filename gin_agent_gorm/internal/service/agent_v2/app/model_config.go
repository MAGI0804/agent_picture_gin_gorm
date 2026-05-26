package app

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gin-biz-web-api/model"
)

type resolvedModelConfig struct {
	Config   model.UserModelConfig
	GlobalID uint
	Global   model.ModelConfig
}

func (svc *Service) resolveRuntimeModelConfig(
	userID uint,
	modelKind string,
	selectedModelConfigID uint,
) (resolvedModelConfig, error) {
	selected, err := svc.permittedGlobalModelConfig(userID, modelKind, selectedModelConfigID)
	if err != nil {
		return resolvedModelConfig{}, err
	}
	config := mergeUserConfigWithGlobalModel(model.UserModelConfig{
		UserID:      userID,
		Temperature: "0.7",
	}, selected, modelKind)
	if strings.TrimSpace(config.Provider) == "" || strings.EqualFold(config.Provider, "mock") {
		return resolvedModelConfig{}, errors.Errorf("所选%s模型不是有效真实模型，请检查全局模型配置", modelKindLabel(modelKind))
	}
	if strings.TrimSpace(config.Temperature) == "" {
		config.Temperature = "0.7"
	}
	if strings.TrimSpace(config.AnthropicAuthToken) == "" {
		config.AnthropicAuthToken = strings.TrimSpace(config.APIKey)
	}
	if strings.TrimSpace(config.APIKey) == "" {
		config.APIKey = strings.TrimSpace(config.AnthropicAuthToken)
	}
	if strings.TrimSpace(config.AnthropicBaseURL) == "" {
		config.AnthropicBaseURL = strings.TrimSpace(config.BaseURL)
	}
	if strings.TrimSpace(config.AnthropicModel) == "" {
		config.AnthropicModel = strings.TrimSpace(config.ChatModel)
	}
	return resolvedModelConfig{
		Config:   config,
		GlobalID: selected.ID,
		Global:   selected,
	}, nil
}

func (svc *Service) resolveVisionRuntimeModelConfig(userID uint) (resolvedModelConfig, error) {
	configs, err := svc.dao.ListPermittedModelConfigs(userID, true, false)
	if err != nil {
		return resolvedModelConfig{}, err
	}
	for _, config := range configs {
		if isMockModelConfig(config) || modelConfigCapability(config) != "vision" {
			continue
		}
		runtimeConfig := mergeUserConfigWithGlobalModel(model.UserModelConfig{
			UserID:      userID,
			Temperature: "0.2",
		}, config, "text")
		if strings.TrimSpace(runtimeConfig.Provider) == "" ||
			strings.EqualFold(runtimeConfig.Provider, "mock") ||
			strings.TrimSpace(runtimeConfig.BaseURL) == "" ||
			strings.TrimSpace(runtimeConfig.APIKey) == "" ||
			strings.TrimSpace(runtimeConfig.ChatModel) == "" {
			return resolvedModelConfig{}, errors.New("真实视觉模型配置不完整")
		}
		return resolvedModelConfig{
			Config:   runtimeConfig,
			GlobalID: config.ID,
			Global:   config,
		}, nil
	}
	return resolvedModelConfig{}, errors.New("未配置真实视觉模型")
}

func (svc *Service) permittedGlobalModelConfig(
	userID uint,
	modelKind string,
	selectedID uint,
) (model.ModelConfig, error) {
	if selectedID != 0 {
		config, err := svc.dao.FindPermittedModelConfig(userID, selectedID)
		if err != nil {
			return model.ModelConfig{}, errors.Wrapf(err, "未选择真实%s模型，请先在输入框或设置页选择模型", modelKindLabel(modelKind))
		}
		if isMockModelConfig(config) || !modelConfigMatchesKind(config, modelKind) {
			return model.ModelConfig{}, errors.Errorf("所选%s模型不是有效真实模型，请检查全局模型配置", modelKindLabel(modelKind))
		}
		return config, nil
	}
	var configs []model.ModelConfig
	var err error
	if modelKind == "image" {
		configs, err = svc.dao.ListPermittedModelConfigs(userID, false, true)
	} else {
		configs, err = svc.dao.ListPermittedModelConfigs(userID, true, false)
	}
	if err != nil {
		return model.ModelConfig{}, err
	}
	for _, config := range configs {
		if isMockModelConfig(config) {
			continue
		}
		if modelKind != "image" && isPreferredTextModel(config) {
			return config, nil
		}
	}
	for _, config := range configs {
		if !isMockModelConfig(config) {
			return config, nil
		}
	}
	return model.ModelConfig{}, errors.Errorf("未选择真实%s模型，请先在输入框或设置页选择模型", modelKindLabel(modelKind))
}

func modelConfigMatchesKind(config model.ModelConfig, modelKind string) bool {
	if modelKind == "image" {
		return config.IsImageModel
	}
	return config.IsTextModel
}

func isPreferredTextModel(config model.ModelConfig) bool {
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	return strings.Contains(modelName, "deepseek-v4-pro") || strings.Contains(modelName, "deepseek_v4_pro")
}

func modelConfigCapability(config model.ModelConfig) string {
	return strings.ToLower(configInfoFirstString(config.ConfigInfo, "capability", "model_capability", "kind"))
}

func modelKindLabel(modelKind string) string {
	if modelKind == "image" {
		return "图片"
	}
	return "文本"
}

func mergeUserConfigWithGlobalModel(
	userConfig model.UserModelConfig,
	globalConfig model.ModelConfig,
	modelKind string,
) model.UserModelConfig {
	config := userConfig
	config.RuntimeConfig = globalConfig.ConfigInfo
	provider := modelConfigProvider(globalConfig)
	baseURL := configInfoFirstString(globalConfig.ConfigInfo, "base_url", "baseURL", "url", "endpoint", "base url")
	if baseURL == "" {
		baseURL = globalConfig.RequestURL
	}
	apiKey := configInfoFirstString(globalConfig.ConfigInfo,
		"api_key", "apiKey", "key", "token", "access_token", "auth_token", "anthropic_auth_token",
		"api key", "access key", "secret access key", "secret key")
	temperature := configInfoFirstString(globalConfig.ConfigInfo, "temperature", "temp")
	if temperature == "" {
		temperature = config.Temperature
	}
	if temperature == "" {
		temperature = "0.7"
	}

	config.Provider = provider
	config.BaseURL = baseURL
	config.APIKey = apiKey
	config.Temperature = temperature
	if modelKind == "image" {
		config.ImageModel = globalConfig.ModelName
		config.SelectedImageModelConfigID = globalConfig.ID
	} else {
		config.ChatModel = globalConfig.ModelName
		config.SelectedTextModelConfigID = globalConfig.ID
		config.AnthropicModel = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_model", "anthropicModel", "anthropic model")
		if config.AnthropicModel == "" {
			config.AnthropicModel = globalConfig.ModelName
		}
	}

	config.AnthropicAuthToken = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_auth_token", "anthropicAuthToken", "anthropic auth token")
	if config.AnthropicAuthToken == "" {
		config.AnthropicAuthToken = apiKey
	}
	config.AnthropicBaseURL = configInfoFirstString(globalConfig.ConfigInfo, "anthropic_base_url", "anthropicBaseURL", "anthropic base url")
	if config.AnthropicBaseURL == "" {
		config.AnthropicBaseURL = baseURL
	}
	config.AnthropicDefaultOpusModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_opus_model")
	config.AnthropicDefaultSonnetModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_sonnet_model")
	config.AnthropicDefaultHaikuModel = configInfoString(globalConfig.ConfigInfo, "anthropic_default_haiku_model")
	config.ClaudeCodeSubagentModel = configInfoString(globalConfig.ConfigInfo, "claude_code_subagent_model")
	config.ClaudeCodeMaxOutputTokens = configInfoString(globalConfig.ConfigInfo, "claude_code_max_output_tokens")
	if config.ClaudeCodeMaxOutputTokens == "" {
		config.ClaudeCodeMaxOutputTokens = "4096"
	}
	return config
}

func modelConfigProvider(config model.ModelConfig) string {
	provider := strings.ToLower(configInfoFirstString(config.ConfigInfo, "provider", "vendor", "api_type", "type", "provider name", "api type"))
	if provider != "" {
		return provider
	}
	source := strings.ToLower(strings.TrimSpace(config.RequestURL) + " " + strings.TrimSpace(config.ModelName))
	switch {
	case strings.Contains(source, "deepseek"):
		return "deepseek"
	case strings.Contains(source, "anthropic") || strings.Contains(source, "claude"):
		return "anthropic"
	case strings.Contains(source, "dashscope") || strings.Contains(source, "qwen") || strings.Contains(source, "aliyun"):
		return "dashscope"
	case strings.Contains(source, "jimeng") || strings.Contains(source, "doubao") || strings.Contains(source, "volc") || strings.Contains(source, "seedream"):
		return "jimeng"
	case strings.Contains(source, "openai") || strings.Contains(source, "gpt"):
		return "openai"
	case strings.TrimSpace(config.RequestURL) != "":
		return "openai-compatible"
	default:
		return ""
	}
}

func configInfoString(values model.JSONMap, key string) string {
	if len(values) == 0 {
		return ""
	}
	if value, ok := values[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	targetKeyLower := strings.ToLower(strings.ReplaceAll(key, " ", ""))
	for k, value := range values {
		if value == nil {
			continue
		}
		kLower := strings.ToLower(strings.ReplaceAll(k, " ", ""))
		if kLower == targetKeyLower {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func configInfoFirstString(values model.JSONMap, keys ...string) string {
	for _, key := range keys {
		if value := configInfoString(values, key); value != "" {
			return value
		}
	}
	return ""
}

func isMockModelConfig(config model.ModelConfig) bool {
	provider := strings.ToLower(configInfoFirstString(config.ConfigInfo, "provider", "vendor", "api_type", "type", "provider name", "api type"))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	requestURL := strings.TrimSpace(config.RequestURL)
	if provider != "" && provider != "mock" {
		return false
	}
	if requestURL != "" && !strings.Contains(requestURL, "mock") {
		return false
	}
	if modelName != "" && !strings.HasPrefix(modelName, "mock-") {
		return false
	}
	return provider == "mock" || strings.HasPrefix(modelName, "mock-")
}

func runtimeTextModelName(config model.UserModelConfig) string {
	if strings.TrimSpace(config.ChatModel) != "" {
		return strings.TrimSpace(config.ChatModel)
	}
	if strings.TrimSpace(config.AnthropicModel) != "" {
		return strings.TrimSpace(config.AnthropicModel)
	}
	return strings.TrimSpace(config.Provider)
}

func runtimeImageModelName(config model.UserModelConfig) string {
	if strings.TrimSpace(config.ImageModel) != "" {
		return strings.TrimSpace(config.ImageModel)
	}
	return strings.TrimSpace(config.Provider)
}
