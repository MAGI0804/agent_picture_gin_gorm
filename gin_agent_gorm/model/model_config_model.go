package model

import "gorm.io/gorm"

// ModelConfig 全局模型配置，用于管理系统可用的 AI 模型。
type ModelConfig struct {
	BaseModel
	ModelName       string  `gorm:"column:model_name;size:128;not null" json:"model_name"`                                            // 模型名称
	RequestURL      string  `gorm:"column:request_url;size:512;not null" json:"request_url"`                                          // API 请求地址
	IsTextModel     bool    `gorm:"column:is_text_model;index:idx_model_configs_text;not null;default:false" json:"is_text_model"`    // 是否为文本模型
	IsImageModel    bool    `gorm:"column:is_image_model;index:idx_model_configs_image;not null;default:false" json:"is_image_model"` // 是否为图片模型
	SupportThinking bool    `gorm:"column:support_thinking;not null;default:false" json:"support_thinking"`                           // 是否支持思考模式
	ConfigInfo      JSONMap `gorm:"column:config_info;type:json" json:"config_info"`                                                  // 额外配置信息（JSON格式）
	CommonTimestampsField
}

// TableName 返回全局模型配置表名。
func (ModelConfig) TableName() string {
	return "model_configs"
}

// BeforeCreate 在创建模型配置前写入时间戳。
func (m *ModelConfig) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新模型配置前刷新时间戳。
func (m *ModelConfig) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}

// TextThinkingModelInput 文本思考模型输入参数。
// 用于调用支持思考功能的文本模型时传递的输入数据。
type TextThinkingModelInput struct {
	ModelName       string                `json:"model_name"`       // 模型名称
	SystemPrompt    string                `json:"system_prompt"`    // 系统提示词
	UserPrompt      string                `json:"user_prompt"`      // 用户输入提示词
	MaxTokens       int                   `json:"max_tokens"`       // 最大输出 token 数
	Temperature     float64               `json:"temperature"`      // 温度参数（0-1）
	TopP            float64               `json:"top_p"`            // Top P 参数
	ReturnThinking  bool                  `json:"return_thinking"`  // 是否返回思考过程
	Stream          bool                  `json:"stream"`           // 是否流式输出
	HistoryMessages []TextThinkingMessage `json:"history_messages"` // 历史消息列表
}

// TextThinkingMessage 文本思考模型的消息格式。
type TextThinkingMessage struct {
	Role    string `json:"role"`    // 角色（user/assistant/system）
	Content string `json:"content"` // 消息内容
}

// TextThinkingModelOutput 文本思考模型输出结果。
// 包含模型返回的文本响应和思考过程。
type TextThinkingModelOutput struct {
	ModelName       string     `json:"model_name"`       // 模型名称
	Content         string     `json:"content"`          // 主要响应内容
	ThinkingContent string     `json:"thinking_content"` // 思考过程内容
	FinishReason    string     `json:"finish_reason"`    // 结束原因（complete/stop/error）
	Usage           TokenUsage `json:"usage"`            // Token 使用统计
	IsStreaming     bool       `json:"is_streaming"`     // 是否为流式输出
}

// TokenUsage Token 使用统计信息。
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`     // 输入 token 数
	CompletionTokens int `json:"completion_tokens"` // 输出 token 数
	TotalTokens      int `json:"total_tokens"`      // 总 token 数
}

// ImageModelInput 图片生成模型输入参数。
// 用于调用图片生成模型时传递的输入数据。
type ImageModelInput struct {
	ModelName      string  `json:"model_name"`      // 模型名称
	Prompt         string  `json:"prompt"`          // 图片生成提示词
	NegativePrompt string  `json:"negative_prompt"` // 负面提示词（可选）
	Width          int     `json:"width"`           // 图片宽度
	Height         int     `json:"height"`          // 图片高度
	Steps          int     `json:"steps"`           // 生成步数
	GuidanceScale  float64 `json:"guidance_scale"`  // 引导比例
	Seed           int64   `json:"seed"`            // 随机种子（可选）
	StylePreset    string  `json:"style_preset"`    // 风格预设（可选）
	ReturnThinking bool    `json:"return_thinking"` // 是否返回思考过程
}

// ImageModelOutput 图片生成模型输出结果。
// 包含生成的图片数据和相关元信息。
type ImageModelOutput struct {
	ModelName       string `json:"model_name"`       // 模型名称
	ImageURL        string `json:"image_url"`        // 生成图片的预览 URL
	ImageBase64     string `json:"image_base64"`     // 图片 Base64 编码（可选）
	ImageWidth      int    `json:"image_width"`      // 图片宽度
	ImageHeight     int    `json:"image_height"`     // 图片高度
	ThinkingContent string `json:"thinking_content"` // 思考过程内容（如果启用）
	Seed            int64  `json:"seed"`             // 使用的随机种子
	FinishReason    string `json:"finish_reason"`    // 结束原因
}

// ModelConfigListResponse 模型配置列表响应结构。
type ModelConfigListResponse struct {
	Configs []ModelConfig `json:"configs"` // 模型配置列表
	Total   int64         `json:"total"`   // 总数
}

// ModelConfigDetailResponse 模型配置详情响应结构。
type ModelConfigDetailResponse struct {
	Config ModelConfig `json:"config"` // 模型配置详情
}
