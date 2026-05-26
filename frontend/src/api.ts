import type { ApiResponse, UserProfile } from './types'

const API_BASE = ''

export function getToken() {
  return localStorage.getItem('agent_token') || ''
}

export function setToken(token: string) {
  if (!token) {
    localStorage.removeItem('agent_token')
    localStorage.removeItem('agent_user')
    return
  }
  localStorage.setItem('agent_token', token)
}

export function getCurrentUser() {
  const raw = localStorage.getItem('agent_user')
  if (!raw) return null
  try {
    return JSON.parse(raw) as UserProfile
  } catch {
    localStorage.removeItem('agent_user')
    return null
  }
}

export function setCurrentUser(user: UserProfile | null) {
  if (!user) {
    localStorage.removeItem('agent_user')
    return
  }
  localStorage.setItem('agent_user', JSON.stringify(user))
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

  let response: Response
  try {
    response = await fetch(API_BASE + path, { ...options, headers })
  } catch {
    throw new Error('无法连接服务器，请确认后端服务已启动')
  }

  let payload: ApiResponse<T>
  try {
    payload = (await response.json()) as ApiResponse<T>
  } catch {
    throw new Error(response.ok ? '服务器返回格式异常' : `请求失败（HTTP ${response.status}）`)
  }
  if (isAuthExpired(payload)) {
    redirectToLogin()
    throw new Error(payload.msg || '登录已过期，请重新登录')
  }
  if (!isSuccess(payload.code)) {
    throw new Error(payload.msg || '请求失败')
  }
  return payload.data
}

function isSuccess(code: number) {
  return code === 200 || code === 0
}

function isAuthExpired<T>(payload: ApiResponse<T>) {
  const message = payload.msg || ''
  return payload.code === 100401 ||
    message.includes('令牌已过期') ||
    message.includes('请求令牌无效') ||
    message.includes('无法找到令牌')
}

function redirectToLogin() {
  setToken('')
  if (window.location.pathname === '/login') return
  const redirect = encodeURIComponent(window.location.pathname + window.location.search)
  window.location.href = `/login?redirect=${redirect}`
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

export async function downloadV2Artifact(id: number, name: string) {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('token', token)
  }
  const response = await fetch(`/api/v2/artifacts/${id}/download`, { headers })
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

export async function fetchV2ArtifactPreviewURL(id: number) {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('token', token)
  }
  const response = await fetch(`/api/v2/artifacts/${id}/preview`, { headers })
  if (!response.ok) {
    throw new Error('预览加载失败')
  }
  const blob = await response.blob()
  return URL.createObjectURL(blob)
}

export interface OptimizePromptResult {
  original_prompt: string
  optimized_prompt: string
  target_length: number
  original_length: number
  optimized_length: number
}

export async function optimizePrompt(content: string, targetLength: number = 700) {
  return apiFetch<OptimizePromptResult>('/api/prompts/optimize', {
    method: 'POST',
    body: JSON.stringify({
      content,
      target_length: targetLength,
    }),
  })
}
