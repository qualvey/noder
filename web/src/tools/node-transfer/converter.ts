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
    tryParseJsonWithArrayFallback(value)
    return 'json2link'
  } catch {
    return fallback
  }
}

/**
 * 容错 JSON 解析：支持标准 JSON、被 [] 包裹的数组，以及裸的以逗号分隔的多对象片段 ({...},\n{...})
 */
export function tryParseJsonWithArrayFallback(text: string): unknown {
  const trimmed = text.trim()
  try {
    return JSON.parse(trimmed)
  } catch (originalErr) {
    // 尝试容错包裹 []（处理裸逗号分隔或无逗号紧跟的多个 JSON 对象，去除末尾逗号）
    let cleaned = trimmed.replace(/,\s*$/, '')
    // 兼容缺少逗号分隔的连续对象：如 }\n{ 替换为 },{
    cleaned = cleaned.replace(/\}\s*\{/g, '},{')
    if (cleaned.startsWith('{') && cleaned.endsWith('}')) {
      try {
        return JSON.parse(`[${cleaned}]`)
      } catch {
        // 二次尝试仍失败，抛出最初的解析异常
      }
    }
    throw originalErr
  }
}

/**
 * 从 JSON 文本中提取多个或单个 Outbound 配置并校验
 */
export function parseJsonConfigs(jsonStr: string): {
  configs: BaseOutboundConfig[]
  error: string | null
  warnings: string[]
} {
  const trimmed = jsonStr.trim()
  if (!trimmed) {
    return { configs: [], error: null, warnings: [] }
  }

  let raw: unknown
  try {
    raw = tryParseJsonWithArrayFallback(trimmed)
  } catch (e: any) {
    return { configs: [], error: `JSON 解析错误: ${e.message}`, warnings: [] }
  }

  const supportedHandlers = getAllHandlers()
  const supportedProtocols = supportedHandlers.map((h) => h.protocol)

  let candidates: unknown[] = []
  if (Array.isArray(raw)) {
    candidates = raw
  } else if (raw && typeof raw === 'object' && Array.isArray((raw as any).outbounds)) {
    candidates = (raw as any).outbounds
  } else if (raw && typeof raw === 'object') {
    candidates = [raw]
  } else {
    return { configs: [], error: '配置必须是一个 JSON 对象或对象数组', warnings: [] }
  }

  if (candidates.length === 0) {
    return { configs: [], error: '未在输入中找到任何节点配置', warnings: [] }
  }

  const configs: BaseOutboundConfig[] = []
  const errors: string[] = []

  for (let i = 0; i < candidates.length; i++) {
    const item = candidates[i]
    if (!item || typeof item !== 'object') {
      errors.push(`第 ${i + 1} 个项不是有效的 JSON 对象`)
      continue
    }

    const type = (item as any).type
    if (!type || !supportedProtocols.includes(type)) {
      // 如果输入的是完整 sing-box 配置里的其他非代理出站（如 direct, block, dns 等），在数组中跳过，不报阻断性错误
      if (candidates.length > 1 && type && ['direct', 'block', 'dns'].includes(type)) {
        continue
      }
      errors.push(
        `第 ${i + 1} 个节点类型 (${type || '未指定'}) 不受支持，目前支持: ${supportedProtocols.join(', ')}`,
      )
      continue
    }

    const handler = getHandlerByProtocol(type)
    if (!handler) continue

    const valErr = handler.validate(item)
    if (valErr) {
      errors.push(`第 ${i + 1} 个节点校验失败: ${valErr}`)
      continue
    }

    configs.push(item as BaseOutboundConfig)
  }

  if (configs.length === 0) {
    return {
      configs: [],
      error: errors.length > 0 ? errors[0] : `未找到有效且支持的节点配置 (支持: ${supportedProtocols.join(', ')})`,
      warnings: [],
    }
  }

  return { configs: normalizeTags(configs), error: null, warnings: errors }
}

/**
 * 兼容单配置解析接口
 */
export function parseJsonConfig(jsonStr: string): {
  config: BaseOutboundConfig | null
  error: string | null
} {
  const res = parseJsonConfigs(jsonStr)
  return {
    config: res.configs.length > 0 ? res.configs[0] : null,
    error: res.error,
  }
}

/**
 * 解析单个分享链接为 sing-box outbound 配置
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
 * 解析多行分享链接为 sing-box outbound 配置列表
 */
export function parseLinksToConfigs(linksStr: string): {
  configs: BaseOutboundConfig[]
  error: string | null
  warnings: string[]
} {
  const trimmed = linksStr.trim()
  if (!trimmed) {
    return { configs: [], error: null, warnings: [] }
  }

  const lines = trimmed
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean)

  if (lines.length === 0) {
    return { configs: [], error: null, warnings: [] }
  }

  const configs: BaseOutboundConfig[] = []
  const warnings: string[] = []
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    try {
      const cfg = parseLinkToConfig(line)
      configs.push(cfg)
    } catch (e: any) {
      warnings.push(lines.length > 1 ? `第 ${i + 1} 行链接解析失败: ${e.message}` : `链接解析错误: ${e.message}`)
    }
  }

  if (configs.length === 0) return { configs: [], error: warnings[0] || '未找到有效链接', warnings }
  return { configs: normalizeTags(configs), error: null, warnings }
}

/** 确保 sing-box outbound 的 tag 唯一，同时尽量保留用户提供的原始 tag。 */
export function normalizeTags(configs: BaseOutboundConfig[]): BaseOutboundConfig[] {
  const used = new Set<string>()
  return configs.map((config, index) => {
    const base = (config.tag || `${config.type || 'node'}-${config.server || index + 1}`).trim() || `node-${index + 1}`
    let tag = base
    let suffix = 2
    while (used.has(tag)) tag = `${base}-${suffix++}`
    used.add(tag)
    return { ...config, tag }
  })
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
 * 将多个配置批量生成分享链接 (以换行连接)
 */
export function buildShareLinks(configs: BaseOutboundConfig[]): string {
  return configs.map((c) => buildShareLink(c)).filter(Boolean).join('\n')
}

/**
 * 提取节点配置展示详情
 */
export function extractConfigDetails(config: BaseOutboundConfig): ConfigDetailItem[] {
  const handler = getHandlerByProtocol(config.type)
  if (!handler) return []
  return handler.getDetails(config)
}
