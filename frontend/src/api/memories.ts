import { apiFetch } from '../api'
import type { ContextMemory } from '../types'

export interface MemoryQuery {
  conversationId: number
  namespace?: string
  limit?: number
}

export function listMemories(query: MemoryQuery) {
  const params = new URLSearchParams({
    conversation_id: String(query.conversationId),
    limit: String(query.limit || 20)
  })
  if (query.namespace) {
    params.set('namespace', query.namespace)
  }
  return apiFetch<{ memories: ContextMemory[] }>(`/api/v2/memories?${params.toString()}`)
}

export function promoteMemoryProposal(id: number, confidence = 0.85) {
  return apiFetch<{ memory: ContextMemory; promoted: boolean }>(`/api/v2/memories/${id}/promote`, {
    method: 'POST',
    body: JSON.stringify({ confidence })
  })
}

export function updateMemoryContent(id: number, content: string) {
  return apiFetch<{ memory: ContextMemory }>(`/api/v2/memories/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ content })
  })
}

export function deleteMemoryById(id: number) {
  return apiFetch<{ deleted: boolean }>(`/api/v2/memories/${id}`, { method: 'DELETE' })
}
