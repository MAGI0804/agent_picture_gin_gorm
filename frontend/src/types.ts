export interface ApiResponse<T> {
  code: number
  msg: string
  data: T
}

export interface UserProfile {
  id: number
  account: string
  email: string
  phone?: string
  nickname: string
  avatar?: string
  introduction?: string
  created_at?: number
  updated_at?: number
}

export interface Conversation {
  id: number
  user_id: number
  title: string
  status: string
  created_at: number
  updated_at: number
}

export interface Message {
  id: number
  conversation_id: number
  user_id: number
  role: 'user' | 'assistant' | 'system'
  input_type: string
  content: string
  is_optimized?: boolean
  original_prompt?: string
  optimized_prompt?: string
  thinking_content?: string
  agent_run_id: number
  created_at: number
  updated_at: number
}

export interface FollowUpQuestion {
  id: number
  conversation_id: number
  message_id: number
  user_id: number
  question: string
  answer: string
  status: string
}

export interface Artifact {
  id: number
  conversation_id: number
  user_id: number
  agent_run_id: number
  name: string
  kind: 'image' | 'html' | string
  mime_type: string
  object_key: string
  preview_url: string
  size_bytes: number
  hash: string
}

export interface GlobalModelConfig {
  id: number
  model_name: string
  request_url: string
  is_text_model: boolean
  is_image_model: boolean
  support_thinking: boolean
  config_info?: Record<string, unknown>
  created_at?: number
  updated_at?: number
}

export interface ModelSelection {
  text_models: GlobalModelConfig[]
  image_models: GlobalModelConfig[]
  text_model_config_id: number
  image_model_config_id: number
}

export interface AgentRun {
  id: number
  conversation_id: number
  user_id: number
  trigger_message_id: number
  status: string
  intent: string
  task_type?: string
  text_model_name?: string
  image_model_name?: string
  is_optimized?: boolean
  optimized_prompt?: string
  error_message: string
}

export interface AgentStep {
  id: number
  agent_run_id: number
  name: string
  status: string
  input: string
  output: string
  think_content: string
  reasoning_content: string
  error_message: string
}

export interface ModelConfig {
  selected_text_model_config_id?: number
  selected_image_model_config_id?: number
  provider: string
  chat_model: string
  image_model: string
  base_url: string
  api_key: string
  temperature: string
  anthropic_auth_token: string
  anthropic_base_url: string
  anthropic_model: string
  anthropic_default_opus_model: string
  anthropic_default_sonnet_model: string
  anthropic_default_haiku_model: string
  claude_code_subagent_model: string
  claude_code_max_output_tokens: string
}
