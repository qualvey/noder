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
  public_key?: string | null
  short_id?: string | null
  fingerprint?: string | null
  flow?: string | null
  congestion_control?: string | null
  remark?: string | null
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

export interface SubNodeItem {
  id: number
  node_name: string
  protocol: string
  server_address: string
  server_port: number
  outbound: Record<string, unknown>
}
