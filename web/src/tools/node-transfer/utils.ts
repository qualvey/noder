/**
 * 安全的 URIComponent 解码，防止非法百分号导致 URIError 崩溃
 */
export function safeDecodeURIComponent(val: string): string {
  if (!val) return ''
  try {
    return decodeURIComponent(val)
  } catch {
    return val
  }
}

/**
 * 解析 host 配置 (string | string[] | headers)
 */
export function extractTransportHost(transport?: {
  host?: string | string[]
  headers?: Record<string, string>
}): string {
  if (!transport) return ''
  if (typeof transport.host === 'string') {
    return transport.host
  }
  if (Array.isArray(transport.host) && transport.host.length > 0) {
    return transport.host[0]
  }
  if (transport.headers) {
    const hostKey = Object.keys(transport.headers).find((k) => k.toLowerCase() === 'host')
    if (hostKey) {
      return transport.headers[hostKey]
    }
  }
  return ''
}
