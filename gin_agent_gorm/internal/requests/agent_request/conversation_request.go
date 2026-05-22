package agent_request

// CreateConversationRequest 是创建会话的请求参数。
type CreateConversationRequest struct {
	Title string `json:"title" form:"title"`
}

// SendMessageRequest 是提交正常对话或补充问题回答的请求参数。
type SendMessageRequest struct {
	InputType           string `json:"input_type" form:"input_type"`
	TaskType            string `json:"task_type" form:"task_type"`
	Content             string `json:"content" form:"content"`
	TextModelConfigID   uint   `json:"text_model_config_id" form:"text_model_config_id"`
	ImageModelConfigID  uint   `json:"image_model_config_id" form:"image_model_config_id"`
	QuestionMode        string `json:"question_mode" form:"question_mode"`
	OriginalPrompt      string `json:"original_prompt" form:"original_prompt"`
	IsOptimized         bool   `json:"is_optimized" form:"is_optimized"`
	OptimizedPrompt     string `json:"optimized_prompt" form:"optimized_prompt"`
	AnsweredQuestionIDs []uint `json:"answered_question_ids" form:"answered_question_ids"`
	Attachments         []uint `json:"attachments" form:"attachments"`
	Stream              bool   `json:"stream" form:"stream"`
	ReturnReasoning     bool   `json:"return_reasoning" form:"return_reasoning"`
}

// OptimizePromptRequest 是智能优化提示词的请求参数。
type OptimizePromptRequest struct {
	Content      string `json:"content" form:"content"`
	TargetLength int    `json:"target_length" form:"target_length"`
}

// LoginRequest 是前端工作台登录接口的请求参数。
type LoginRequest struct {
	Account  string `json:"account" form:"account"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}
