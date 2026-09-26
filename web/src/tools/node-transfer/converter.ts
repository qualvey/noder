import type { BaseOutboundConfig, ConfigDetailItem } from './types'
import { getHandlerByProtocol, getHandlerByUrl, getAllHandlers } from './handlers'

export type TransferDirection = 'json2link' | 'link2json'

/**
 * 自动识别转换方向
 */
export function detectDirection(
  input: string,
  fallback: TransferDirection = 'json2link',
): TransferDirection {
  const value = input.trim()
  if (!value) return fallback
  if (/^[a-z][a-z\d+.-]*:\/\//i.test(value)) return 'link2json'
  if (value.startsWith('{') || value.startsWith('[')) return 'json2link'
  try {
    JSON.parse(value)
    return 'json2link'
  } catch {
    return fallback
  }
}

/**
 * 从 JSON 文本中提取 Outbound 配置并校验
 */
export function parseJsonConfig(jsonStr: string): {
  config: BaseOutboundConfig | null
  error: string | null
} {
  const trimmed = jsonStr.trim()
  if (!trimmed) {
    return { config: null, error: null }
  }

  let raw: unknown
  try {
    raw = JSON.parse(trimmed)
  } catch (e: any) {
    return { config: null, error: `JSON 解析错误: ${e.message}` }
  }

  const supportedHandlers = getAllHandlers()
  const supportedProtocols = supportedHandlers.map((h) => h.protocol)

  let config: any = raw
  if (raw && typeof raw === 'object') {
    if (Array.isArray(raw)) {
      config = raw.find((item: any) => item && supportedProtocols.includes(item.type))
      if (!config) {
        return {
          config: null,
          error: `数组中未找到 type 为 ${supportedProtocols.join(' 或 ')} 的出站配置`,
        }
      }
    } else if (Array.isArray((raw as any).outbounds)) {
      const outbounds = (raw as any).outbounds
      config = outbounds.find((item: any) => item && supportedProtocols.includes(item.type))
      if (!config) {
        return {
          config: null,
          error: `配置文件的 outbounds 中未找到 type 为 ${supportedProtocols.join(' 或 ')} 的出站配置`,
        }
      }
    }
  }

  if (!config || typeof config !== 'object') {
    return { config: null, error: '配置必须是一个 JSON 对象' }
  }

  const handler = getHandlerByProtocol(config.type)
  if (!handler) {
    return {
      config: null,
      error: `配置类型必须为 ${supportedProtocols.join(' 或 ')}，当前为: ${config.type || '未指定'}`,
    }
  }

  const validationError = handler.validate(config)
  if (validationError) {
    return { config: null, error: validationError }
  }

  return { config, error: null }
}

/**
 * 解析分享链接为 sing-box outbound 配置
 */
export function parseLinkToConfig(link: string): BaseOutboundConfig {
  const cleanLink = link.trim()
  if (!cleanLink) {
    throw new Error('输入内容为空')
  }

  const handler = getHandlerByUrl(cleanLink)
  if (!handler) {
    const supportedSchemes = getAllHandlers()
      .map((h) => h.urlScheme)
      .join(' 或 ')
    throw new Error(`未知的分享链接格式，必须以 ${supportedSchemes} 开头`)
  }

  let url: URL
  try {
    url = new URL(cleanLink)
  } catch (e: any) {
    throw new Error(`链接格式不合法: ${e.message}`)
  }

  return handler.parseUrl(url)
}

/**
 * 将 sing-box outbound 配置生成分享链接
 */
export function buildShareLink(config: BaseOutboundConfig): string {
  const handler = getHandlerByProtocol(config.type)
  if (!handler) return ''
  return handler.buildLink(config)
}

/**
 * 提取节点配置展示详情
 */
export function extractConfigDetails(config: BaseOutboundConfig): ConfigDetailItem[] {
  const handler = getHandlerByProtocol(config.type)
  if (!handler) return []
  return handler.getDetails(config)
}
