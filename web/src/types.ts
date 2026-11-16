// 与后端 API 对齐的 TypeScript 类型定义

export interface Node {
  id: number
  tag: string
  node_name?: string | null
  protocol: 'tuic' | 'vless' | 'anytls'
  server_address: string
  server_port: number
  method?: string | null
  security?: string | null
  sni?: string | null
  transport_type?: 'direct' | 'http' | 'ws' | 'quic' | 'grpc' | 'httpupgrade' | 'xhttp' | string | null
  path?: string | null
  is_active: boolean
  is_agentless?: boolean
  public_key?: string | null
  short_id?: string | null
  fingerprint?: string | null
  flow?: string | null
  congestion_control?: string | null
  remark?: string | null
}

export interface Agent {
  id: string
  name: string
  status: 'online' | 'offline' | 'disabled'
  last_heartbeat_at?: string | null
  is_online: boolean
  metadata: string
  binding_count: number
  created_at: string
  updated_at: string
}

export interface AgentBinding {
  endpoint_id: number
  endpoint_tag: string
  endpoint_name: string
  protocol: string
  is_active: boolean
  status: string
  last_sync_at?: string | null
  last_error?: string | null
}

export interface EndpointBinding {
  agent_id: string
  agent_name: string
  agent_status: string
  is_online: boolean
  last_heartbeat_at?: string | null
  status: string
  last_sync_at?: string | null
  last_error?: string | null
}

export interface Endpoint {
  id?: number
  user_id?: number
  node_id: number
  tag: string
  uuid: string
  password?: string | null
  is_active?: boolean
  is_urltest?: boolean
  sort_order?: number
  remark?: string | null
  node?: Node
}

export interface User {
  id: number
  name: string
  token: string
  uuid?: string | null
  password?: string | null
  is_active: boolean
  remark?: string | null
  config_override?: string | null
  node_ids: number[]
  endpoints?: Endpoint[]
  template_branch_id?: number | null
  independent_node_ids: number[]
  download_count: number
  last_download_at?: string | null
}

export interface TemplateBranch {
  id: number
  name: string
  target: string
  content_format: string
  content: string
  remark?: string | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface Template {
  id: number
  name: string
  target: string
  content_format: string
  content: string
  version: number
  updated_at: string
}

export interface UserDownload {
  id: number
  file_id: number
  file_name: string
  downloaded_at: string
}

export type FileType = 'apk' | 'zip' | 'text'

export interface DistFile {
  id: number
  name: string
  file_type: FileType
  template_name?: string | null
  original_name: string
  download_name?: string | null
  size: number
  is_active: boolean
  remark?: string | null
  source_url?: string | null
  cached_at?: string | null
  created_at: string
}

export interface ServiceReleaseLog {
  id: number
  subscription_id: string
  credential_id?: string | null
  endpoint_id: number
  external_ref?: string | null
  caller_ip: string
  request_id?: string | null
  http_status: number
  business_code: number
  error_code?: string | null
  message?: string | null
  delivered: boolean
  deployment_count: number
  agent_online: boolean
  latency_ms: number
  created_at: string
}

export interface ServiceReleaseLogPage {
  items: ServiceReleaseLog[]
  total: number
  page: number
  limit: number
  page_size: number
  total_pages: number
}

export interface ServiceReleaseLogStats {
  total: number
  success: number
  binding_missing: number
  not_found: number
  delivered: number
  agent_offline: number
}

export interface SubNodeItem {
  id: number
  node_name: string
  protocol: string
  server_address: string
  server_port: number
  outbound: Record<string, unknown>
}
