import { onBeforeUnmount, ref } from 'vue'

export type ToastType = 'info' | 'error' | 'success' | 'warning'

export interface ToastItem {
  id: number
  message: string
  type: ToastType
}

/**
 * 全局 Toast 状态管理。
 * 在根组件（App.vue）中初始化一次，通过 provide 注入给全应用。
 * 定时器在组件卸载时统一清理，避免内存泄漏。
 */
export function useToast() {
  const toasts = ref<ToastItem[]>([])
  let toastId = 0
  const timers = new Map<number, ReturnType<typeof setTimeout>>()

  function showToast(message: string, type: ToastType = 'info') {
    const id = ++toastId
    toasts.value.push({ id, message, type })

    const timer = setTimeout(() => {
      removeToast(id)
      timers.delete(id)
    }, 3000)
    timers.set(id, timer)
  }

  function removeToast(id: number) {
    const timer = timers.get(id)
    if (timer) {
      clearTimeout(timer)
      timers.delete(id)
    }
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  function clearAll() {
    timers.forEach((timer) => clearTimeout(timer))
    timers.clear()
    toasts.value = []
  }

  onBeforeUnmount(() => {
    clearAll()
  })

  return {
    toasts,
    showToast,
    removeToast,
    clearAll,
  }
}
