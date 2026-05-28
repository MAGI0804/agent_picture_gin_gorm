import { apiFetch } from '../api'
import type { AgentRun, AgentV2RunResponse, ModelSelection } from '../types'

export interface CreateAgentRunInput {
  conversationId: number
  content: string
  textModelConfigId: number
  imageModelConfigId: number
  candidateCount: number
  disableClarification: boolean
}

export function fetchModelSelection() {
  return apiFetch<ModelSelection>('/api/settings/model-selection')
}

export function createAgentRun(input: CreateAgentRunInput) {
  return apiFetch<AgentV2RunResponse>(`/api/v2/conversations/${input.conversationId}/runs/async`, {
    method: 'POST',
    body: JSON.stringify({
      content: input.content,
      task_type: 'image_generation',
      text_model_config_id: input.textModelConfigId,
      image_model_config_id: input.imageModelConfigId,
      candidate_count: input.candidateCount,
      disable_clarification: input.disableClarification,
      idempotency_key: `${input.conversationId}-${Date.now()}`
    })
  })
}

export function fetchAgentRun(runId: number) {
  return apiFetch<AgentV2RunResponse>(`/api/v2/runs/${runId}`)
}

export function resumeAgentRun(runId: number, content: string) {
  return apiFetch<AgentV2RunResponse>(`/api/v2/runs/${runId}/resume`, {
    method: 'POST',
    body: JSON.stringify({ content })
  })
}

export function cancelAgentRun(runId: number) {
  return apiFetch<{ agent_run: AgentRun; cancelled: boolean }>(`/api/v2/runs/${runId}/cancel`, {
    method: 'POST'
  })
}
