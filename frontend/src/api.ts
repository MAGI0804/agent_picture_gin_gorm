import type { ApiResponse } from './types'

const API_BASE = ''

export function getToken() {
  return localStorage.getItem('agent_token') || ''
}

export function setToken(token: string) {
  localStorage.setItem('agent_token', token)
}

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  const token = getToken()
  if (token) {
    headers.set('token', token)
  }

  const response = await fetch(API_BASE + path, { ...options, headers })
  const payload = (await response.json()) as ApiResponse<T>
  if (payload.code !== 0) {
    throw new Error(payload.msg || '请求失败')
  }
  return payload.data
}

export async function downloadArtifact(id: number, name: string) {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('token', token)
  }
  const response = await fetch(`/api/artifacts/${id}/download`, { headers })
  if (!response.ok) {
    throw new Error('下载失败')
  }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
