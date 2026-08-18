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
  transport_type?: string | null
  path?: string | null
  is_active: boolean
  public_key?: string | null
  short_id?: string | null
  fingerprint?: string | null
  flow?: string | null
  congestion_control?: string | null
  remark?: string | null
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
