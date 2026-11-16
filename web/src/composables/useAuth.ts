import { onBeforeUnmount, ref } from 'vue'
import {
  getAdminToken,
  setAdminToken,
  validateAdminToken,
} from '../api'

const UNLOCKED_KEY = 'noder_unlocked'

/**
 * 管理员 Token 与解锁状态管理。
 * sessionStorage 中的 noder_unlocked 仅是 UI 缓存，重新加载后需重新验证 Token。
 */
export function useAuth() {
  const isUnlocked = ref(false)
  const adminToken = ref(getAdminToken() || '')

  async function validate(token: string): Promise<boolean> {
    try {
      return await validateAdminToken(token)
    } catch {
      return false
    }
  }

  async function unlock(token: string): Promise<boolean> {
    const trimmed = token.trim()
    if (!trimmed) return false

    const ok = await validate(trimmed)
    if (ok) {
      setAdminToken(trimmed)
      adminToken.value = trimmed
      sessionStorage.setItem(UNLOCKED_KEY, 'true')
      isUnlocked.value = true
    } else {
      setAdminToken('')
      adminToken.value = ''
      sessionStorage.removeItem(UNLOCKED_KEY)
      isUnlocked.value = false
    }
    return ok
  }

  function saveToken(token?: string) {
    const trimmed = (token ?? adminToken.value).trim()
    setAdminToken(trimmed)
    adminToken.value = trimmed
  }

  function logout() {
    setAdminToken('')
    adminToken.value = ''
    sessionStorage.removeItem(UNLOCKED_KEY)
    isUnlocked.value = false
  }

  // 页面加载时重新验证缓存的 Token
  async function revalidate() {
    if (sessionStorage.getItem(UNLOCKED_KEY) === 'true' && getAdminToken()) {
      const ok = await validate(getAdminToken())
      if (ok) {
        adminToken.value = getAdminToken()
        isUnlocked.value = true
      } else {
        logout()
      }
    }
  }

  // 全局按键监听（Escape 可由父组件用于关闭弹窗等）
  onBeforeUnmount(() => {
    // 无需在此清理，由调用方按需管理
  })

  return {
    isUnlocked,
    adminToken,
    unlock,
    saveToken,
    logout,
    revalidate,
  }
}
