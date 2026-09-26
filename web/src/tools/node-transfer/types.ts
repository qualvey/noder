export type ProtocolType = 'vless' | 'tuic' | string

export interface BaseOutboundConfig {
  type: ProtocolType
  tag?: string
  server: string
  server_port: number
  [key: string]: unknown
}

export interface TlsConfig {
  enabled?: boolean
  server_name?: string
  insecure?: boolean
  alpn?: string[]
  utls?: {
    enabled?: boolean
    fingerprint?: string
  }
  reality?: {
    enabled?: boolean
    public_key?: string
    short_id?: string
  }
}

export interface TransportConfig {
  type?: 'ws' | 'http' | 'grpc' | 'quic' | 'httpupgrade' | string
  path?: string
  host?: string | string[]
  headers?: Record<string, string>
  service_name?: string
}

export interface VlessOutboundConfig extends BaseOutboundConfig {
  type: 'vless'
  uuid: string
  flow?: string
  tls?: TlsConfig
  transport?: TransportConfig
  domain_resolver?: string
}

export interface TuicOutboundConfig extends BaseOutboundConfig {
  type: 'tuic'
  uuid: string
  password?: string
  congestion_control?: string
  udp_relay_mode?: string
  alpn?: string[]
  tls?: TlsConfig
}

export type OutboundConfig = VlessOutboundConfig | TuicOutboundConfig | BaseOutboundConfig

export interface ConfigDetailItem {
  label: string
  value: string | number
  highlight?: boolean
  copyable?: boolean
}

export interface ProtocolSample {
  json: string
  link: string
}

export interface ProtocolHandler<T extends BaseOutboundConfig = BaseOutboundConfig> {
  protocol: string
  urlScheme: string
  displayName: string
  sample: ProtocolSample
  parseUrl(url: URL): T
  buildLink(config: T): string
  validate(config: unknown): string | null
  getDetails(config: T): ConfigDetailItem[]
}
