import { nextTick, ref } from 'vue'

export interface PopoverState {
  title: string
  x: number
  y: number
  onConfirm: () => void
}

/**
 * 鼠标位置删除确认弹窗状态。
 * 定位逻辑保持简单：紧贴触发按钮上方/下方，视口边缘防溢出。
 * 精确的按钮中心对齐应由消费组件在 nextTick 后根据实际 DOM 测量完成。
 */
export function usePopover() {
  const popover = ref<PopoverState | null>(null)

  function showPopover(targetEl: Element, title: string, onConfirm: () => void) {
    const el = (targetEl as HTMLElement).closest?.('button, .btn, .icon-btn') || targetEl
    const rect = el.getBoundingClientRect()
    const targetCenterX = rect.left + rect.width / 2

    const gap = 4
    const estWidth = 230
    const estHeight = 82

    let initLeft = Math.round(targetCenterX - estWidth / 2)
    let initTop = Math.round(rect.top - estHeight - gap)
    if (initTop < 10) {
      initTop = Math.round(rect.bottom + gap)
    }
    if (initLeft < 10) initLeft = 10
    if (initLeft + estWidth > window.innerWidth - 10) initLeft = window.innerWidth - estWidth - 10

    popover.value = { title, x: initLeft, y: initTop, onConfirm }

    nextTick().then(() => {
      if (!popover.value || popover.value.title !== title) return
      // 允许消费组件在下一次 tick 进行精确定位校正
    })
  }

  function hidePopover() {
    popover.value = null
  }

  function confirmPopover() {
    const p = popover.value
    hidePopover()
    if (p) p.onConfirm()
  }

  return { popover, showPopover, hidePopover, confirmPopover }
}
