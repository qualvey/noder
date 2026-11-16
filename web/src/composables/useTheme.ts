import { onBeforeUnmount, ref, watch } from 'vue'

export type Theme = 'light' | 'dark'

const THEME_KEY = 'noder_theme'
const DATA_ATTR = 'data-theme'

/**
 * 主题管理：统一处理 localStorage、data-theme、系统偏好。
 * 初始值应在 mount 前由 main.ts 设置（避免闪烁），此处负责运行时切换与持久化。
 */
export function useTheme() {
  const savedTheme = localStorage.getItem(THEME_KEY)
  const systemLight = window.matchMedia?.('(prefers-color-scheme: light)').matches
  const initialTheme: Theme =
    (savedTheme === 'light' || savedTheme === 'dark' ? savedTheme : systemLight ? 'light' : 'dark')

  const theme = ref<Theme>(initialTheme)

  function applyTheme(next: Theme) {
    theme.value = next
    document.documentElement.dataset[DATA_ATTR] = next
    localStorage.setItem(THEME_KEY, next)
  }

  function toggleTheme() {
    applyTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  // 同步 system 偏好变更（若用户未手动指定主题）
  const mediaQuery = window.matchMedia?.('(prefers-color-scheme: light)')
  if (mediaQuery && !savedTheme) {
    const handleChange = (e: MediaQueryListEvent) => {
      applyTheme(e.matches ? 'light' : 'dark')
    }
    mediaQuery.addEventListener('change', handleChange)
    onBeforeUnmount(() => mediaQuery.removeEventListener('change', handleChange))
  }

  // 持久化 theme 变更
  watch(theme, (next) => {
    applyTheme(next)
  })

  return { theme, toggleTheme, applyTheme }
}
