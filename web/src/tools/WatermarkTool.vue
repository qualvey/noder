<script setup lang="ts">
import { ref, watch, nextTick, onUnmounted, inject } from 'vue'
import { embedBlindWatermark, extractBlindWatermark } from './blindWatermark'

const toast = inject<(message: string, type?: 'info' | 'error' | 'success' | 'warning') => void>('toast')
const ElMessage = {
  success: (message: string) => toast?.(message, 'success'),
  error: (message: string) => toast?.(message, 'error'),
  warning: (message: string) => toast?.(message, 'warning'),
}

const originalImage = ref<HTMLImageElement | null>(null)
const originalFileName = ref<string>('')
const watermarkedImageSrc = ref<string>('')
const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const previewContainer = ref<HTMLElement | null>(null)
const isPreviewExpanded = ref(false)
const mosaicCanvas = ref<HTMLCanvasElement | null>(null)
const isMosaicMode = ref(false)
const isPaintingMosaic = ref(false)
// PS 式笔刷调整：Ctrl + 右键拖拽（横向=笔刷大小，纵向=方格密度）
const isResizingBrush = ref(false)
const brushResizeStart = ref<{ x: number; y: number; size: number; density: number } | null>(null)
const mosaicBrushSize = ref(24)
const mosaicCellsPerBrush = ref(4) // 每笔方格密度 N×N（默认 4×4，主流马赛克观感）
const mosaicBrushColor = ref('#111827')
const mosaicBrushPosition = ref<{ x: number; y: number } | null>(null)

// 描边：points 为归一化坐标（0~1），size 为笔刷足迹（预览像素），previewWidth 用于换算回原图坐标系
// cellsPerBrush：每笔包含的方格数（N×N），足迹内为方格网格
interface MosaicStroke {
  size: number
  previewWidth: number
  cellsPerBrush: number
  points: Array<{ x: number; y: number }>
}

const mosaicStrokes = ref<MosaicStroke[]>([])
// 独立马赛克图层：与原图同分辨率，合成时位于水印文字之下（涂抹区域水印仍清晰可见）
let mosaicLayer: HTMLCanvasElement | null = null
// 复用的 8x8 采样画布（避免涂抹时高频创建临时 canvas 造成内存压力/崩溃）
const mosaicPatchCanvas = document.createElement('canvas')
const mosaicPatchCtx = mosaicPatchCanvas.getContext('2d')!
// 预览渲染节流（每帧最多重绘一次，避免高频 pointermove 拖垮浏览器）
let mosaicRenderRaf = 0
// 最近一次描边创建时间（双击进全屏时用于清理单击污点）
let lastStrokePaintedAt = 0

// 快速预设文字
const presets = [
  'www.ryugo.org',
  '仅供实名认证，他用无效',
]

// 水印参数
const watermarkText = ref<string>(presets[0] || "www.ryugo.org")

// 盲水印（不可见水印）
const blindEnabled = ref(false)
const blindText = ref(watermarkText.value) // 默认复用可见水印文本，可单独改
const blindBits = ref(1) // 嵌入强度：蓝通道最低 1~2 位
const extractedText = ref<string | null>(null)
const extractFileInput = ref<HTMLInputElement | null>(null)

const watermarkEnabled = ref(true)
const fontSize = ref<number>(47)
const opacity = ref<number>(0.55)
const angle = ref<number>(30)
const textColor = ref('#64748b') // 默认 Slate 500
const gapX = ref(180)
const gapY = ref(180)
const fontStyle = ref<'normal' | 'bold' | 'italic'>('normal')

const handlePresetClick = (text: string) => {
  watermarkText.value = text
}

// 颜色预设
const colorPresets = [
  { name: 'Slate', value: '#64748b' },
  { name: 'Red', value: '#ef4444' },
  { name: 'Blue', value: '#3b82f6' },
  { name: 'White', value: '#ffffff' },
  { name: 'Black', value: '#000000' },
]

// 处理拖拽事件
const onDragOver = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = true
}

const onDragLeave = () => {
  isDragging.value = false
}

const onDrop = (e: DragEvent) => {
  e.preventDefault()
  isDragging.value = false
  const files = e.dataTransfer?.files
  if (files && files.length > 0 && files[0]) {
    processFile(files[0])
  }
}

