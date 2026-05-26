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
  display_model_name?: string
  display_dialog_type?: string
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
  artifact_group_id?: string
  rank_score?: number
  selected_at?: number
}

export interface ArtifactVersion {
  id: number
  artifact_id: number
  parent_version_id: number
  agent_run_id: number
  version_no: number
  operation: string
  prompt: string
  negative_prompt: string
  model_provider: string
  model_name: string
  generation_params: string
  source_refs: string
  quality_scores: string
  object_key: string
  preview_url: string
  hash: string
  created_at: number
  updated_at: number
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
  step_key?: string
  attempt?: number
  duration_ms?: number
  input_hash?: string
  output_hash?: string
  output_json?: string
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

export interface AgentV2RunResponse {
  conversation: Conversation
  user_message?: Message
  assistant_message?: Message
  agent_run: AgentRun
  steps: AgentStep[]
  artifacts?: Artifact[]
  state?: Record<string, unknown>
  idempotent?: boolean
}
