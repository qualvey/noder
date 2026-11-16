import { ref, type Ref } from 'vue'

export interface RectangleMark {
  x: number
  y: number
  width: number
  height: number
}

function normalizeRectangle(start: { x: number; y: number }, end: { x: number; y: number }) {
  return {
    x: Math.min(start.x, end.x),
    y: Math.min(start.y, end.y),
    width: Math.abs(end.x - start.x),
    height: Math.abs(end.y - start.y),
  }
}

/**
 * 矩形标记工具：框选绘制、拖拽实时预览、撤销/清空。
 * 绘制落点与预览刷新通过回调交还调用方（覆盖层重绘 + 水印重合成），
 * 与马赛克引擎、文件输入逻辑解耦。
 */
export function useRectangleTool(
  canvasRef: Ref<HTMLCanvasElement | null>,
  opts: {
    renderOverlay: () => void
    regenerate: () => void
  },
) {
  const rectangleColor = ref('#ef4444')
  const rectangleWidth = ref(4)
  const isDrawingRectangle = ref(false)
  const rectangleStart = ref<{ x: number; y: number } | null>(null)
  const rectanglePreview = ref<RectangleMark | null>(null)
  const rectangleMarks = ref<RectangleMark[]>([])

  function getCanvasPoint(event: PointerEvent) {
    const canvas = canvasRef.value
    if (!canvas) return null
    const rect = canvas.getBoundingClientRect()
    return {
      x: Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)),
      y: Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height)),
    }
  }

  function startRectangle(event: PointerEvent) {
    if (event.button !== 0 || !canvasRef.value) return
    const point = getCanvasPoint(event)
    if (!point) return
    event.preventDefault()
    rectangleStart.value = point
    rectanglePreview.value = { x: point.x, y: point.y, width: 0, height: 0 }
    isDrawingRectangle.value = true
    canvasRef.value.setPointerCapture(event.pointerId)
    opts.renderOverlay()
  }

  function moveRectangle(event: PointerEvent) {
    if (!isDrawingRectangle.value || !rectangleStart.value) return
    const point = getCanvasPoint(event)
    if (!point) return
    rectanglePreview.value = normalizeRectangle(rectangleStart.value, point)
    opts.renderOverlay()
  }

  function finishRectangle(event: PointerEvent) {
    if (!isDrawingRectangle.value || !rectangleStart.value) return
    const point = getCanvasPoint(event)
    const mark = point ? normalizeRectangle(rectangleStart.value, point) : null
    if (mark && mark.width > 0.005 && mark.height > 0.005) rectangleMarks.value.push(mark)
    isDrawingRectangle.value = false
    rectangleStart.value = null
    rectanglePreview.value = null
    if (canvasRef.value?.hasPointerCapture(event.pointerId)) {
      canvasRef.value.releasePointerCapture(event.pointerId)
    }
    opts.renderOverlay()
    opts.regenerate()
  }

  function cancelRectangle(event: PointerEvent) {
    isDrawingRectangle.value = false
    rectangleStart.value = null
    rectanglePreview.value = null
    if (canvasRef.value?.hasPointerCapture(event.pointerId)) {
      canvasRef.value.releasePointerCapture(event.pointerId)
    }
    opts.renderOverlay()
  }

  function undoRectangle() {
    if (!rectangleMarks.value.length) return
    rectangleMarks.value.pop()
    opts.renderOverlay()
    opts.regenerate()
  }

  function clearRectangles() {
    rectangleMarks.value = []
    opts.renderOverlay()
    opts.regenerate()
  }

  function resetPreview() {
    rectanglePreview.value = null
  }

  // 标记图层使用预览坐标绘制，保证拖拽时即时反馈。
  function renderMarks(ctx: CanvasRenderingContext2D, outWidth: number, outHeight: number) {
    if (!rectangleMarks.value.length && !rectanglePreview.value) return
    ctx.strokeStyle = rectangleColor.value
    ctx.lineWidth = rectangleWidth.value
    for (const mark of rectangleMarks.value) {
      ctx.strokeRect(mark.x * outWidth, mark.y * outHeight, mark.width * outWidth, mark.height * outHeight)
    }
    if (rectanglePreview.value) {
      const mark = rectanglePreview.value
      ctx.strokeRect(mark.x * outWidth, mark.y * outHeight, mark.width * outWidth, mark.height * outHeight)
    }
  }

  return {
    rectangleColor,
    rectangleWidth,
    isDrawingRectangle,
    rectanglePreview,
    rectangleMarks,
    startRectangle,
    moveRectangle,
    finishRectangle,
    cancelRectangle,
    undoRectangle,
    clearRectangles,
    resetPreview,
    renderMarks,
  }
}
