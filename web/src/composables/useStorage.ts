/**
 * 统一的浏览器存储封装。
 * 所有 localStorage / sessionStorage 访问应通过此 composable 进行，
 * 便于在 SSR 环境下做降级处理，也方便单元测试 mock。
 */
export function useStorage(type: 'local' | 'session' = 'local') {
  const storage = type === 'local' ? localStorage : sessionStorage

  function get(key: string): string | null {
    try {
      return storage.getItem(key)
    } catch {
      return null
    }
  }

  function set(key: string, value: string): void {
    try {
      storage.setItem(key, value)
    } catch {
      // 隐私模式等环境下存储可能不可用，静默忽略
    }
  }

  function remove(key: string): void {
    try {
      storage.removeItem(key)
    } catch {
      /* ignore */
    }
  }

  function getBoolean(key: string, defaultValue = false): boolean {
    const raw = get(key)
    if (raw === null) return defaultValue
    return raw === 'true'
  }

  function setBoolean(key: string, value: boolean): void {
    set(key, String(value))
  }

  return { get, set, remove, getBoolean, setBoolean }
}
