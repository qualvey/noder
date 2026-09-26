import type {
  TuicOutboundConfig,
  ProtocolHandler,
  ConfigDetailItem,
} from '../types'
import { safeDecodeURIComponent } from '../utils'

const SAMPLE_TUIC_JSON = `{
  "type": "tuic",
  "tag": "rjp-tuic",
  "server": "rjp.wowoha.top",
  "server_port": 8443,
  "uuid": "45d357f4-0540-41ed-8e0d-10806b70b386",
  "password": "my-secret-password",
  "congestion_control": "bbr",
  "udp_relay_mode": "native",
  "alpn": [
    "h3"
  ],
  "tls": {
    "enabled": true,
    "server_name": "www.amazon.com",
    "insecure": false
  }
}`

const SAMPLE_TUIC_LINK =
  'tuic://45d357f4-0540-41ed-8e0d-10806b70b386:my-secret-password@rjp.wowoha.top:8443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=www.amazon.com#rjp-tuic'

export const tuicHandler: ProtocolHandler<TuicOutboundConfig> = {
  protocol: 'tuic',
  urlScheme: 'tuic:',
  displayName: 'TUIC',
  sample: {
    json: SAMPLE_TUIC_JSON,
    link: SAMPLE_TUIC_LINK,
  },

  validate(config: unknown): string | null {
    if (!config || typeof config !== 'object') {
      return '配置必须是一个 JSON 对象'
    }
    const c = config as Record<string, unknown>
    if (!c.server) return '缺少 server (服务器地址) 字段'
    if (!c.server_port) return '缺少 server_port (服务器端口) 字段'
    if (!c.uuid) return '缺少 uuid 字段'
    if (!c.password) return '缺少 password 字段 (TUIC 协议必填)'
    return null
  },

  parseUrl(url: URL): TuicOutboundConfig {
    if (url.protocol !== 'tuic:') {
      throw new Error('协议不正确，必须为 tuic://')
    }

    const uuid = url.username
    const password = safeDecodeURIComponent(url.password || '')
    const server = url.hostname
    const portStr = url.port
    const port = portStr ? parseInt(portStr, 10) : 443
    const tag = url.hash ? safeDecodeURIComponent(url.hash.substring(1)) : 'tuic'

    if (!uuid) throw new Error('链接中缺少 UUID')
    if (!password) throw new Error('链接中缺少密码 (password)')
    if (!server) throw new Error('链接中缺少服务器地址 (server)')
    if (!portStr) throw new Error('链接中缺少服务器端口 (port)')

    const searchParams = url.searchParams
    const congestionControl = searchParams.get('congestion_control')
    const udpRelayMode = searchParams.get('udp_relay_mode')
    const alpn = searchParams.get('alpn')
    const sni = searchParams.get('sni')
    const allowInsecure = searchParams.get('allow_insecure')

    const config: TuicOutboundConfig = {
      type: 'tuic',
      tag,
      server,
      server_port: port,
      uuid,
      password,
    }

    if (congestionControl) {
      config.congestion_control = congestionControl
    }
    if (udpRelayMode) {
      config.udp_relay_mode = udpRelayMode
    }

    // ALPN & TLS
    const hasTlsParams = alpn || sni || allowInsecure === '1'
    if (hasTlsParams) {
      config.tls = {
        enabled: true,
      }
      if (sni) {
        config.tls.server_name = sni
      }
      if (allowInsecure === '1') {
        config.tls.insecure = true
      }
    }

    if (alpn) {
      config.alpn = alpn.split(',').map((item) => safeDecodeURIComponent(item.trim()))
    }

    return config
  },

  buildLink(config: TuicOutboundConfig): string {
    const uuid = config.uuid || ''
    const password = config.password || ''
    const server = config.server || ''
    const port = config.server_port || ''
    const tag = encodeURIComponent(config.tag || 'tuic')

    const params: string[] = []

    if (config.congestion_control) {
      params.push(`congestion_control=${config.congestion_control}`)
    }
    if (config.udp_relay_mode) {
      params.push(`udp_relay_mode=${config.udp_relay_mode}`)
    }

    // ALPN
    if (Array.isArray(config.alpn) && config.alpn.length > 0) {
      params.push(`alpn=${encodeURIComponent(config.alpn.join(','))}`)
    } else if (config.tls?.alpn && Array.isArray(config.tls.alpn) && config.tls.alpn.length > 0) {
      params.push(`alpn=${encodeURIComponent(config.tls.alpn.join(','))}`)
    }

    // SNI
    if (config.tls?.server_name) {
      params.push(`sni=${config.tls.server_name}`)
    }

    // Allow Insecure
    if (config.tls?.insecure) {
      params.push('allow_insecure=1')
    }

    const queryString = params.length > 0 ? `?${params.join('&')}` : ''
    return `tuic://${uuid}:${password}@${server}:${port}${queryString}#${tag}`
  },

  getDetails(config: TuicOutboundConfig): ConfigDetailItem[] {
    const details: ConfigDetailItem[] = [
      { label: '出站协议 (type)', value: config.type, highlight: true },
      { label: '别名标识 (tag)', value: config.tag || '-' },
      { label: '服务器地址 (server)', value: config.server },
      { label: '端口 (server_port)', value: config.server_port },
      { label: 'UUID', value: config.uuid, copyable: true },
      { label: '密码 (password)', value: config.password || '-', copyable: true },
      { label: '拥塞控制 (congestion_control)', value: config.congestion_control || 'bbr' },
      { label: 'UDP 转发模式 (udp_relay_mode)', value: config.udp_relay_mode || 'native' },
    ]

    let alpn = '-'
    if (Array.isArray(config.alpn)) {
      alpn = config.alpn.join(', ')
    } else if (config.tls?.alpn && Array.isArray(config.tls.alpn)) {
      alpn = config.tls.alpn.join(', ')
    }
    details.push({ label: 'ALPN', value: alpn })
    details.push({
      label: 'TLS SNI (server_name)',
      value: config.tls?.server_name || '-',
    })
    details.push({
      label: '允许不安全 TLS (insecure)',
      value: config.tls?.insecure ? '是 (true)' : '否 (false)',
    })

    return details
  },
}
