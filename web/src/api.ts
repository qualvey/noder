// API 客户端：统一鉴权、错误处理、相对路径（兼容子路径反代）
import type { DistFile, Endpoint, Node, Template, TemplateBranch, User, UserDownload } from './types'

const base = import.meta.env.BASE_URL
// dev 下 BASE_URL 为 '/'，build 后为 './'
export function apiBase(): string {
  return base.endsWith('/') ? base.slice(0, -1) : base
}

const ADMIN_TOKEN_KEY = 'noder_admin_token'

function loadAdminToken(): string {
  try {
    return sessionStorage.getItem(ADMIN_TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

let adminToken = loadAdminToken()

export function getAdminToken(): string {
  return adminToken
}

export function setAdminToken(t: string): void {
  const next = t.trim()
  adminToken = next

  try {
    if (next) {
      sessionStorage.setItem(ADMIN_TOKEN_KEY, next)
    } else {
      sessionStorage.removeItem(ADMIN_TOKEN_KEY)
    }
  } catch {
    // 某些隐私模式或浏览器限制下，sessionStorage 可能不可用，直接忽略即可
  }
}

export function clearAdminToken(): void {
  adminToken = ''
  try {
    sessionStorage.removeItem(ADMIN_TOKEN_KEY)
  } catch {
    // ignore
  }
}

export async function validateAdminToken(token: string): Promise<boolean> {
  const res = await fetch(apiBase() + '/api/auth/check', {
    headers: { 'X-Admin-Token': token },
  })
  return res.ok
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
  agents: {
    list: (params?: { status?: string; q?: string; page?: number; limit?: number }) => {
      const qs = new URLSearchParams()
      if (params?.status) qs.set('status', params.status)
      if (params?.q) qs.set('q', params.q)
      if (params?.page) qs.set('page', String(params.page))
      if (params?.limit) qs.set('limit', String(params.limit))
      const suffix = qs.toString() ? `?${qs}` : ''
      return request<{ total: number; items: import('./types').Agent[]; page: number; limit: number }>(`/api/agents${suffix}`).then(r => (r as unknown as { data: { total: number; items: import('./types').Agent[]; page: number; limit: number } }).data ?? r as unknown as { total: number; items: import('./types').Agent[]; page: number; limit: number })
    },
    create: (body: { id?: string; name: string }) => request<{ id: string; name: string; status: string; control_token: string }>(`/api/agents`, { method: 'POST', body }),
    get: (id: string) => request<import('./types').Agent>(`/api/agents/${encodeURIComponent(id)}`).then(r => (r as unknown as { data: import('./types').Agent }).data ?? r as unknown as import('./types').Agent),
    update: (id: string, body: { name?: string; status?: string }) => request<import('./types').Agent>(`/api/agents/${encodeURIComponent(id)}`, { method: 'PUT', body }),
    remove: (id: string, force?: boolean) => request<{ message: string }>(`/api/agents/${encodeURIComponent(id)}${force ? '?force=true' : ''}`, { method: 'DELETE' }),
    rotateToken: (id: string) => request<{ id: string; control_token: string }>(`/api/agents/${encodeURIComponent(id)}/token/rotate`, { method: 'POST' }).then(r => (r as unknown as { data: { id: string; control_token: string } }).data ?? r as unknown as { id: string; control_token: string }),
    listBindings: (id: string) => request<import('./types').AgentBinding[]>(`/api/agents/${encodeURIComponent(id)}/bindings`).then(r => (r as unknown as { data: import('./types').AgentBinding[] }).data ?? r as unknown as import('./types').AgentBinding[]),
    addBindings: (id: string, body: { endpoint_id?: number; endpoint_ids?: number[] }) => request<import('./types').AgentBinding[]>(`/api/agents/${encodeURIComponent(id)}/bindings`, { method: 'POST', body }).then(r => (r as unknown as { data: import('./types').AgentBinding[] }).data ?? r as unknown as import('./types').AgentBinding[]),
    removeBinding: (id: string, endpointId: number) => request<{ message: string }>(`/api/agents/${encodeURIComponent(id)}/bindings/${endpointId}`, { method: 'DELETE' }),
  },
  endpointBindings: {
    list: (endpointId: number) => request<import('./types').EndpointBinding[]>(`/api/endpoints/${endpointId}/bindings`).then(r => (r as unknown as { data: import('./types').EndpointBinding[] }).data ?? r as unknown as import('./types').EndpointBinding[]),
    add: (endpointId: number, body: { agent_id?: string; agent_ids?: string[] }) => request<import('./types').EndpointBinding[]>(`/api/endpoints/${endpointId}/bindings`, { method: 'POST', body }).then(r => (r as unknown as { data: import('./types').EndpointBinding[] }).data ?? r as unknown as import('./types').EndpointBinding[]),
    remove: (endpointId: number, agentId: string) => request<{ message: string }>(`/api/endpoints/${endpointId}/bindings/${encodeURIComponent(agentId)}`, { method: 'DELETE' }),
  },
  nodes: {
    list: () => request<Node[]>('/api/nodes'),
    create: (body: Partial<Node>) => request<Node>('/api/nodes', { method: 'POST', body }),
    update: (id: number, body: Partial<Node>) => request<Node>(`/api/nodes/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/nodes/${id}`, { method: 'DELETE' }),
  },
  users: {
    list: () => request<User[]>('/api/users'),
    downloads: (id: number) => request<UserDownload[]>(`/api/users/${id}/downloads`),
    create: (body: Partial<User>) => request<User>('/api/users', { method: 'POST', body }),
    update: (id: number, body: Partial<User>) => request<User>(`/api/users/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/users/${id}`, { method: 'DELETE' }),
    endpoints: (id: number) => request<Endpoint[]>(`/api/users/${id}/endpoints`),
    addEndpoint: (id: number, body: Partial<Endpoint>) => request<Endpoint>(`/api/users/${id}/endpoints`, { method: 'POST', body }),
    updateEndpoint: (endpointId: number, body: Partial<Endpoint>) => request<Endpoint>(`/api/users/0/endpoints/${endpointId}`, { method: 'PUT', body }),
    removeEndpoint: (endpointId: number) => request<{ message: string }>(`/api/users/0/endpoints/${endpointId}`, { method: 'DELETE' }),
  },
  files: {
    list: () => request<DistFile[]>('/api/files'),
    create: (fd: FormData) => request<DistFile>('/api/files', { method: 'POST', body: fd }),
    createRemote: (body: {
      url: string
      name?: string
      remark?: string
      download_name?: string
      file_type?: string
    }) => request<DistFile>('/api/files/remote', { method: 'POST', body }),
    createWithProgress: (
      fd: FormData,
      onProgress?: (percent: number, loaded: number, total: number) => void
    ): Promise<DistFile> => {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        xhr.open('POST', apiBase() + '/api/files')
        xhr.setRequestHeader('X-Admin-Token', adminToken)

        if (xhr.upload && onProgress) {
          xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
              const percent = Math.min(100, Math.round((e.loaded / e.total) * 100))
              onProgress(percent, e.loaded, e.total)
            }
          }
        }

        xhr.onload = () => {
          if (xhr.status === 401) {
            return reject(new ApiError(401, '鉴权失败：Admin Token 无效'))
          }
          try {
            const data = JSON.parse(xhr.responseText || '{}')
            if (xhr.status >= 200 && xhr.status < 300) {
              resolve(data as DistFile)
            } else {
              reject(new ApiError(xhr.status, data.detail || xhr.statusText || '上传失败'))
            }
          } catch {
            reject(new ApiError(xhr.status, xhr.statusText || '上传失败'))
          }
        }

        xhr.onerror = () => {
          reject(new ApiError(0, '网络异常，上传中断'))
        }

        xhr.send(fd)
      })
    },
    update: (id: number, body: Partial<DistFile>) => request<DistFile>(`/api/files/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/files/${id}`, { method: 'DELETE' }),
    refresh: (id: number) => request<DistFile>(`/api/files/${id}/refresh`, { method: 'POST' }),
    getContent: (id: number) => request<{ content: string }>(`/api/files/${id}/content`),
    updateContent: (id: number, content_text: string) =>
      request<DistFile>(`/api/files/${id}/content`, { method: 'POST', body: { content_text } }),
  },
  templateBranches: {
    list: () => request<TemplateBranch[]>('/api/template-branches'),
    create: (body: {
      name: string
      base_template_id?: number
      target: string
      content_format: string
      content: string
      remark?: string
    }) => request<TemplateBranch>('/api/template-branches', { method: 'POST', body }),
    get: (id: number) => request<TemplateBranch>(`/api/template-branches/${id}`),
    update: (id: number, body: {
      name?: string
      content?: string
      content_format?: string
      remark?: string
      is_active?: boolean
    }) => request<TemplateBranch>(`/api/template-branches/${id}`, { method: 'PUT', body }),
    remove: (id: number) => request<{ message: string }>(`/api/template-branches/${id}`, { method: 'DELETE' }),
  },
  templates: {
    list: () => request<Template[]>('/api/templates'),
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
  releaseLogs: {
    list: (params?: {
      page?: number
      limit?: number
      page_size?: number
      subscription_id?: string
      endpoint_id?: number
      credential_id?: string
      external_ref?: string
      caller_ip?: string
      request_id?: string
      http_status?: number
      business_code?: number
      error_code?: string
      delivered?: boolean
      agent_online?: boolean
      since?: string
      until?: string
      q?: string
    }) => {
      const qs = new URLSearchParams()
      if (params)
        for (const [k, v] of Object.entries(params)) {
          if (v === undefined || v === null || v === '') continue
          qs.set(k, String(v))
        }
      const suffix = qs.toString() ? `?${qs}` : ''
      return request<{ items: import('./types').ServiceReleaseLog[]; total: number; page: number; limit: number; total_pages: number }>(`/api/admin/service-release-logs${suffix}`).then(r => (r as unknown as { data: { items: import('./types').ServiceReleaseLog[]; total: number; page: number; limit: number; total_pages: number } }).data ?? r as unknown as { items: import('./types').ServiceReleaseLog[]; total: number; page: number; limit: number; total_pages: number })
    },
    get: (id: number) => request<import('./types').ServiceReleaseLog>(`/api/admin/service-release-logs/${id}`).then(r => (r as unknown as { data: import('./types').ServiceReleaseLog }).data ?? r as unknown as import('./types').ServiceReleaseLog),
    stats: (params?: { since?: string; until?: string }) => {
      const qs = new URLSearchParams()
      if (params?.since) qs.set('since', params.since)
      if (params?.until) qs.set('until', params.until)
      const suffix = qs.toString() ? `?${qs}` : ''
      return request<import('./types').ServiceReleaseLogStats>(`/api/admin/service-release-logs/stats${suffix}`).then(r => (r as unknown as { data: import('./types').ServiceReleaseLogStats }).data ?? r as unknown as import('./types').ServiceReleaseLogStats)
    },
  },
  // 用户侧节点 JSON（无需 Admin Token）
  userNodes: async (token: string): Promise<Record<string, unknown>[]> => {
    const res = await fetch(`${apiBase()}/api/user/nodes?token=${encodeURIComponent(token)}`)
    if (!res.ok) throw new ApiError(res.status, `节点列表拉取失败 (${res.status})`)
    return res.json()
  },
}
