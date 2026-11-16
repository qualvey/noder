import type { ProtocolHandler } from '../types'
import { vlessHandler } from './vless'
import { tuicHandler } from './tuic'

const handlers = new Map<string, ProtocolHandler>()
const schemeMap = new Map<string, ProtocolHandler>()

export function registerHandler(handler: ProtocolHandler) {
  handlers.set(handler.protocol.toLowerCase(), handler)
  schemeMap.set(handler.urlScheme.toLowerCase(), handler)
}

// 默认注册支持的协议
registerHandler(vlessHandler)
registerHandler(tuicHandler)

export function getHandlerByProtocol(protocol?: string): ProtocolHandler | undefined {
  if (!protocol) return undefined
  return handlers.get(protocol.toLowerCase())
}

export function getHandlerByUrl(urlStr: string): ProtocolHandler | undefined {
  try {
    const match = urlStr.match(/^([a-z][a-z\d+.-]*):/i)
    if (!match) return undefined
    const scheme = `${match[1].toLowerCase()}:`
    return schemeMap.get(scheme)
  } catch {
    return undefined
  }
}

export function getAllHandlers(): ProtocolHandler[] {
  return Array.from(handlers.values())
}

export { vlessHandler, tuicHandler }
