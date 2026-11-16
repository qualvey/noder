import { ref, type Ref } from 'vue'

export interface MosaicStroke {
  /** 笔刷足迹（预览像素） */
  size: number
  /** 描边创建时的预览画布宽度，用于换算回原图坐标系 */
  previewWidth: number
  /** 每笔包含的方格数（N×N） */
  cellsPerBrush: number
  /** 归一化坐标（0~1） */
  points: Array<{ x: number; y: number }>
}

// 复用的 1x1 采样画布（避免涂抹时高频创建临时 canvas 造成内存压力）
const mosaicPatchCanvas = document.createElement('canvas')
const mosaicPatchCtx = mosaicPatchCanvas.getContext('2d')!

/**
 * 马赛克涂抹引擎：笔刷描边、图层合成、预览覆盖层、笔刷手势、撤回。
 * 画布由调用方持有；矩形标记等其它图层通过 renderExtra 叠加；
 * 合成（generateWatermark）完成后由 regenerate 通知调用方。
 */
export function useMosaicEngine(opts: {
  canvas: Ref<HTMLCanvasElement | null>
  sourceImage: Ref<HTMLImageElement | null>
  isActive: () => boolean
  renderExtra?: (ctx: CanvasRenderingContext2D, outWidth: number, outHeight: number) => void
  regenerate: () => void
}) {
  const strokes = ref<MosaicStroke[]>([])
  const brushSize = ref(24)
  const cellsPerBrush = ref(4) // 每笔方格密度 N×N（默认 4×4，主流马赛克观感）
  const brushPosition = ref<{ x: number; y: number } | null>(null)
  const isPainting = ref(false)
  const isResizingBrush = ref(false)
  const resizeStart = ref<{ x: number; y: number; size: number; density: number } | null>(null)

  // 独立马赛克图层：与原图同分辨率，合成时位于水印文字之下
  let mosaicLayer: HTMLCanvasElement | null = null
  // 最近一次描边创建时间（双击进全屏时用于清理单击污点）
  let lastStrokePaintedAt = 0

  function getSourceSize() {
    const img = opts.sourceImage.value
    if (!img) return { w: 0, h: 0 }
    return { w: img.naturalWidth || img.width, h: img.naturalHeight || img.height }
  }

  // 初始化/重置独立马赛克图层（与原图同分辨率）
  function initMosaicLayer() {
    const { w, h } = getSourceSize()
    if (w <= 0 || h <= 0) return
    if (!mosaicLayer) {
      mosaicLayer = document.createElement('canvas')
    }
    mosaicLayer.width = w
    mosaicLayer.height = h
    mosaicLayer.getContext('2d')?.clearRect(0, 0, w, h)
  }

  function getCanvasSize() {
    const canvas = opts.canvas.value
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const width = Math.max(1, Math.round(rect.width))
    const height = Math.max(1, Math.round(rect.height))
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width
      canvas.height = height
      renderOverlay()
    }
  }

  function clearOverlay() {
    const canvas = opts.canvas.value
    canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
  }

  // 把源图一块区域的平均色绘制成一个实心马赛克方格
  // 采样 1x1 平均色 -> 关闭平滑放大为 cell×cell 实心方格
  function stampMosaicCell(
    ctx: CanvasRenderingContext2D,
    src: CanvasImageSource,
    cx: number,
    cy: number,
    cell: number,
  ) {
    const img = src as HTMLImageElement
    if (cell <= 0 || !img || !img.naturalWidth || !img.naturalHeight) return
    const sx = Math.max(0, cx - cell / 2)
    const sy = Math.max(0, cy - cell / 2)
    const sw = Math.min(cell, img.naturalWidth - sx)
    const sh = Math.min(cell, img.naturalHeight - sy)
    if (sw <= 0 || sh <= 0) return

    if (!mosaicPatchCtx) return
    mosaicPatchCanvas.width = 1
    mosaicPatchCanvas.height = 1
    mosaicPatchCtx.imageSmoothingEnabled = true // 采样 1x1 取区域平均色
    mosaicPatchCtx.drawImage(img, sx, sy, sw, sh, 0, 0, 1, 1)

    ctx.imageSmoothingEnabled = false
    ctx.drawImage(mosaicPatchCanvas, 0, 0, 1, 1, sx, sy, cell, cell)
  }

  // 将描边的一个点绘制到马赛克图层（原图坐标系）
  // 每笔按 N×N 方格网格铺满笔刷足迹：方格小而密，模糊到位且多格可见
  function paintPointToLayer(stroke: MosaicStroke, px: number, py: number) {
    const img = opts.sourceImage.value
    const layer = mosaicLayer
    const { w, h } = getSourceSize()
    if (!img || !layer || !w || !h) return

    const ctx = layer.getContext('2d')
    if (!ctx) return

    const foot = stroke.size * (w / stroke.previewWidth) // 笔刷足迹（原图像素）
    const n = Math.max(1, Math.round(stroke.cellsPerBrush))
    const cell = foot / n // 单个方格边长（原图像素）
    const cx = px * w
    const cy = py * h
    const x0 = cx - foot / 2
    const y0 = cy - foot / 2
    // 足迹覆盖的网格范围（对齐全局网格，边缘不留半格）
    const gx0 = Math.floor(x0 / cell)
    const gy0 = Math.floor(y0 / cell)
    const gx1 = Math.floor((x0 + foot) / cell)
    const gy1 = Math.floor((y0 + foot) / cell)
    for (let gy = gy0; gy <= gy1; gy++) {
      for (let gx = gx0; gx <= gx1; gx++) {
        stampMosaicCell(ctx, img, gx * cell + cell / 2, gy * cell + cell / 2, cell)
      }
    }
  }

  // 将整条描边绘制到马赛克图层（用于撤回后的全量重绘）
  function paintStrokeToLayer(stroke: MosaicStroke) {
    for (const p of stroke.points) {
      if (p) paintPointToLayer(stroke, p.x, p.y)
    }
  }

  // 全量重绘马赛克图层（撤回/清空后重建）
  function redrawMosaicLayer() {
    const layer = mosaicLayer
    if (!layer) return
    const ctx = layer.getContext('2d')
    if (!ctx) return
    ctx.clearRect(0, 0, layer.width, layer.height)
    for (const stroke of strokes.value) {
      paintStrokeToLayer(stroke)
    }
  }

  // 马赛克方格边缘轻微柔化，呈现主流马赛克观感（模糊的方格）
  // outWidth 为绘制目标画布宽度，blur 半径按输出像素中的方格边长计算，预览/导出观感一致
  function getMosaicBlurPx(outWidth: number): number {
    const last = strokes.value[strokes.value.length - 1]
    if (!last) return 0
    const { w } = getSourceSize()
    if (!w) return 0
    const n = Math.max(1, Math.round(last.cellsPerBrush))
    const cell = (last.size * (w / last.previewWidth)) / n // 单格边长（原图像素）
    const cellOut = Math.max(1, cell * (outWidth / w)) // 输出画布像素边长
    return Math.min(3, Math.max(1, Math.round(cellOut * 0.1)))
  }

  // 预览覆盖层：把马赛克图层缩放显示到预览画布（涂抹时的实时反馈）
  function renderOverlay() {
    const canvas = opts.canvas.value
    if (!canvas) return

    getCanvasSize()
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    ctx.clearRect(0, 0, canvas.width, canvas.height)
    if (mosaicLayer && opts.sourceImage.value && opts.isActive()) {
      ctx.imageSmoothingEnabled = false
      ctx.filter = `blur(${getMosaicBlurPx(canvas.width)}px)`
      ctx.drawImage(mosaicLayer, 0, 0, canvas.width, canvas.height)
      ctx.filter = 'none'
    }

    opts.renderExtra?.(ctx, canvas.width, canvas.height)
  }

  // 涂抹结束后把马赛克图层叠加进合成图（原图 -> 马赛克 -> 水印）
  function applyToCanvas(canvas: HTMLCanvasElement, ctx: CanvasRenderingContext2D) {
    if (!mosaicLayer || !opts.sourceImage.value) return
    ctx.save()
    ctx.imageSmoothingEnabled = false
    const blurPx = getMosaicBlurPx(canvas.width)
    if (blurPx > 0) {
      try {
        ctx.filter = `blur(${blurPx}px)`
      } catch {
        // 不支持 filter 时降级为无柔化（硬边方格）
      }
    }
    ctx.drawImage(mosaicLayer, 0, 0)
    ctx.restore()
  }

  // 换图/清空：丢弃全部描边并清空图层
  function reset() {
    strokes.value = []
    isPainting.value = false
    brushPosition.value = null
    mosaicLayer?.getContext('2d')?.clearRect(0, 0, mosaicLayer.width, mosaicLayer.height)
  }

  // 确保马赛克图层与目标画布同分辨率（换图/导出时重建）
  function ensureLayer(width: number, height: number) {
    if (!mosaicLayer || mosaicLayer.width !== width || mosaicLayer.height !== height) {
      initMosaicLayer()
    }
  }

  // 重建图层并重绘覆盖层（撤回/污点清理后调用，调用方负责重新合成）
  function repaint() {
    redrawMosaicLayer()
    renderOverlay()
  }

  // 双击进全屏时清理第一击留下的单击污点（无轨迹单点描边），返回是否清理过
  function popAccidentalTap(maxAgeMs = 800): boolean {
    const last = strokes.value[strokes.value.length - 1]
    if (last && Date.now() - lastStrokePaintedAt < maxAgeMs && last.points.length <= 2) {
      strokes.value.pop()
      return true
    }
    return false
  }

  function updatePointerPosition(event: PointerEvent) {
    const canvas = opts.canvas.value
    if (!canvas || !opts.isActive()) {
      brushPosition.value = null
      return
    }
    const rect = canvas.getBoundingClientRect()
    const x = event.clientX - rect.left
    const y = event.clientY - rect.top
    brushPosition.value = {
      x: Math.min(Math.max(x, 0), rect.width),
      y: Math.min(Math.max(y, 0), rect.height),
    }
  }

  // Ctrl + 右键按下：进入笔刷调整模式（横向=大小，纵向=密度），不画点
  function startBrushResize(event: PointerEvent) {
    const canvas = opts.canvas.value
    if (!opts.isActive() || !canvas) return false
    if (event.button !== 2 || !event.ctrlKey) return false
    event.preventDefault()
    isResizingBrush.value = true
    resizeStart.value = {
      x: event.clientX,
      y: event.clientY,
      size: brushSize.value,
      density: cellsPerBrush.value,
    }
    updatePointerPosition(event) // 按下即显示调整提示框
    try {
      canvas.setPointerCapture(event.pointerId) // 拖出画布仍持续接收事件
    } catch {
      /* 合成事件等场景不支持捕获时忽略 */
    }
    return true
  }

  // 拖拽中：实时更新笔刷大小与方格密度（带灵敏度与范围钳制）
  function moveBrushResize(event: PointerEvent) {
    if (!isResizingBrush.value || !resizeStart.value) return
    updatePointerPosition(event)
    const s = resizeStart.value
    const dx = event.clientX - s.x
    const dy = event.clientY - s.y
    brushSize.value = Math.min(80, Math.max(8, Math.round(s.size + dx * 0.5)))
    cellsPerBrush.value = Math.min(8, Math.max(1, Math.round(s.density + dy * 0.05)))
  }

  function endBrushResize(event: PointerEvent) {
    if (!isResizingBrush.value) return
    isResizingBrush.value = false
    resizeStart.value = null
    const canvas = opts.canvas.value
    if (canvas && canvas.hasPointerCapture(event.pointerId)) {
      canvas.releasePointerCapture(event.pointerId)
    }
  }

  function startPaint(event: PointerEvent) {
    if (!opts.isActive() || !opts.canvas.value) return
    // 仅左键涂抹；双击的第二击（detail>1）不画点：避免双击进全屏时留下污点
    if (event.button !== 0 || event.detail > 1) return

    getCanvasSize()
    updatePointerPosition(event)

    const rect = opts.canvas.value.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rect.width
    const y = (event.clientY - rect.top) / rect.height

    const stroke: MosaicStroke = {
      size: brushSize.value,
      previewWidth: Math.max(1, opts.canvas.value.width),
      cellsPerBrush: cellsPerBrush.value,
      points: [{ x: Math.min(Math.max(x, 0), 1), y: Math.min(Math.max(y, 0), 1) }],
    }

    strokes.value.push(stroke)
    lastStrokePaintedAt = Date.now()
    isPainting.value = true
    paintPointToLayer(stroke, stroke.points[0]?.x ?? 0, stroke.points[0]?.y ?? 0)
    renderOverlay()
  }

  function movePaint(event: PointerEvent) {
    updatePointerPosition(event)

    if (!isPainting.value || !opts.canvas.value) return

    const rect = opts.canvas.value.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rect.width
    const y = (event.clientY - rect.top) / rect.height
    const currentStroke = strokes.value[strokes.value.length - 1]

    if (!currentStroke) return

    const point = { x: Math.min(Math.max(x, 0), 1), y: Math.min(Math.max(y, 0), 1) }
    const last = currentStroke.points[currentStroke.points.length - 1]
    // 两点之间线性插值：快速滑动时描边连续，方格不留缝隙，原图轮廓不残留
    if (last) {
      const { w } = getSourceSize()
      const foot = currentStroke.size * (w / currentStroke.previewWidth) // 笔刷足迹（原图像素）
      const step = Math.max(foot / w, 1e-4) / 2 // 归一化步长 = 半足迹，确保相邻足迹重叠、不露原图
      const dist = Math.hypot(point.x - last.x, point.y - last.y)
      const n = Math.max(1, Math.ceil(dist / step))
      for (let i = 1; i <= n; i++) {
        const t = i / n
        const p = {
          x: last.x + (point.x - last.x) * t,
          y: last.y + (point.y - last.y) * t,
        }
        currentStroke.points.push(p)
        paintPointToLayer(currentStroke, p.x, p.y)
      }
    } else {
      currentStroke.points.push(point)
      paintPointToLayer(currentStroke, point.x, point.y)
    }
    renderOverlay()
  }

  function stopPaint() {
    isPainting.value = false
    brushPosition.value = null
    // 一笔结束：把马赛克图层合并进水印合成图（原图 -> 马赛克 -> 水印）
    if (strokes.value.length > 0 && opts.sourceImage.value) {
      opts.regenerate()
    }
  }

  // 撤回上一步：移除最后一条描边，重建马赛克图层并重新合成
  function undo() {
    if (!strokes.value.length) return
    strokes.value.pop()
    redrawMosaicLayer()
    renderOverlay()
    opts.regenerate()
  }

  function clear() {
    strokes.value = []
    redrawMosaicLayer()
    renderOverlay()
    opts.regenerate()
  }

  // Ctrl/Cmd+Z 触发马赛克撤回；焦点在输入框/文本域时放行（不干扰文本撤销）
  function handleUndoKeydown(e: KeyboardEvent) {
    if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 'z' || e.shiftKey) return
    const target = e.target as HTMLElement | null
    const isTextTarget =
      target instanceof HTMLInputElement ||
      target instanceof HTMLTextAreaElement ||
      !!target?.isContentEditable
    if (isTextTarget) return
    if (!strokes.value.length) return
    e.preventDefault()
    undo()
  }

  return {
    strokes,
    brushSize,
    cellsPerBrush,
    brushPosition,
    isResizingBrush,
    initMosaicLayer,
    getCanvasSize,
    clearOverlay,
    renderOverlay,
    applyToCanvas,
    ensureLayer,
    reset,
    repaint,
    popAccidentalTap,
    updatePointerPosition,
    startBrushResize,
    moveBrushResize,
    endBrushResize,
    startPaint,
    movePaint,
    stopPaint,
    undo,
    clear,
    handleUndoKeydown,
  }
}
