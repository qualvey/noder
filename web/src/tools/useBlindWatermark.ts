import { ref, watch, type Ref } from 'vue'
import { embedBlindWatermark, extractBlindWatermark } from './blindWatermark'

export interface BlindWatermarkNotify {
  success: (message: string) => void
  warning: (message: string) => void
  error: (message: string) => void
}

/**
 * 盲水印（不可见水印）：嵌入参数状态、在最终合成像素上就地写入、
 * 从任意图片源真盲提取。嵌入参数变化时通过 onEmbedChange 通知
 * 调用方重跑合成；与可见水印/马赛克逻辑解耦。
 */
export function useBlindWatermark(opts: {
  initialText: string
  bits: number
  watermarkedImageSrc: Ref<string>
  notify: BlindWatermarkNotify
  onEmbedChange: () => void
}) {
  const blindEnabled = ref(false)
  const blindText = ref(opts.initialText)
  const blindBits = ref(opts.bits)
  const extractedText = ref<string | null>(null)

  // 盲水印嵌入：在最终合成像素上就地写入（PNG 无损导出后可提取）
  function applyBlindWatermark(canvas: HTMLCanvasElement, ctx: CanvasRenderingContext2D) {
    if (!blindEnabled.value) return
    const text = blindText.value.trim()
    if (!text) return
    const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height)
    const ok = embedBlindWatermark(imgData, text, { bits: blindBits.value })
    if (ok) {
      ctx.putImageData(imgData, 0, 0)
    } else {
      opts.notify.warning('图片尺寸过小，无法嵌入盲水印')
    }
  }

  // 从图片源提取盲水印（真盲提取，不需要原图）
  async function extractBlindFromSource(src: string): Promise<string | null> {
    const img = new Image()
    await new Promise<void>((resolve, reject) => {
      img.onload = () => resolve()
      img.onerror = () => reject(new Error('图片解码失败'))
      img.src = src
    })
    const canvas = document.createElement('canvas')
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx) return null
    ctx.drawImage(img, 0, 0)
    const data = ctx.getImageData(0, 0, canvas.width, canvas.height)
    return extractBlindWatermark(data, { bits: blindBits.value })
  }

  async function runExtract(src: string) {
    try {
      const text = await extractBlindFromSource(src)
      extractedText.value = text
      if (text) opts.notify.success('盲水印提取成功')
      else opts.notify.warning('未检测到盲水印（或已被有损压缩破坏）')
    } catch {
      opts.notify.error('提取失败：图片解码异常')
    }
  }

  // 从当前预览图提取
  function extractFromPreview() {
    if (!opts.watermarkedImageSrc.value) return
    runExtract(opts.watermarkedImageSrc.value)
  }

  // 从外部文件提取（由设置面板透出 File 对象）
  function onExtractUpload(file: File) {
    const reader = new FileReader()
    reader.onload = () => runExtract(reader.result as string)
    reader.readAsDataURL(file)
  }

  watch([blindEnabled, blindText, blindBits], () => {
    opts.onEmbedChange()
  })

  return {
    blindEnabled,
    blindText,
    blindBits,
    extractedText,
    applyBlindWatermark,
    extractFromPreview,
    onExtractUpload,
  }
}
