import type {
  BaseOutboundConfig,
  NodeMeta,
  TlsConfig,
  TransportConfig,
  VlessOutboundConfig,
} from './types'

export interface XrayOutboundConfig {
  protocol: 'vless'
  tag: string
  settings: {
    address: string
    port: number
    id: string
    encryption: 'none'
    flow?: string
  }
  streamSettings: {
    method: string
    security: 'reality' | 'tls' | 'none'
    realitySettings?: {
      serverName: string
      fingerprint: string
      password: string
      shortId: string
    }
    tlsSettings?: {
      serverName?: string
    }
  }
}

function readTls(tls: TlsConfig | undefined): Pick<
  NodeMeta,
  'security' | 'serverName' | 'fingerprint' | 'publicKey' | 'shortId'
> {
  if (!tls?.enabled) return { security: 'none' }

  const reality = tls.reality?.enabled
  return {
    security: reality ? 'reality' : 'tls',
    serverName: tls.server_name,
    fingerprint: tls.utls?.fingerprint,
    publicKey: tls.reality?.public_key,
    shortId: tls.reality?.short_id,
  }
}

/** 将 Sing-box outbound 解析为与输出格式无关的节点元数据。 */
export function toNodeMeta(config: BaseOutboundConfig): NodeMeta {
  const tls = (config as VlessOutboundConfig).tls
  const result: NodeMeta = {
    protocol: config.type,
    tag: config.tag,
    server: config.server,
    serverPort: config.server_port,
    uuid: typeof config.uuid === 'string' ? config.uuid : undefined,
    password: typeof config.password === 'string' ? config.password : undefined,
    flow: typeof config.flow === 'string' ? config.flow : undefined,
    transport: config.transport as TransportConfig | undefined,
  }

  if (tls) Object.assign(result, readTls(tls))
  return result
}

/** 将节点元数据还原为标准 Sing-box outbound。 */
export function toSingBoxConfig(meta: NodeMeta): BaseOutboundConfig {
  const config: BaseOutboundConfig = {
    type: meta.protocol,
    ...(meta.tag ? { tag: meta.tag } : {}),
    server: meta.server,
    server_port: meta.serverPort,
  }

  if (meta.uuid) config.uuid = meta.uuid
  if (meta.password) config.password = meta.password
  if (meta.flow) config.flow = meta.flow
  if (meta.transport) config.transport = meta.transport

  if (meta.security && meta.security !== 'none') {
    const tls: TlsConfig = { enabled: true }
    if (meta.serverName) tls.server_name = meta.serverName
    if (meta.fingerprint) tls.utls = { enabled: true, fingerprint: meta.fingerprint }
    if (meta.security === 'reality') {
      tls.reality = {
        enabled: true,
        public_key: meta.publicKey || '',
        short_id: meta.shortId || '',
      }
    }
    config.tls = tls
  }

  return config
}

/** 生成可直接导入 sing-box 的最小完整配置。 */
export function buildSingBoxConfig(outbounds: BaseOutboundConfig[]): Record<string, unknown> {
  const tags = outbounds
    .map((outbound) => outbound.tag)
    .filter((tag): tag is string => Boolean(tag))
  const primary = tags[0] || 'direct'

  return {
    log: { level: 'warn', timestamp: true },
    dns: { servers: [{ tag: 'local', type: 'local' }] },
    inbounds: [
      {
        type: 'mixed',
        tag: 'mixed-in',
        listen: '127.0.0.1',
        listen_port: 2080,
      },
    ],
    outbounds: [
      ...outbounds,
      { type: 'direct', tag: 'direct' },
      { type: 'block', tag: 'block' },
      ...(tags.length > 1
        ? [{ type: 'selector', tag: 'proxy', outbounds: [...tags, 'direct'], default: primary }]
        : []),
    ],
    route: {
      auto_detect_interface: true,
      final: tags.length > 1 ? 'proxy' : primary,
    },
  }
}

/** 将 VLESS 元数据映射为 Xray 配置中的单个 outbound。 */
export function toXrayOutbound(meta: NodeMeta): XrayOutboundConfig {
  if (meta.protocol !== 'vless') {
    throw new Error(`Xray 目前仅支持 VLESS，无法转换 ${meta.protocol}`)
  }
  if (!meta.uuid) throw new Error('Xray VLESS 配置缺少 UUID')

  const outbound: XrayOutboundConfig = {
    protocol: 'vless',
    tag: meta.tag || 'vless',
    settings: {
      address: meta.server,
      port: meta.serverPort,
      id: meta.uuid,
      encryption: 'none',
      ...(meta.flow ? { flow: meta.flow } : {}),
    },
    streamSettings: {
      method: 'raw',
      security: meta.security || 'none',
    },
  }

  if (meta.security === 'reality') {
    if (!meta.serverName || !meta.fingerprint || !meta.publicKey || !meta.shortId) {
      throw new Error('Xray VLESS Reality 配置缺少 SNI、指纹、公钥或 Short ID')
    }
    outbound.streamSettings.realitySettings = {
      serverName: meta.serverName,
      fingerprint: meta.fingerprint,
      password: meta.publicKey,
      shortId: meta.shortId,
    }
  } else if (meta.security === 'tls') {
    outbound.streamSettings.tlsSettings = { serverName: meta.serverName }
  }

  return outbound
}

/** 生成与项目后端一致的 Xray 完整配置。 */
export function toXrayConfig(meta: NodeMeta): Record<string, unknown> {
  return { outbounds: [toXrayOutbound(meta)] }
}
