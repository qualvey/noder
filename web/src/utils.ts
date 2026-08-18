// 通用工具函数

export function escapeHtml(str: unknown): string {
  return String(str ?? '').replace(/[&<>"']/g, (c) => {
    const map: Record<string, string> = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;',
    }
    return map[c]
  })
}

export function formatFileSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

export function randomUUID(): string {
  return crypto.randomUUID()
}

export function randomPassword(): string {
  return Math.random().toString(36).substring(2, 10) + Math.random().toString(36).substring(2, 10)
}

// 从粘贴文本中提取 uuid / password (兼容 yaml/json 写法)
export function parseCredentials(raw: string): { uuid?: string; password?: string } {
  const uuidMatch = raw.match(/["']?uuid["']?\s*[:=]\s*["']?([0-9a-fA-F-]{36})["']?/i)
  const pwdMatch = raw.match(/["']?password["']?\s*[:=]\s*["']?([^"'\s,}]+)["']?/i)
  return {
    uuid: uuidMatch ? uuidMatch[1] : undefined,
    password: pwdMatch ? pwdMatch[1] : undefined,
  }
}

// 从粘贴的 outbound JSON 解析节点字段
export function parseNodeJson(jsonStr: string): Partial<Record<string, unknown>> | null {
  try {
    const obj = JSON.parse(jsonStr)
    if (typeof obj !== 'object' || obj === null) return null
    return obj as Record<string, unknown>
  } catch {
    return null
  }
}
