package agent_request

// CreateConversationRequest 是创建会话的请求参数。
type CreateConversationRequest struct {
	Title string `json:"title" form:"title"`
}

// SendMessageRequest 是提交正常对话或补充问题回答的请求参数。
type SendMessageRequest struct {
	InputType           string                 `json:"input_type" form:"input_type"`
	TaskType            string                 `json:"task_type" form:"task_type"`
	Content             string                 `json:"content" form:"content"`
	AnsweredQuestionIDs []uint                 `json:"answered_question_ids" form:"answered_question_ids"`
	Attachments         []uint                 `json:"attachments" form:"attachments"`
	ModelConfig         map[string]interface{} `json:"model_config" form:"model_config"`
	Stream              bool                   `json:"stream" form:"stream"`
	ReturnReasoning     bool                   `json:"return_reasoning" form:"return_reasoning"`
}

// LoginRequest 是前端工作台登录接口的请求参数。
type LoginRequest struct {
	Account  string `json:"account" form:"account"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}
