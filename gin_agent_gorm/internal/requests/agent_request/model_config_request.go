package agent_request

// SaveModelConfigRequest saves the user model configuration.
type SaveModelConfigRequest struct {
	Provider                    string `json:"provider" form:"provider"`
	ChatModel                   string `json:"chat_model" form:"chat_model"`
	ImageModel                  string `json:"image_model" form:"image_model"`
	BaseURL                     string `json:"base_url" form:"base_url"`
	APIKey                      string `json:"api_key" form:"api_key"`
	Temperature                 string `json:"temperature" form:"temperature"`
	AnthropicAuthToken          string `json:"anthropic_auth_token" form:"anthropic_auth_token"`
	AnthropicBaseURL            string `json:"anthropic_base_url" form:"anthropic_base_url"`
	AnthropicModel              string `json:"anthropic_model" form:"anthropic_model"`
	AnthropicDefaultOpusModel   string `json:"anthropic_default_opus_model" form:"anthropic_default_opus_model"`
	AnthropicDefaultSonnetModel string `json:"anthropic_default_sonnet_model" form:"anthropic_default_sonnet_model"`
	AnthropicDefaultHaikuModel  string `json:"anthropic_default_haiku_model" form:"anthropic_default_haiku_model"`
	ClaudeCodeSubagentModel     string `json:"claude_code_subagent_model" form:"claude_code_subagent_model"`
	ClaudeCodeMaxOutputTokens   string `json:"claude_code_max_output_tokens" form:"claude_code_max_output_tokens"`
}

// GlobalSaveModelConfigRequest saves the global model configuration.
type GlobalSaveModelConfigRequest struct {
	ID              uint                   `json:"id" form:"id"`
	ModelName       string                 `json:"model_name" form:"model_name"`
	RequestURL      string                 `json:"request_url" form:"request_url"`
	IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
	IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
	SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
	ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
}
