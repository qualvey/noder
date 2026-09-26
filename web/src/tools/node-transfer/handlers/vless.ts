import type {
  VlessOutboundConfig,
  ProtocolHandler,
  ConfigDetailItem,
} from '../types'
import { safeDecodeURIComponent, extractTransportHost } from '../utils'

const SAMPLE_VLESS_JSON = `{
  "type": "vless",
  "tag": "rjp-reality",
  "server": "rjp.wowoha.top",
  "server_port": 8443,
  "uuid": "45d357f4-0540-41ed-8e0d-10806b70b386",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "www.amazon.com",
    "utls": {
      "enabled": true,
      "fingerprint": "chrome"
    },
    "reality": {
      "enabled": true,
      "public_key": "PjePONzwgc9HCWQf209unDj_gVwaqX4c-a2hgi4t2R0",
      "short_id": "0123456789abcdef"
    }
  },
  "domain_resolver": "ali"
}`

const SAMPLE_VLESS_LINK =
  'vless://45d357f4-0540-41ed-8e0d-10806b70b386@rjp.wowoha.top:8443?flow=xtls-rprx-vision&security=reality&sni=www.amazon.com&fp=chrome&pbk=PjePONzwgc9HCWQf209unDj_gVwaqX4c-a2hgi4t2R0&sid=0123456789abcdef#rjp-reality'

