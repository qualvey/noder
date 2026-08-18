// API 客户端：统一鉴权、错误处理、相对路径（兼容子路径反代）
import type { DistFile, Node, User } from './types'

const base = import.meta.env.BASE_URL
// dev 下 BASE_URL 为 '/'，build 后为 './'
export function apiBase(): string {
  return base.endsWith('/') ? base.slice(0, -1) : base
}

let adminToken = localStorage.getItem('admin_token') || 'admin-secret'

export function getAdminToken(): string {
  return adminToken
}

export function setAdminToken(t: string): void {
  adminToken = t
  localStorage.setItem('admin_token', t)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, options: Omit<RequestInit, 'body'> & { body?: unknown } = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...((options.headers as Record<string, string>) || {}),
  }
  headers['X-Admin-Token'] = adminToken
  if (options.body && typeof options.body === 'object' && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(options.body)
  }
  const { body: rawBody, ...rest } = options
  const init: RequestInit = { ...rest, headers }
  if (rawBody !== undefined) {
    init.body = rawBody as BodyInit
  }
  const res = await fetch(apiBase() + path, init)
  if (res.status === 401) throw new ApiError(401, '鉴权失败：Admin Token 无效')
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new ApiError(res.status, (data as { detail?: string }).detail || res.statusText)
  }
  return res.json() as Promise<T>
}

export const api = {
  nodes: {
    list: () => request<Node[]>('/api/nodes'),
    create: (body: Partial<Node>) => request<Node>('/api/nodes', { method: 'POST', body }),
    update: (id: number, body: Partial<Node>) => request<Node>(`/api/nodes/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/nodes/${id}`, { method: 'DELETE' }),
  },
  users: {
    list: () => request<User[]>('/api/users'),
    create: (body: Partial<User>) => request<User>('/api/users', { method: 'POST', body }),
    update: (id: number, body: Partial<User>) => request<User>(`/api/users/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/users/${id}`, { method: 'DELETE' }),
  },
  files: {
    list: () => request<DistFile[]>('/api/files'),
    create: (fd: FormData) => request<DistFile>('/api/files', { method: 'POST', body: fd }),
    update: (id: number, body: Partial<DistFile>) => request<DistFile>(`/api/files/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/files/${id}`, { method: 'DELETE' }),
    refresh: (id: number) => request<DistFile>(`/api/files/${id}/refresh`, { method: 'POST' }),
    getContent: (id: number) => request<{ content: string }>(`/api/files/${id}/content`),
    updateContent: (id: number, content_text: string) =>
      request<DistFile>(`/api/files/${id}/content`, { method: 'POST', body: { content_text } }),
  },
  settings: {
    sharedToken: () => request<{ token: string }>('/api/settings/shared-token'),
    resetSharedToken: () => request<{ token: string }>('/api/settings/shared-token/reset', { method: 'POST' }),
  },
  // 用户侧下载（无需 Admin Token）：ZIP 个性化渲染走用户 token
  downloadZip: async (id: number, token: string): Promise<Blob> => {
    const res = await fetch(`${apiBase()}/dl/${id}?${new URLSearchParams({ token }).toString()}`)
    if (!res.ok) throw new ApiError(res.status, `下载失败 (${res.status})`)
    return res.blob()
  },
  // 用户侧订阅预览（无需 Admin Token）
  sub: async (token: string): Promise<Record<string, unknown>> => {
    const res = await fetch(`${apiBase()}/sub?token=${encodeURIComponent(token)}`)
    if (!res.ok) throw new ApiError(res.status, `订阅拉取失败 (${res.status})`)
    return res.json()
  },
  // 用户侧 Mihomo 代理列表 (YAML)
  mihomo: async (token: string): Promise<string> => {
    const res = await fetch(`${apiBase()}/mihomo?token=${encodeURIComponent(token)}`)
    if (!res.ok) throw new ApiError(res.status, `订阅拉取失败 (${res.status})`)
    return res.text()
  },
}