// 图片区域支持 Ctrl+V / 右键粘贴图片
// 规则：剪贴板含图片时接管；若焦点在文本框且剪贴板同时含文本，则放行让文本框正常粘贴文本
const onPasteImage = (e: ClipboardEvent) => {
  const items = e.clipboardData?.items
  if (!items) return
  const imageItem = Array.from(items).find((it) => it.type.startsWith('image/'))
  if (!imageItem) return
  const target = e.target as HTMLElement | null
  const isTextTarget =
    target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement
  if (isTextTarget && e.clipboardData && e.clipboardData.getData('text/plain')) return
  const file = imageItem.getAsFile()
  if (!file) return
  e.preventDefault()
  processFile(file)
}

const triggerFileSelect = () => {
  fileInput.value?.click()
}

const updateMosaicPointerPosition = (event: PointerEvent) => {
  const canvas = mosaicCanvas.value
  if (!canvas || !isMosaicMode.value) {
    mosaicBrushPosition.value = null
    return
  }

  const rect = canvas.getBoundingClientRect()
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top

  mosaicBrushPosition.value = {
    x: Math.min(Math.max(x, 0), rect.width),
    y: Math.min(Math.max(y, 0), rect.height),
  }
}

const getMosaicCanvasSize = () => {
  const canvas = mosaicCanvas.value
  if (!canvas) return

  const rect = canvas.getBoundingClientRect()
  const width = Math.max(1, Math.round(rect.width))
  const height = Math.max(1, Math.round(rect.height))

  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width
    canvas.height = height
    renderMosaicOverlay()
  }
}

// 获取原图分辨率
const getSourceSize = () => {
  const img = originalImage.value
  if (!img) return { w: 0, h: 0 }
  return { w: img.naturalWidth || img.width, h: img.naturalHeight || img.height }
}

// 预览容器内图片保持原始比例：按容器宽高计算最大可放入的适配尺寸（宽高同时约束）
const previewBox = ref<{ w: number; h: number } | null>(null)
let previewObserver: ResizeObserver | null = null

