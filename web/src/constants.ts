// 前端字段取值白名单 —— 与后端 app/contracts.py 的 enum 对齐（契约层为唯一事实源）
// 改动时两处必须同步：app/contracts.py 的 UTLS_FINGERPRINTS / VLESS_FLOWS / TUIC_CONGESTION_CONTROLS

export const UTLS_FINGERPRINTS = [
  'chrome', 'firefox', 'edge', 'safari', '360', 'qq', 'ios',
  'android', 'random', 'randomized',
] as const

export const VLESS_FLOWS = ['', 'xtls-rprx-vision'] as const

export const TUIC_CONGESTION_CONTROLS = ['bbr', 'cubic', 'new_reno'] as const

// Sing-box V2Ray transport types. direct means the default transport and is
// intentionally omitted from generated sing-box JSON.
export const TRANSPORT_TYPES = ['http', 'ws', 'quic', 'grpc', 'httpupgrade'] as const

export function normalizeTransportType(value: unknown): string {
  const transport = String(value ?? '').trim().toLowerCase()
  return (TRANSPORT_TYPES as readonly string[]).includes(transport) ? transport : 'direct'
}

export function isEnumValue<T extends readonly string[]>(list: T, value: string | null | undefined): value is T[number] {
  return value !== null && value !== undefined && (list as readonly string[]).includes(value)
}