export const vlessHandler: ProtocolHandler<VlessOutboundConfig> = {
  protocol: 'vless',
  urlScheme: 'vless:',
  displayName: 'VLESS',
  sample: {
    json: SAMPLE_VLESS_JSON,
    link: SAMPLE_VLESS_LINK,
  },

  validate(config: unknown): string | null {
    if (!config || typeof config !== 'object') {
      return '配置必须是一个 JSON 对象'
    }
    const c = config as Record<string, unknown>
    if (!c.server) return '缺少 server (服务器地址) 字段'
    if (!c.server_port) return '缺少 server_port (服务器端口) 字段'
    if (!c.uuid) return '缺少 uuid 字段'
    return null
  },

  parseUrl(url: URL): VlessOutboundConfig {
    if (url.protocol !== 'vless:') {
      throw new Error('协议不正确，必须为 vless://')
    }

    const uuid = url.username
    const server = url.hostname
    const portStr = url.port
    const port = portStr ? parseInt(portStr, 10) : 443
    const tag = url.hash ? safeDecodeURIComponent(url.hash.substring(1)) : 'vless'

    if (!uuid) throw new Error('链接中缺少 UUID')
    if (!server) throw new Error('链接中缺少服务器地址 (server)')
    if (!portStr) throw new Error('链接中缺少服务器端口 (port)')

    const searchParams = url.searchParams
    const flow = searchParams.get('flow')
    const security = searchParams.get('security')
    const sni = searchParams.get('sni')
    const fp = searchParams.get('fp')
    const pbk = searchParams.get('pbk')
    const sid = searchParams.get('sid')

    const config: VlessOutboundConfig = {
      type: 'vless',
      tag,
      server,
      server_port: port,
      uuid,
    }

    if (flow) {
      config.flow = flow
    }

    // TLS & Reality
    if (security === 'reality' || security === 'tls' || sni || fp || pbk || sid) {
      config.tls = {
        enabled: true,
      }
      if (sni) {
        config.tls.server_name = sni
      }
      if (fp) {
        config.tls.utls = {
          enabled: true,
          fingerprint: fp,
        }
      }
      if (security === 'reality' || pbk || sid) {
        config.tls.reality = {
          enabled: true,
          public_key: pbk || '',
          short_id: sid || '',
        }
      }
    }

    // Transport
    const transportType = searchParams.get('type')
    const validTransports = ['ws', 'http', 'grpc', 'quic', 'httpupgrade']
    if (transportType && validTransports.includes(transportType)) {
      config.transport = {
        type: transportType,
      }
      if (transportType === 'ws' || transportType === 'http' || transportType === 'httpupgrade') {
        const path = searchParams.get('path')
        if (path) {
          config.transport.path = safeDecodeURIComponent(path)
        }
        const host = searchParams.get('host')
        if (host) {
          config.transport.host = [safeDecodeURIComponent(host)]
        }
      } else if (transportType === 'grpc') {
        const serviceName = searchParams.get('serviceName')
        if (serviceName) {
          config.transport.service_name = safeDecodeURIComponent(serviceName)
        }
      }
    }

    return config
  },

  buildLink(config: VlessOutboundConfig): string {
    const uuid = config.uuid || ''
    const server = config.server || ''
    const port = config.server_port || ''
    const tag = encodeURIComponent(config.tag || 'vless')

    const params: string[] = []

    // Flow
    if (config.flow) {
      params.push(`flow=${config.flow}`)
    }

    // Security & TLS & Reality
    let security = 'none'
    if (config.tls?.enabled) {
      if (config.tls.reality?.enabled) {
        security = 'reality'
      } else {
        security = 'tls'
      }
    }
    params.push(`security=${security}`)

    // SNI
    if (config.tls?.server_name) {
      params.push(`sni=${config.tls.server_name}`)
    }

    // uTLS Fingerprint
    if (config.tls?.utls?.enabled && config.tls.utls.fingerprint) {
      params.push(`fp=${config.tls.utls.fingerprint}`)
    }

    // Reality parameters
    if (security === 'reality' && config.tls?.reality) {
      const pbk = config.tls.reality.public_key
      if (pbk) {
        params.push(`pbk=${pbk}`)
      }
      const sid = config.tls.reality.short_id
      if (sid) {
        params.push(`sid=${sid}`)
      }
    }

    // Transport settings
    const validTransports = ['ws', 'http', 'grpc', 'quic', 'httpupgrade']
    if (config.transport?.type && validTransports.includes(config.transport.type)) {
      const transportType = config.transport.type
      params.push(`type=${transportType}`)

      if (transportType === 'ws' || transportType === 'http' || transportType === 'httpupgrade') {
        if (config.transport.path) {
          params.push(`path=${encodeURIComponent(config.transport.path)}`)
        }
        const host = extractTransportHost(config.transport)
        if (host) {
          params.push(`host=${encodeURIComponent(host)}`)
        }
      } else if (transportType === 'grpc') {
        if (config.transport.service_name) {
          params.push(`serviceName=${encodeURIComponent(config.transport.service_name)}`)
        }
      }
    }

    const queryString = params.length > 0 ? `?${params.join('&')}` : ''
    return `vless://${uuid}@${server}:${port}${queryString}#${tag}`
  },

  getDetails(config: VlessOutboundConfig): ConfigDetailItem[] {
    const details: ConfigDetailItem[] = [
      { label: '出站协议 (type)', value: config.type, highlight: true },
      { label: '别名标识 (tag)', value: config.tag || '-' },
      { label: '服务器地址 (server)', value: config.server },
      { label: '端口 (server_port)', value: config.server_port },
      { label: 'UUID', value: config.uuid, copyable: true },
      { label: '流控算法 (flow)', value: config.flow || 'none' },
    ]

    // TLS Info
    const tlsEnabled = !!config.tls?.enabled
    details.push({
      label: 'TLS 状态',
      value: tlsEnabled ? '已启用 (Enabled)' : '未启用 (Disabled)',
    })

    if (tlsEnabled) {
      details.push({
        label: 'TLS SNI (server_name)',
        value: config.tls?.server_name || '-',
      })

      const utlsEnabled = !!config.tls?.utls?.enabled
      details.push({
        label: 'uTLS 混淆指纹',
        value: utlsEnabled ? `已启用 (${config.tls?.utls?.fingerprint || 'none'})` : '未启用',
      })

      const realityEnabled = !!config.tls?.reality?.enabled
      details.push({
        label: 'Reality 混淆',
        value: realityEnabled ? '已启用 (REALITY)' : '未启用',
      })

      if (realityEnabled && config.tls?.reality) {
        details.push(
          {
            label: 'Reality 公钥 (pbk)',
            value: config.tls.reality.public_key || '-',
            copyable: true,
          },
          {
            label: 'Reality 短 ID (sid)',
            value: config.tls.reality.short_id || '-',
          },
        )
      }
    }

    // Transport Info
    if (config.transport?.type) {
      details.push({
        label: '传输层协议 (type)',
        value: config.transport.type,
      })
      if (config.transport.path) {
        details.push({
          label: '传输路径 (path)',
          value: config.transport.path,
        })
      }

      const host = extractTransportHost(config.transport)
      if (host) {
        details.push({ label: '传输主机 (host)', value: host })
      }

      if (config.transport.service_name) {
        details.push({
          label: 'gRPC 服务名 (service_name)',
          value: config.transport.service_name,
        })
      }
    }

    return details
  },
}