const updatePreviewBox = () => {
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

const ensurePreviewObserver = () => {
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

// 初始化/重置独立马赛克图层（与原图同分辨率）
const initMosaicLayer = () => {
  const { w, h } = getSourceSize()
  if (w <= 0 || h <= 0) return
  if (!mosaicLayer) {
    mosaicLayer = document.createElement('canvas')
  }
  mosaicLayer.width = w
  mosaicLayer.height = h
  mosaicLayer.getContext('2d')?.clearRect(0, 0, w, h)
}

// 把源图一块区域的平均色绘制成一个实心马赛克方格（主流马赛克：清晰大方格）
// 采样 1x1 平均色 -> 关闭平滑放大为 cell×cell 实心方格
// 复用模块级 patch 画布，避免高频涂抹时反复创建临时 canvas
const stampMosaicCell = (
  ctx: CanvasRenderingContext2D,
  src: CanvasImageSource,
  cx: number,
  cy: number,
  cell: number,
) => {
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
// 采样源 = 原图：马赛克只遮挡图片内容，水印文字独立绘制在最上层，不受影响
// 每笔按 N×N 方格网格铺满笔刷足迹：方格小而密，模糊到位且多格可见
const paintPointToLayer = (stroke: MosaicStroke, px: number, py: number) => {
  const img = originalImage.value
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
const paintStrokeToLayer = (stroke: MosaicStroke) => {
  for (const p of stroke.points) {
    if (p) paintPointToLayer(stroke, p.x, p.y)
  }
}

// 全量重绘马赛克图层（撤回/清空后重建）
const redrawMosaicLayer = () => {
  const layer = mosaicLayer
  if (!layer) return
  const ctx = layer.getContext('2d')
  if (!ctx) return
  ctx.clearRect(0, 0, layer.width, layer.height)
  for (const stroke of mosaicStrokes.value) {
    paintStrokeToLayer(stroke)
  }
}

// 马赛克方格边缘轻微柔化，呈现主流马赛克观感（模糊的方格）
// outWidth 为绘制目标画布宽度，blur 半径按输出像素中的方格边长计算，预览/导出观感一致
const getMosaicBlurPx = (outWidth: number): number => {
  const last = mosaicStrokes.value[mosaicStrokes.value.length - 1]
  if (!last) return 0
  const { w } = getSourceSize()
  if (!w) return 0
  const n = Math.max(1, Math.round(last.cellsPerBrush))
  const cell = (last.size * (w / last.previewWidth)) / n // 单格边长（原图像素）
  const cellOut = Math.max(1, cell * (outWidth / w)) // 输出画布像素边长
  return Math.min(3, Math.max(1, Math.round(cellOut * 0.1)))
}

// 预览覆盖层：把马赛克图层缩放显示到预览画布（涂抹时的实时反馈）
const renderMosaicOverlay = () => {
  const canvas = mosaicCanvas.value
  if (!canvas) return

  getMosaicCanvasSize()
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  ctx.clearRect(0, 0, canvas.width, canvas.height)
  if (!mosaicLayer || !originalImage.value || !isMosaicMode.value) return

  ctx.imageSmoothingEnabled = false
  ctx.filter = `blur(${getMosaicBlurPx(canvas.width)}px)`
  ctx.drawImage(mosaicLayer, 0, 0, canvas.width, canvas.height)
  ctx.filter = 'none'
}

// 涂抹结束后把马赛克图层叠加进合成图（原图 -> 马赛克 -> 水印）
const applyMosaicToCanvas = (canvas: HTMLCanvasElement, ctx: CanvasRenderingContext2D) => {
  if (!mosaicLayer || !originalImage.value) return
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

const updatePreviewState = () => {
  if (!previewContainer.value) {
    isPreviewExpanded.value = false
    return
  }

  isPreviewExpanded.value = document.fullscreenElement === previewContainer.value

  // 全屏/退出全屏时预览容器尺寸变化：重设马赛克画布分辨率，并清掉覆盖层（马赛克已在合成图内，避免遮挡水印）；涂抹开始时重新显示
  requestAnimationFrame(() => {
    getMosaicCanvasSize()
    const canvas = mosaicCanvas.value
    canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
  })
}

// 进入全屏：自动开启马赛克模式，便于「放大 + 涂抹」同时进行
const enterPreviewFullscreen = async () => {
  const container = previewContainer.value
  if (!container || !watermarkedImageSrc.value) return
  try {
    // 全屏的唯一目的就是放大后涂抹，直接激活画笔
    isMosaicMode.value = true
    await container.requestFullscreen()
  } catch (error) {
    console.error('Fullscreen request failed:', error)
    ElMessage.error('当前浏览器不支持全屏模式')
  }
}

const togglePreviewFullscreen = async () => {
  const container = previewContainer.value
  if (!container || !watermarkedImageSrc.value) return

  const shouldEnterFullscreen = !document.fullscreenElement

  // 双击进全屏：第一击会留下一个单击污点（无轨迹单点描边），这里自动清掉
  const last = mosaicStrokes.value[mosaicStrokes.value.length - 1]
  if (shouldEnterFullscreen && last && Date.now() - lastStrokePaintedAt < 800 && last.points.length <= 2) {
    mosaicStrokes.value.pop()
    redrawMosaicLayer()
    renderMosaicOverlay()
    generateWatermark()
  }

  if (shouldEnterFullscreen) {
    isMosaicMode.value = true
  }

  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await container.requestFullscreen()
    }
  } catch (error) {
    console.error('Fullscreen toggle failed:', error)
    isPreviewExpanded.value = !isPreviewExpanded.value
    ElMessage.error('当前浏览器不支持全屏模式，已切换为缩放展示')
  }
}

document.addEventListener('fullscreenchange', updatePreviewState)
document.addEventListener('paste', onPasteImage)

onUnmounted(() => {
  document.removeEventListener('fullscreenchange', updatePreviewState)
  document.removeEventListener('paste', onPasteImage)
  document.removeEventListener('keydown', onMosaicUndoKeydown)
  previewObserver?.disconnect()
  previewObserver = null
})

// Ctrl + 右键按下：进入笔刷调整模式（横向=大小，纵向=密度），不画点
const startBrushResize = (event: PointerEvent) => {
  const canvas = mosaicCanvas.value
  if (!isMosaicMode.value || !canvas) return
  if (event.button !== 2 || !event.ctrlKey) return
  event.preventDefault()
  isResizingBrush.value = true
  brushResizeStart.value = {
    x: event.clientX,
    y: event.clientY,
    size: mosaicBrushSize.value,
    density: mosaicCellsPerBrush.value,
  }
  updateMosaicPointerPosition(event) // 按下即显示调整提示框
  try {
    canvas.setPointerCapture(event.pointerId) // 拖出画布仍持续接收事件
  } catch {
    /* 合成事件等场景不支持捕获时忽略 */
  }
}

// 拖拽中：实时更新笔刷大小与方格密度（带灵敏度与范围钳制）
const moveBrushResize = (event: PointerEvent) => {
  if (!isResizingBrush.value || !brushResizeStart.value) return
  updateMosaicPointerPosition(event)
  const s = brushResizeStart.value
  const dx = event.clientX - s.x
  const dy = event.clientY - s.y
  mosaicBrushSize.value = Math.min(80, Math.max(8, Math.round(s.size + dx * 0.5)))
  mosaicCellsPerBrush.value = Math.min(8, Math.max(1, Math.round(s.density + dy * 0.05)))
}

const endBrushResize = (event: PointerEvent) => {
  if (!isResizingBrush.value) return
  isResizingBrush.value = false
  brushResizeStart.value = null
  const canvas = mosaicCanvas.value
  if (canvas && canvas.hasPointerCapture(event.pointerId)) {
    canvas.releasePointerCapture(event.pointerId)
  }
}

// canvas 指针事件统一入口：先判断是否笔刷调整，否则走涂抹
const onCanvasPointerDown = (event: PointerEvent) => {
  startBrushResize(event)
  if (!isResizingBrush.value) startMosaicPaint(event)
}

const onCanvasPointerMove = (event: PointerEvent) => {
  if (isResizingBrush.value) moveBrushResize(event)
  else moveMosaicPaint(event)
}

const onCanvasPointerUp = (event: PointerEvent) => {
  endBrushResize(event)
  stopMosaicPaint()
}

const onCanvasPointerCancel = (event: PointerEvent) => {
  endBrushResize(event)
  stopMosaicPaint()
}

const startMosaicPaint = (event: PointerEvent) => {
  if (!isMosaicMode.value || !mosaicCanvas.value) return
  // 仅左键涂抹；双击的第二击（detail>1）不画点：避免双击进全屏时留下污点
  if (event.button !== 0 || event.detail > 1) return

  getMosaicCanvasSize()
  updateMosaicPointerPosition(event)

  const rect = mosaicCanvas.value.getBoundingClientRect()
  const x = (event.clientX - rect.left) / rect.width
  const y = (event.clientY - rect.top) / rect.height

  const stroke: MosaicStroke = {
    size: mosaicBrushSize.value,
    previewWidth: Math.max(1, mosaicCanvas.value.width),
    cellsPerBrush: mosaicCellsPerBrush.value,
    points: [{ x: Math.min(Math.max(x, 0), 1), y: Math.min(Math.max(y, 0), 1) }],
  }

  mosaicStrokes.value.push(stroke)
  lastStrokePaintedAt = Date.now()
  isPaintingMosaic.value = true
  paintPointToLayer(stroke, stroke.points[0]?.x ?? 0, stroke.points[0]?.y ?? 0)
  renderMosaicOverlay()
}

const moveMosaicPaint = (event: PointerEvent) => {
  updateMosaicPointerPosition(event)

  if (!isPaintingMosaic.value || !mosaicCanvas.value) return

  const rect = mosaicCanvas.value.getBoundingClientRect()
  const x = (event.clientX - rect.left) / rect.width
  const y = (event.clientY - rect.top) / rect.height
  const currentStroke = mosaicStrokes.value[mosaicStrokes.value.length - 1]

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
  renderMosaicOverlay()
}

const stopMosaicPaint = () => {
  isPaintingMosaic.value = false
  mosaicBrushPosition.value = null
  // 一笔结束：把马赛克图层合并进水印合成图（原图 -> 马赛克 -> 水印）
  if (mosaicStrokes.value.length > 0 && originalImage.value) {
    generateWatermark()
  }
}

// 撤回上一步：移除最后一条描边，重建马赛克图层并重新合成
const undoMosaic = () => {
  if (!mosaicStrokes.value.length) return
  mosaicStrokes.value.pop()
  redrawMosaicLayer()
  renderMosaicOverlay()
  generateWatermark()
}

// Ctrl/Cmd+Z 触发马赛克撤回；焦点在输入框/文本域时放行（不干扰文本撤销）
const onMosaicUndoKeydown = (e: KeyboardEvent) => {
  if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 'z' || e.shiftKey) return
  const target = e.target as HTMLElement | null
  const isTextTarget =
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    !!target?.isContentEditable
  if (isTextTarget) return
  if (!mosaicStrokes.value.length) return
  e.preventDefault()
  undoMosaic()
}

document.addEventListener('keydown', onMosaicUndoKeydown)

const clearMosaic = () => {
  mosaicStrokes.value = []
  redrawMosaicLayer()
  renderMosaicOverlay()
  generateWatermark()
}

const onFileChange = (e: Event) => {
  const target = e.target as HTMLInputElement
  const files = target.files
  if (files && files.length > 0 && files[0]) {
    processFile(files[0])
  }
}

// 处理图片文件
const processFile = (file: File) => {
  if (!file.type.startsWith('image/')) {
    ElMessage.error('请选择有效的图片文件！')
    return
  }
  originalFileName.value = file.name
  mosaicStrokes.value = []

  const reader = new FileReader()
  reader.onload = (e) => {
    const img = new Image()
    img.onload = () => {
      originalImage.value = img
      initMosaicLayer()
      generateWatermark()
    }
    img.src = e.target?.result as string
  }
  reader.readAsDataURL(file)
}

// 渲染水印：图层顺序 = 原图 -> 马赛克（独立图层） -> 水印文字
const generateWatermark = () => {
  const img = originalImage.value
  if (!img) return

  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  // 保证导出质量：Canvas 宽高设为图片真实分辨率
  canvas.width = img.naturalWidth || img.width
  canvas.height = img.naturalHeight || img.height

  // 确保马赛克图层与原图同分辨率（换图后重建）
  if (!mosaicLayer || mosaicLayer.width !== canvas.width || mosaicLayer.height !== canvas.height) {
    initMosaicLayer()
  }

  // 1. 绘制原图
  ctx.drawImage(img, 0, 0)

  // 2. 马赛克独立图层（位于水印文字之下：涂抹区域水印文字仍清晰可见）
  applyMosaicToCanvas(canvas, ctx)

  if (watermarkEnabled.value) {
    // 配置文字格式
    const stylePrefix =
      fontStyle.value === 'bold' ? 'bold ' : fontStyle.value === 'italic' ? 'italic ' : ''
    ctx.font = `${stylePrefix}${fontSize.value}px sans-serif`
    ctx.fillStyle = textColor.value
    ctx.globalAlpha = opacity.value
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'

    // 计算旋转与对角线覆盖范围，确保水印交叉铺满
    const diagonal = Math.sqrt(canvas.width * canvas.width + canvas.height * canvas.height)
    const angleRad = (angle.value * Math.PI) / 180

    // 计算文本真实宽度，确保长文本在任何字号下都不会发生重叠
    const textWidth = ctx.measureText(watermarkText.value).width
    const stepX = textWidth + gapX.value
    const stepY = gapY.value

    const startX = -diagonal / 2
    const endX = diagonal / 2
    const startY = -diagonal / 2
    const endY = diagonal / 2

    // 绘制单向倾斜的交错平铺水印（无重叠交叉）
    ctx.save()
    ctx.translate(canvas.width / 2, canvas.height / 2)
    ctx.rotate(angleRad)
    let row = 0
    for (let y = startY; y < endY; y += stepY) {
      // 奇偶行错开以实现斜向交叉铺满
      const offsetX = row % 2 === 0 ? 0 : stepX / 2
      for (let x = startX + offsetX; x < endX; x += stepX) {
        ctx.fillText(watermarkText.value, x, y)
      }
      row++
    }
    ctx.restore()
  }

  // 3. 盲水印（不可见）：嵌入最终像素，肉眼不可见，PNG 导出后可提取
  applyBlindWatermark(canvas, ctx)

  // 生成新的 DataURL 用于预览 and 下载
  watermarkedImageSrc.value = canvas.toDataURL('image/png')
}

// 盲水印嵌入：在最终合成像素上就地写入（PNG 无损导出后可提取）
const applyBlindWatermark = (canvas: HTMLCanvasElement, ctx: CanvasRenderingContext2D) => {
  if (!blindEnabled.value) return
  const text = blindText.value.trim()
  if (!text) return
  const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height)
  const ok = embedBlindWatermark(imgData, text, { bits: blindBits.value })
  if (ok) {
    ctx.putImageData(imgData, 0, 0)
  } else {
    ElMessage.warning('图片尺寸过小，无法嵌入盲水印')
  }
}

// 监听参数变化，实时刷新水印
watch([watermarkEnabled, watermarkText, fontSize, opacity, angle, textColor, gapX, gapY, fontStyle], () => {
  if (originalImage.value) {
    generateWatermark()
  }
})

watch([blindEnabled, blindText, blindBits], () => {
  if (originalImage.value) {
    generateWatermark()
  }
})

// 从图片源提取盲水印（真盲提取，不需要原图）
const extractBlindFromSource = async (src: string): Promise<string | null> => {
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

const runExtract = async (src: string) => {
  try {
    const text = await extractBlindFromSource(src)
    extractedText.value = text
    if (text) ElMessage.success('盲水印提取成功')
    else ElMessage.warning('未检测到盲水印（或已被有损压缩破坏）')
  } catch (e) {
    console.error('Blind extract failed:', e)
    ElMessage.error('提取失败：图片解码异常')
  }
}

// 从当前预览图提取
const extractFromPreview = () => {
  if (!watermarkedImageSrc.value) return
  runExtract(watermarkedImageSrc.value)
}

// 从外部文件提取
const onExtractFile = (e: Event) => {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => runExtract(reader.result as string)
  reader.readAsDataURL(file)
  target.value = ''
}

watch(watermarkedImageSrc, () => {
  // 合成图更新后，清掉预览覆盖层（马赛克已含在合成图内），避免遮挡水印
  const canvas = mosaicCanvas.value
  canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
})

// 下载水印图片
const downloadImage = () => {
  if (!watermarkedImageSrc.value) return

  const link = document.createElement('a')
  const baseName = originalFileName.value.replace(/\.[^/.]+$/, '')
  link.download = `${baseName}-watermarked.png`
  link.href = watermarkedImageSrc.value
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  ElMessage.success('水印图片已成功生成并下载！')
}

// 移除当前图片
const clearImage = () => {
  originalImage.value = null
  originalFileName.value = ''
  watermarkedImageSrc.value = ''
  mosaicStrokes.value = []
  mosaicLayer?.getContext('2d')?.clearRect(0, 0, mosaicLayer.width, mosaicLayer.height)
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}
</script>

<template>
  <section class="watermark-tool" @paste="onPasteImage">
    <header class="tool-intro"><div><span class="eyebrow">IMAGE STUDIO</span><h2>图片水印</h2><p>添加可见水印、盲水印，并在下载前进行马赛克处理。</p></div><div class="local-badge">● 浏览器本地处理</div></header>

    <div class="watermark-layout">
      <section class="preview-panel">
        <div class="panel-top"><div><span class="eyebrow">PREVIEW</span><h3>{{ watermarkedImageSrc ? '实时预览' : '选择图片开始' }}</h3></div><div v-if="watermarkedImageSrc" class="preview-actions"><button @click="togglePreviewFullscreen">全屏</button><button @click="clearImage">更换图片</button></div></div>
        <div v-if="!watermarkedImageSrc" class="upload-zone" :class="{ dragging: isDragging }" @dragover="onDragOver" @dragleave="onDragLeave" @drop="onDrop" @click="triggerFileSelect">
          <input ref="fileInput" type="file" accept="image/*" hidden @change="onFileChange" />
          <span class="upload-mark">↑</span><strong>拖拽图片到这里</strong><p>或点击选择图片，也可以直接粘贴</p><small>支持 PNG、JPG、WEBP 等常见格式</small>
        </div>
        <template v-else>
          <div ref="previewContainer" class="image-stage" :class="{ painting: isMosaicMode }" @pointermove="updateMosaicPointerPosition">
            <div class="image-frame" :style="previewBox ? { width: `${previewBox.w}px`, height: `${previewBox.h}px` } : undefined">
              <img class="preview-image" :src="watermarkedImageSrc" alt="水印预览" @dblclick="togglePreviewFullscreen" />
              <canvas ref="mosaicCanvas" class="mosaic-layer" @pointerdown="onCanvasPointerDown" @pointermove="onCanvasPointerMove" @pointerup="onCanvasPointerUp" @pointercancel="onCanvasPointerCancel" @pointerleave="stopMosaicPaint"></canvas>
              <span v-if="mosaicBrushPosition && isMosaicMode" class="brush-cursor" :style="{ left: `${mosaicBrushPosition.x}px`, top: `${mosaicBrushPosition.y}px`, width: `${mosaicBrushSize}px`, height: `${mosaicBrushSize}px` }"></span>
            </div>
            <div v-if="isMosaicMode" class="painting-tip">拖动涂抹马赛克 · 松开完成</div>
          </div>
          <div class="preview-footer"><span>{{ originalFileName }}</span><div><button v-if="mosaicStrokes.length" @click="undoMosaic">撤销涂抹</button><button v-if="mosaicStrokes.length" @click="clearMosaic">清除马赛克</button><button class="primary" @click="downloadImage">下载图片</button></div></div>
        </template>
      </section>

      <aside class="settings-panel">
        <div class="settings-heading"><span class="eyebrow">SETTINGS</span><h3>水印设置</h3></div>
        <div class="setting-block"><div class="setting-label"><label>可见水印</label><button class="toggle" :class="{ on: watermarkEnabled }" @click="watermarkEnabled = !watermarkEnabled"><i></i></button></div><input v-model="watermarkText" class="text-input" placeholder="输入水印文字" /><div class="preset-list"><button v-for="preset in presets" :key="preset" @click="handlePresetClick(preset)">{{ preset }}</button></div></div>
        <div class="setting-block"><div class="setting-label"><label>字体样式</label><select v-model="fontStyle"><option value="normal">常规</option><option value="bold">粗体</option><option value="italic">斜体</option></select></div><div class="range-row"><label>大小 <b>{{ fontSize }}px</b></label><input v-model.number="fontSize" type="range" min="12" max="120" /></div><div class="range-row"><label>透明度 <b>{{ Math.round(opacity * 100) }}%</b></label><input v-model.number="opacity" type="range" min="0.05" max="1" step="0.05" /></div><div class="range-row"><label>旋转角度 <b>{{ angle }}°</b></label><input v-model.number="angle" type="range" min="-90" max="90" /></div></div>
        <div class="setting-block"><div class="setting-label"><label>水印颜色</label><span class="color-value"><input v-model="textColor" type="color" />{{ textColor }}</span></div><div class="color-list"><button v-for="color in colorPresets" :key="color.value" :title="color.name" :style="{ background: color.value }" :class="{ selected: textColor === color.value }" @click="textColor = color.value"></button></div></div>
        <div class="setting-block"><div class="setting-label"><label>盲水印</label><button class="toggle" :class="{ on: blindEnabled }" @click="blindEnabled = !blindEnabled"><i></i></button></div><input v-model="blindText" class="text-input" :disabled="!blindEnabled" placeholder="嵌入不可见的文字" /><div class="range-row"><label>嵌入强度 <b>{{ blindBits }} bit</b></label><input v-model.number="blindBits" type="range" min="1" max="2" :disabled="!blindEnabled" /></div></div>
        <div class="setting-block mosaic-block"><div class="setting-label"><label>马赛克涂抹</label><button class="toggle" :class="{ on: isMosaicMode }" @click="isMosaicMode = !isMosaicMode"><i></i></button></div><div v-if="isMosaicMode" class="range-row"><label>笔刷大小 <b>{{ mosaicBrushSize }}px</b></label><input v-model.number="mosaicBrushSize" type="range" min="8" max="100" /></div></div>
        <div class="extract-block"><div class="setting-label"><label>提取盲水印</label></div><div class="extract-actions"><button @click="extractFromPreview" :disabled="!watermarkedImageSrc">从当前图提取</button><button @click="extractFileInput?.click()">选择文件</button><input ref="extractFileInput" type="file" accept="image/*" hidden @change="onExtractFile" /></div><p v-if="extractedText !== null" class="extract-result">{{ extractedText || '未检测到盲水印' }}</p></div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.watermark-tool { display: grid; gap: 18px; max-width: 1280px; margin: 0 auto; color: var(--text-main); }.tool-intro { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }.eyebrow { color: var(--primary); font-size: .62rem; font-weight: 800; letter-spacing: .14em; }.tool-intro h2 { margin: 5px 0; font-size: 1.2rem; }.tool-intro p { margin: 0; color: var(--text-muted); font-size: .74rem; }.local-badge { padding: 6px 9px; border: 1px solid var(--border-glass); border-radius: 999px; color: var(--accent-emerald); font-size: .65rem; }
.upload-zone { display: grid; min-height: 420px; place-items: center; align-content: center; gap: 9px; border: 1px dashed var(--border-glass); border-radius: 14px; background: var(--bg-panel); text-align: center; cursor: pointer; transition: .2s; }.upload-zone:hover, .upload-zone.dragging { border-color: var(--primary); background: color-mix(in srgb, var(--primary) 7%, var(--bg-panel)); }.upload-mark { display: grid; place-items: center; width: 58px; height: 58px; border: 1px solid color-mix(in srgb, var(--primary) 30%, var(--border-glass)); border-radius: 16px; color: var(--primary); font-size: 2rem; }.upload-zone strong { font-size: .9rem; }.upload-zone p, .upload-zone small { margin: 0; color: var(--text-muted); font-size: .72rem; }.upload-zone small { color: var(--text-dim); font-size: .64rem; }
.watermark-layout { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 16px; align-items: start; }.preview-panel, .settings-panel { min-width: 0; border: 1px solid var(--border-glass); border-radius: 14px; background: var(--bg-panel); }.preview-panel { overflow: hidden; }.panel-top, .preview-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 15px 17px; }.panel-top { border-bottom: 1px solid var(--border-glass); }.panel-top h3, .settings-heading h3 { margin: 4px 0 0; font-size: .9rem; }.preview-actions, .preview-footer > div { display: flex; gap: 6px; }.preview-actions button, .preview-footer button, .extract-actions button { border: 1px solid var(--border-glass); border-radius: 7px; padding: 7px 9px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .65rem; font-weight: 700; }.preview-actions button:hover, .preview-footer button:hover, .extract-actions button:hover { border-color: var(--primary); color: var(--primary); }.preview-footer { border-top: 1px solid var(--border-glass); color: var(--text-dim); font: .64rem var(--font-mono); }.preview-footer button.primary { border-color: var(--primary); background: var(--primary); color: white; }.image-stage { position: relative; display: grid; min-height: 540px; place-items: center; overflow: hidden; padding: 22px; background: repeating-conic-gradient(color-mix(in srgb, var(--bg-input) 80%, white) 0 25%, var(--bg-input) 0 50%) 50% / 20px 20px; }.image-frame { position: relative; max-width: 100%; max-height: 100%; }.preview-image { display: block; width: 100%; height: 100%; object-fit: contain; }.mosaic-layer { position: absolute; inset: 0; width: 100%; height: 100%; cursor: crosshair; }.image-stage:not(.painting) .mosaic-layer { pointer-events: none; }.brush-cursor { position: absolute; z-index: 2; transform: translate(-50%, -50%); border: 1px solid white; border-radius: 50%; box-shadow: 0 0 0 1px #111; pointer-events: none; }.painting-tip { position: absolute; top: 12px; left: 50%; transform: translateX(-50%); padding: 5px 8px; border-radius: 6px; background: rgba(2,8,23,.72); color: white; font-size: .65rem; }
.settings-panel { padding: 17px; }.settings-heading { margin-bottom: 17px; }.setting-block, .extract-block { display: grid; gap: 10px; padding: 15px 0; border-top: 1px solid var(--border-glass); }.setting-label { display: flex; align-items: center; justify-content: space-between; gap: 8px; }.setting-label label { color: var(--text-main); font-size: .72rem; font-weight: 700; }.toggle { width: 31px; height: 18px; padding: 2px; border: 0; border-radius: 20px; background: var(--text-dim); cursor: pointer; }.toggle i { display: block; width: 14px; height: 14px; border-radius: 50%; background: white; transition: .2s; }.toggle.on { background: var(--primary); }.toggle.on i { transform: translateX(13px); }.text-input, .setting-label select { min-width: 0; border: 1px solid var(--border-glass); border-radius: 7px; padding: 8px 9px; outline: none; background: var(--bg-input); color: var(--text-main); font: inherit; font-size: .7rem; }.text-input:focus, .setting-label select:focus { border-color: var(--primary); }.preset-list { display: flex; flex-wrap: wrap; gap: 5px; }.preset-list button { border: 1px solid var(--border-glass); border-radius: 5px; padding: 4px 6px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: .6rem; }.range-row { display: grid; gap: 6px; }.range-row label { display: flex; justify-content: space-between; color: var(--text-muted); font-size: .65rem; }.range-row b { color: var(--text-main); font-weight: 600; }.range-row input[type=range] { width: 100%; accent-color: var(--primary); }.color-value { display: inline-flex; align-items: center; gap: 5px; color: var(--text-muted); font: .62rem var(--font-mono); }.color-value input { width: 22px; height: 22px; padding: 0; border: 0; background: transparent; cursor: pointer; }.color-list { display: flex; gap: 8px; }.color-list button { width: 22px; height: 22px; border: 2px solid transparent; border-radius: 50%; cursor: pointer; }.color-list button.selected { border-color: var(--primary); box-shadow: 0 0 0 2px var(--bg-panel); outline: 1px solid var(--primary); }.extract-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }.extract-result { margin: 0; padding: 8px; border-radius: 6px; background: color-mix(in srgb, var(--accent-emerald) 10%, transparent); color: var(--accent-emerald); font-size: .66rem; overflow-wrap: anywhere; }
@media (max-width: 900px) { .watermark-layout { grid-template-columns: 1fr; }.settings-panel { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; }.settings-heading { grid-column: 1 / -1; }.extract-block { grid-column: 1 / -1; } }
@media (max-width: 600px) { .tool-intro { align-items: flex-start; flex-direction: column; }.watermark-layout { display: flex; flex-direction: column; }.settings-panel { display: block; width: 100%; box-sizing: border-box; }.image-stage { min-height: 360px; padding: 12px; }.panel-top, .preview-footer { align-items: flex-start; flex-direction: column; }.preview-footer > div { width: 100%; }.preview-footer button { flex: 1; } }
</style>
