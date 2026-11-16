import { nextTick, onBeforeUnmount, ref, watch, type Ref } from 'vue'

/**
 * 预览容器内图片保持原始比例：按容器宽高计算最大可放入的适配尺寸。
 * 换图 / 容器尺寸变化（含全屏进出）时自动重算，卸载时断开 observer。
 */
export function usePreviewBox(
  originalImage: Ref<HTMLImageElement | null>,
  previewContainer: Ref<HTMLElement | null>,
) {
  const previewBox = ref<{ w: number; h: number } | null>(null)
  let previewObserver: ResizeObserver | null = null

  function updatePreviewBox() {
    const img = originalImage.value
    const el = previewContainer.value
    if (!img || !el || !img.naturalWidth || !img.naturalHeight) {
      previewBox.value = null
      return
    }
    const cs = getComputedStyle(el)
    const padX = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight)
    const padY = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom)
    const cw = Math.max(1, el.clientWidth - padX)
    const ch = Math.max(1, el.clientHeight - padY)
    const scale = Math.min(cw / img.naturalWidth, ch / img.naturalHeight)
    previewBox.value = {
      w: Math.max(1, Math.round(img.naturalWidth * scale)),
      h: Math.max(1, Math.round(img.naturalHeight * scale)),
    }
  }

  function ensurePreviewObserver() {
    if (previewObserver || !previewContainer.value) return
    previewObserver = new ResizeObserver(() => updatePreviewBox())
    previewObserver.observe(previewContainer.value)
  }

  // 换图 / 容器尺寸变化（含全屏进出）时重算适配尺寸
  watch(originalImage, async () => {
    await nextTick()
    updatePreviewBox()
    ensurePreviewObserver()
  })

  onBeforeUnmount(() => {
    previewObserver?.disconnect()
    previewObserver = null
  })

  return { previewBox, updatePreviewBox }
}
