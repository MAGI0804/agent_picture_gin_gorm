package agent_svc

import (
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// GetModelConfig 查询当前用户绑定的模型配置，不存在时返回默认配置。
func (svc *AgentService) GetModelConfig(userID uint) (model.UserModelConfig, error) {
	config := model.UserModelConfig{
		UserID:      userID,
		Temperature: "0.7",
	}
	return config, nil
}

// GetModelSelection returns the global models the user is permitted to use.
func (svc *AgentService) GetModelSelection(userID uint) (map[string]interface{}, error) {
	textModels, err := svc.ListUserTextModels(userID)
	if err != nil {
		return nil, err
	}
	imageModels, err := svc.ListUserImageModels(userID)
	if err != nil {
		return nil, err
	}

	var textModelID uint
	if len(textModels) > 0 {
		textModelID = textModels[0].ID
	}
	var imageModelID uint
	if len(imageModels) > 0 {
		imageModelID = imageModels[0].ID
	}

	return map[string]interface{}{
		"text_models":           textModels,
		"image_models":          imageModels,
		"text_model_config_id":  textModelID,
		"image_model_config_id": imageModelID,
	}, nil
}

// SaveModelSelection validates the selected model IDs against user permissions.
// The selection is request-scoped on the frontend and is no longer persisted.
func (svc *AgentService) SaveModelSelection(
	userID uint,
	request agent_request.SaveModelSelectionRequest,
) (map[string]interface{}, error) {
	if request.TextModelConfigID != 0 {
		config, err := svc.dao.FindPermittedModelConfig(userID, request.TextModelConfigID)
		if err != nil {
			return nil, errors.Wrap(err, "text model config is not permitted")
		}
		if !config.IsTextModel {
			return nil, errors.New("selected text model is not a text model")
		}
	}
	if request.ImageModelConfigID != 0 {
		config, err := svc.dao.FindPermittedModelConfig(userID, request.ImageModelConfigID)
		if err != nil {
			return nil, errors.Wrap(err, "image model config is not permitted")
		}
		if !config.IsImageModel {
			return nil, errors.New("selected image model is not an image model")
		}
	}
	return svc.GetModelSelection(userID)
}

// SaveModelConfig 保存当前用户绑定的模型配置。
func (svc *AgentService) SaveModelConfig(userID uint, request agent_request.SaveModelConfigRequest) (model.UserModelConfig, error) {
	config := model.UserModelConfig{
		UserID:      userID,
		Provider:    strings.TrimSpace(request.Provider),
		ChatModel:   strings.TrimSpace(request.ChatModel),
		ImageModel:  strings.TrimSpace(request.ImageModel),
		BaseURL:     strings.TrimSpace(request.BaseURL),
		APIKey:      strings.TrimSpace(request.APIKey),
		Temperature: strings.TrimSpace(request.Temperature),

		AnthropicAuthToken:          strings.TrimSpace(request.AnthropicAuthToken),
		AnthropicBaseURL:            strings.TrimSpace(request.AnthropicBaseURL),
		AnthropicModel:              strings.TrimSpace(request.AnthropicModel),
		AnthropicDefaultOpusModel:   strings.TrimSpace(request.AnthropicDefaultOpusModel),
		AnthropicDefaultSonnetModel: strings.TrimSpace(request.AnthropicDefaultSonnetModel),
		AnthropicDefaultHaikuModel:  strings.TrimSpace(request.AnthropicDefaultHaikuModel),
		ClaudeCodeSubagentModel:     strings.TrimSpace(request.ClaudeCodeSubagentModel),
		ClaudeCodeMaxOutputTokens:   strings.TrimSpace(request.ClaudeCodeMaxOutputTokens),
	}
	if config.Provider == "" {
		config.Provider = "deepseek-anthropic"
	}
	if config.ChatModel == "" {
		config.ChatModel = "deepseek-v4-pro"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com/anthropic"
	}
	if config.Temperature == "" {
		config.Temperature = "0.7"
	}
	if config.AnthropicAuthToken == "" {
		config.AnthropicAuthToken = config.APIKey
	}
	if config.APIKey == "" {
		config.APIKey = config.AnthropicAuthToken
	}
	if config.AnthropicBaseURL == "" {
		config.AnthropicBaseURL = config.BaseURL
	}
	if config.AnthropicModel == "" {
		config.AnthropicModel = config.ChatModel
	}
	if config.AnthropicDefaultOpusModel == "" {
		config.AnthropicDefaultOpusModel = config.AnthropicModel
	}
	if config.AnthropicDefaultSonnetModel == "" {
		config.AnthropicDefaultSonnetModel = config.AnthropicModel
	}
	if config.AnthropicDefaultHaikuModel == "" {
		config.AnthropicDefaultHaikuModel = config.AnthropicModel
	}
	if config.ClaudeCodeSubagentModel == "" {
		config.ClaudeCodeSubagentModel = config.AnthropicModel
	}
	if config.ClaudeCodeMaxOutputTokens == "" {
		config.ClaudeCodeMaxOutputTokens = "32000"
	}

	if err := svc.dao.SaveUserModelConfig(&config); err != nil {
		return config, err
	}
	delete(svc.userConfigCache, userID)
	return svc.GetModelConfig(userID)
}

// ListModelConfigs 获取全局模型配置列表。
func (svc *AgentService) ListModelConfigs(page, perPage int, isTextModel, isImageModel string) ([]model.ModelConfig, int64, error) {
	var configs []model.ModelConfig
	query := database.DB.Model(&model.ModelConfig{})

	if isTextModel != "" {
		query = query.Where("is_text_model = ?", isTextModel == "true")
	}
	if isImageModel != "" {
		query = query.Where("is_image_model = ?", isImageModel == "true")
	}

	var total int64
	query.Count(&total)

	offset := (page - 1) * perPage
	err := query.Order("id desc").Offset(offset).Limit(perPage).Find(&configs).Error
	return configs, total, err
}

// GetModelConfigByID 根据 ID 获取模型配置。
func (svc *AgentService) GetModelConfigByID(id uint) (model.ModelConfig, error) {
	if config, ok := svc.globalConfigCache[id]; ok {
		return config, nil
	}
	var config model.ModelConfig
	err := database.DB.Where("id = ?", id).First(&config).Error
	if err == nil {
		svc.globalConfigCache[id] = config
	}
	return config, err
}

// CreateModelConfig 创建新的模型配置。
func (svc *AgentService) CreateModelConfig(request struct {
	ModelName       string                 `json:"model_name" form:"model_name"`
	RequestURL      string                 `json:"request_url" form:"request_url"`
	IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
	IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
	SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
	ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
}) (model.ModelConfig, error) {
	config := model.ModelConfig{
		ModelName:       request.ModelName,
		RequestURL:      request.RequestURL,
		IsTextModel:     request.IsTextModel,
		IsImageModel:    request.IsImageModel,
		SupportThinking: request.SupportThinking,
	}

	if request.ConfigInfo != nil {
		config.ConfigInfo = model.JSONMap(request.ConfigInfo)
	}

	err := database.DB.Create(&config).Error
	return config, err
}

// UpdateModelConfig 更新模型配置。
func (svc *AgentService) UpdateModelConfig(id uint, request struct {
	ModelName       string                 `json:"model_name" form:"model_name"`
	RequestURL      string                 `json:"request_url" form:"request_url"`
	IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
	IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
	SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
	ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
}) (model.ModelConfig, error) {
	var config model.ModelConfig
	err := database.DB.Where("id = ?", id).First(&config).Error
	if err != nil {
		return config, err
	}

	if request.ModelName != "" {
		config.ModelName = request.ModelName
	}
	if request.RequestURL != "" {
		config.RequestURL = request.RequestURL
	}
	config.IsTextModel = request.IsTextModel
	config.IsImageModel = request.IsImageModel
	config.SupportThinking = request.SupportThinking

	if request.ConfigInfo != nil {
		config.ConfigInfo = model.JSONMap(request.ConfigInfo)
	}

	err = database.DB.Save(&config).Error
	return config, err
}

// DeleteModelConfig 删除模型配置。
func (svc *AgentService) DeleteModelConfig(id uint) error {
	return database.DB.Delete(&model.ModelConfig{}, id).Error
}

// ListTextModels 获取所有文本模型配置。
func (svc *AgentService) ListTextModels() ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	err := database.DB.Where("is_text_model = ?", true).Order("id desc").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return filterRealModelConfigs(configs), nil
}

