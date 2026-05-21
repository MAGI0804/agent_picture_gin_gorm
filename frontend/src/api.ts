import type { ApiResponse } from './types'

const API_BASE = ''

export function getToken() {
  return localStorage.getItem('agent_token') || ''
}

export function setToken(token: string) {
  if (!token) {
    localStorage.removeItem('agent_token')
    return
  }
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
  if (isAuthExpired(payload)) {
    redirectToLogin()
    throw new Error(payload.msg || '登录已过期，请重新登录')
  }
  if (payload.code !== 0) {
    throw new Error(payload.msg || '请求失败')
  }
  return payload.data
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
