package model

import "gorm.io/gorm"

// UserModelConfig 表示用户绑定的模型配置。
type UserModelConfig struct {
	BaseModel
	UserID                     uint   `gorm:"column:user_id;uniqueIndex;not null" json:"user_id"`                                // 所属用户 ID。
	SelectedTextModelConfigID  uint   `gorm:"column:selected_text_model_config_id;index" json:"selected_text_model_config_id"`   // 用户选择的全局文本模型配置 ID。
	SelectedImageModelConfigID uint   `gorm:"column:selected_image_model_config_id;index" json:"selected_image_model_config_id"` // 用户选择的全局图片模型配置 ID。
	Provider                   string `gorm:"column:provider;size:64;not null;default:deepseek" json:"provider"`                 // 模型供应商，例如 deepseek、openai。
	ChatModel                  string `gorm:"column:chat_model;size:128;not null" json:"chat_model"`                             // 对话模型名称。
	ImageModel                 string `gorm:"column:image_model;size:128" json:"image_model"`                                    // 图片模型名称。
	BaseURL                    string `gorm:"column:base_url;size:255" json:"base_url"`                                          // 模型 API 地址。
	APIKey                     string `gorm:"column:api_key;type:text" json:"api_key"`                                           // 模型 API Key。
	Temperature                string `gorm:"column:temperature;size:32;not null;default:0.7" json:"temperature"`                // 模型温度参数。

	AnthropicAuthToken          string `gorm:"column:anthropic_auth_token;type:text" json:"anthropic_auth_token"`                    // Anthropic 兼容鉴权 token。
	AnthropicBaseURL            string `gorm:"column:anthropic_base_url;size:255" json:"anthropic_base_url"`                         // Anthropic 兼容 API 地址。
	AnthropicModel              string `gorm:"column:anthropic_model;size:128" json:"anthropic_model"`                               // Anthropic 兼容默认模型。
	AnthropicDefaultOpusModel   string `gorm:"column:anthropic_default_opus_model;size:128" json:"anthropic_default_opus_model"`     // Opus 档位模型。
	AnthropicDefaultSonnetModel string `gorm:"column:anthropic_default_sonnet_model;size:128" json:"anthropic_default_sonnet_model"` // Sonnet 档位模型。
	AnthropicDefaultHaikuModel  string `gorm:"column:anthropic_default_haiku_model;size:128" json:"anthropic_default_haiku_model"`   // Haiku 档位模型。
	ClaudeCodeSubagentModel     string `gorm:"column:claude_code_subagent_model;size:128" json:"claude_code_subagent_model"`         // 子 Agent 使用模型。
	ClaudeCodeMaxOutputTokens   string `gorm:"column:claude_code_max_output_tokens;size:32" json:"claude_code_max_output_tokens"`    // 最大输出 token。

	CommonTimestampsField
}

// TableName 返回用户模型配置表名。
func (UserModelConfig) TableName() string {
	return "user_model_configs"
}

// BeforeCreate 在创建模型配置前写入时间戳。
func (m *UserModelConfig) BeforeCreate(tx *gorm.DB) error {
	setCreateTimestamps(&m.CommonTimestampsField)
	return nil
}

// BeforeUpdate 在更新模型配置前刷新更新时间。
func (m *UserModelConfig) BeforeUpdate(tx *gorm.DB) error {
	setUpdateTimestamp(&m.CommonTimestampsField)
	return nil
}