// ListImageModels 获取所有图片模型配置。
func (svc *AgentService) ListImageModels() ([]model.ModelConfig, error) {
	var configs []model.ModelConfig
	err := database.DB.Where("is_image_model = ?", true).Order("id desc").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return filterRealModelConfigs(configs), nil
}

func (svc *AgentService) ListUserTextModels(userID uint) ([]model.ModelConfig, error) {
	configs, err := svc.dao.ListPermittedModelConfigs(userID, true, false)
	if err != nil {
		return nil, err
	}
	return filterRealModelConfigs(configs), nil
}

func (svc *AgentService) ListUserImageModels(userID uint) ([]model.ModelConfig, error) {
	configs, err := svc.dao.ListPermittedModelConfigs(userID, false, true)
	if err != nil {
		return nil, err
	}
	return filterRealModelConfigs(configs), nil
}

func containsModelConfigID(configs []model.ModelConfig, id uint) bool {
	if id == 0 {
		return false
	}
	for _, config := range configs {
		if config.ID == id {
			return true
		}
	}
	return false
}

func filterRealModelConfigs(configs []model.ModelConfig) []model.ModelConfig {
	filtered := make([]model.ModelConfig, 0, len(configs))
	for _, config := range configs {
		if isMockModelConfig(config) {
			continue
		}
		filtered = append(filtered, config)
	}
	return filtered
}

func isMockModelConfig(config model.ModelConfig) bool {
	provider := strings.ToLower(configInfoFirstString(config.ConfigInfo, "provider", "vendor", "api_type", "type", "provider name", "api type"))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	requestURL := strings.TrimSpace(config.RequestURL)

	// 如果有 provider 且不是 mock，则不是 mock 模型
	if provider != "" && provider != "mock" {
		return false
	}

	// 如果有 requestURL 且不是 mock，则不是 mock 模型
	if requestURL != "" && !strings.Contains(requestURL, "mock") {
		return false
	}

	// 如果 modelName 不以 mock- 开头，则不是 mock 模型
	if modelName != "" && !strings.HasPrefix(modelName, "mock-") {
		return false
	}

	// 如果 provider 是 mock 或者 modelName 以 mock- 开头，则是 mock 模型
	return provider == "mock" || strings.HasPrefix(modelName, "mock-")
}

// IsNotFound 判断错误是否为 GORM 记录不存在。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
