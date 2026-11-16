<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, inject } from 'vue'
import { useBlindWatermark } from './useBlindWatermark'
import { useImageIntake } from './useImageIntake'
import { useRectangleTool } from './useRectangleTool'
import { useMosaicEngine } from './useMosaicEngine'
import { ToastKey, type ToastType } from '../injectionKeys'
import WatermarkSettingsPanel from './WatermarkSettingsPanel.vue'
import { usePreviewBox } from './usePreviewBox'

const toast = inject<ToastType extends never ? never : (msg: string, type?: ToastType) => void>(ToastKey)
const ElMessage = {
  success: (message: string) => toast?.(message, 'success'),
  error: (message: string) => toast?.(message, 'error'),
  warning: (message: string) => toast?.(message, 'warning'),
}

const watermarkedImageSrc = ref<string>('')
const {
  originalImage,
  originalFileName,
  isDragging,
  fileInput,
  onDragOver,
  onDragLeave,
  onDrop,
  onPasteImage,
  triggerFileSelect,
  onFileChange,
  clearImage,
} = useImageIntake({
  onImageLoaded: () => {
    initMosaicLayer()
    generateWatermark()
  },
  onReset: () => {
    watermarkedImageSrc.value = ''
    resetMosaic()
    rectangleMarks.value = []
  },
  onInvalidType: () => ElMessage.error('请选择有效的图片文件！'),
})
const previewContainer = ref<HTMLElement | null>(null)
const isPreviewExpanded = ref(false)
const mosaicCanvas = ref<HTMLCanvasElement | null>(null)
const isMosaicMode = ref(false)
const isRectangleMode = ref(false)
const {
  rectangleColor,
  rectangleWidth,
  rectangleMarks,
  startRectangle,
  moveRectangle,
  finishRectangle,
  cancelRectangle,
  undoRectangle,
  clearRectangles,
  resetPreview: resetRectanglePreview,
  renderMarks: renderRectangleMarks,
} = useRectangleTool(mosaicCanvas, {
  renderOverlay: () => renderMosaicOverlay(),
  regenerate: () => {
    if (originalImage.value) generateWatermark()
  },
})
const {
  strokes: mosaicStrokes,
  brushSize: mosaicBrushSize,
  brushPosition: mosaicBrushPosition,
  isResizingBrush,
  initMosaicLayer,
  reset: resetMosaic,
  repaint: repaintMosaic,
  popAccidentalTap,
  updatePointerPosition: updateMosaicPointerPosition,
  getCanvasSize: getMosaicCanvasSize,
  clearOverlay: clearMosaicOverlay,
  renderOverlay: renderMosaicOverlay,
  applyToCanvas: applyMosaicToCanvas,
  ensureLayer: ensureMosaicLayer,
  startBrushResize,
  moveBrushResize,
  endBrushResize,
  startPaint: startMosaicPaint,
  movePaint: moveMosaicPaint,
  stopPaint: stopMosaicPaint,
  undo: undoMosaic,
  clear: clearMosaic,
  handleUndoKeydown: onMosaicUndoKeydown,
} = useMosaicEngine({
  canvas: mosaicCanvas,
  sourceImage: originalImage,
  isActive: () => isMosaicMode.value,
  renderExtra: (ctx, outWidth, outHeight) => {
    if (isRectangleMode.value || rectangleMarks.value.length) {
      renderRectangleMarks(ctx, outWidth, outHeight)
    }
  },
  regenerate: () => {
    if (originalImage.value) generateWatermark()
  },
})

// 快速预设文字
const presets = [
  'www.ryugo.org',
  '仅供实名认证，他用无效',
]

// 水印参数
const watermarkText = ref<string>(presets[0] || "www.ryugo.org")

// 盲水印（不可见水印）
const {
  blindEnabled,
  blindText,
  blindBits,
  extractedText,
  applyBlindWatermark,
  extractFromPreview,
  onExtractUpload,
} = useBlindWatermark({
  initialText: watermarkText.value, // 默认复用可见水印文本，可单独改
  bits: 1, // 嵌入强度：蓝通道最低 1~2 位
  watermarkedImageSrc,
  notify: ElMessage,
  onEmbedChange: () => {
    if (originalImage.value) generateWatermark()
  },
})

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

const selectEditTool = (tool: 'none' | 'mosaic' | 'rectangle') => {
  isMosaicMode.value = tool === 'mosaic'
  isRectangleMode.value = tool === 'rectangle'
  mosaicBrushPosition.value = null
  resetRectanglePreview()
}

const { previewBox } = usePreviewBox(originalImage, previewContainer)

const updatePreviewState = () => {
  if (!previewContainer.value) {
    isPreviewExpanded.value = false
    return
  }

  isPreviewExpanded.value = document.fullscreenElement === previewContainer.value

  // 全屏/退出全屏时预览容器尺寸变化：重设马赛克画布分辨率，并清掉覆盖层（马赛克已在合成图内，避免遮挡水印）；涂抹开始时重新显示
  requestAnimationFrame(() => {
    getMosaicCanvasSize()
    clearMosaicOverlay()
  })
}

const togglePreviewFullscreen = async () => {
  const container = previewContainer.value
  if (!container || !watermarkedImageSrc.value) return

  const shouldEnterFullscreen = !document.fullscreenElement

  // 双击进全屏：第一击会留下一个单击污点（无轨迹单点描边），这里自动清掉
  if (shouldEnterFullscreen && popAccidentalTap()) {
    repaintMosaic()
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
  } catch {
    isPreviewExpanded.value = !isPreviewExpanded.value
    ElMessage.error('当前浏览器不支持全屏模式，已切换为缩放展示')
  }
}

// canvas 指针事件统一入口：矩形模式走矩形工具，否则走马赛克（先判断笔刷调整）
const onCanvasPointerDown = (event: PointerEvent) => {
  if (isRectangleMode.value) {
    startRectangle(event)
    return
  }
  if (!startBrushResize(event)) startMosaicPaint(event)
}

const onCanvasPointerMove = (event: PointerEvent) => {
  if (isRectangleMode.value) {
    moveRectangle(event)
    return
  }
  if (isResizingBrush.value) moveBrushResize(event)
  else moveMosaicPaint(event)
}

const onCanvasPointerUp = (event: PointerEvent) => {
  if (isRectangleMode.value) {
    finishRectangle(event)
    return
  }
  endBrushResize(event)
  stopMosaicPaint()
}

const onCanvasPointerCancel = (event: PointerEvent) => {
  if (isRectangleMode.value) {
    cancelRectangle(event)
    return
  }
  endBrushResize(event)
  stopMosaicPaint()
}

onMounted(() => {
  document.addEventListener('fullscreenchange', updatePreviewState)
  document.addEventListener('paste', onPasteImage)
  document.addEventListener('keydown', onMosaicUndoKeydown)
})

onUnmounted(() => {
  document.removeEventListener('fullscreenchange', updatePreviewState)
  document.removeEventListener('paste', onPasteImage)
  document.removeEventListener('keydown', onMosaicUndoKeydown)
})

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
  ensureMosaicLayer(canvas.width, canvas.height)

  // 1. 绘制原图
  ctx.drawImage(img, 0, 0)

  // 2. 马赛克独立图层（位于水印文字之下：涂抹区域水印文字仍清晰可见）
  applyMosaicToCanvas(canvas, ctx)

  // 3. 矩形标记图层，使用原图坐标导出，线宽随输出尺寸等比缩放。
  if (rectangleMarks.value.length) {
    ctx.save()
    renderRectangleMarks(ctx, canvas.width, canvas.height)
    ctx.restore()
  }

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

// 监听参数变化，实时刷新水印
watch([watermarkEnabled, watermarkText, fontSize, opacity, angle, textColor, gapX, gapY, fontStyle], () => {
  if (originalImage.value) {
    generateWatermark()
  }
})

watch([rectangleColor, rectangleWidth, isRectangleMode], () => {
  renderMosaicOverlay()
  if (originalImage.value) generateWatermark()
})

watch(watermarkedImageSrc, () => {
  // 合成图更新后，清掉预览覆盖层（马赛克已含在合成图内），避免遮挡水印
  const canvas = mosaicCanvas.value
  canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
  if (isRectangleMode.value) renderMosaicOverlay()
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
           <div ref="previewContainer" class="image-stage" :class="{ painting: isMosaicMode || isRectangleMode }" @pointermove="updateMosaicPointerPosition">
            <div class="image-frame" :style="previewBox ? { width: `${previewBox.w}px`, height: `${previewBox.h}px` } : undefined">
              <img class="preview-image" :src="watermarkedImageSrc" alt="水印预览" @dblclick="togglePreviewFullscreen" />
              <canvas ref="mosaicCanvas" class="mosaic-layer" @pointerdown="onCanvasPointerDown" @pointermove="onCanvasPointerMove" @pointerup="onCanvasPointerUp" @pointercancel="onCanvasPointerCancel" @pointerleave="stopMosaicPaint"></canvas>
              <span v-if="mosaicBrushPosition && isMosaicMode" class="brush-cursor" :style="{ left: `${mosaicBrushPosition.x}px`, top: `${mosaicBrushPosition.y}px`, width: `${mosaicBrushSize}px`, height: `${mosaicBrushSize}px` }"></span>
            </div>
             <div v-if="isMosaicMode" class="painting-tip">拖动涂抹马赛克 · 松开完成</div>
             <div v-else-if="isRectangleMode" class="painting-tip">拖动添加矩形标记 · 松开完成</div>
          </div>
           <div class="preview-footer"><span>{{ originalFileName }}</span><div><button v-if="mosaicStrokes.length" @click="undoMosaic">撤销涂抹</button><button v-if="mosaicStrokes.length" @click="clearMosaic">清除马赛克</button><button v-if="rectangleMarks.length" @click="undoRectangle">撤销标记</button><button v-if="rectangleMarks.length" @click="clearRectangles">清除标记</button><button class="primary" @click="downloadImage">下载图片</button></div></div>
        </template>
      </section>

      <WatermarkSettingsPanel
        v-model:watermarkEnabled="watermarkEnabled"
        v-model:watermarkText="watermarkText"
        v-model:fontStyle="fontStyle"
        v-model:fontSize="fontSize"
        v-model:opacity="opacity"
        v-model:angle="angle"
        v-model:textColor="textColor"
        v-model:blindEnabled="blindEnabled"
        v-model:blindText="blindText"
        v-model:blindBits="blindBits"
        v-model:isMosaicMode="isMosaicMode"
        v-model:isRectangleMode="isRectangleMode"
        v-model:mosaicBrushSize="mosaicBrushSize"
        v-model:rectangleColor="rectangleColor"
        v-model:rectangleWidth="rectangleWidth"
        :presets="presets"
        :color-presets="colorPresets"
        :watermarked-image-src="watermarkedImageSrc"
        :extracted-text="extractedText"
        @preset-click="handlePresetClick"
        @select-tool="selectEditTool"
        @extract-preview="extractFromPreview"
        @extract-file="onExtractUpload"
      />
    </div>
  </section>
</template>

<style scoped>
.watermark-tool { display: grid; gap: 18px; max-width: 1280px; margin: 0 auto; color: var(--text-main); }.tool-intro { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }.eyebrow { color: var(--primary); font-size: .62rem; font-weight: 800; letter-spacing: .14em; }.tool-intro h2 { margin: 5px 0; font-size: 1.2rem; }.tool-intro p { margin: 0; color: var(--text-muted); font-size: .74rem; }.local-badge { padding: 6px 9px; border: 1px solid var(--border-glass); border-radius: 999px; color: var(--accent-emerald); font-size: .65rem; }
.upload-zone { display: grid; min-height: 420px; place-items: center; align-content: center; gap: 9px; border: 1px dashed var(--border-glass); border-radius: 14px; background: var(--bg-panel); text-align: center; cursor: pointer; transition: .2s; }.upload-zone:hover, .upload-zone.dragging { border-color: var(--primary); background: color-mix(in srgb, var(--primary) 7%, var(--bg-panel)); }.upload-mark { display: grid; place-items: center; width: 58px; height: 58px; border: 1px solid color-mix(in srgb, var(--primary) 30%, var(--border-glass)); border-radius: 16px; color: var(--primary); font-size: 2rem; }.upload-zone strong { font-size: .9rem; }.upload-zone p, .upload-zone small { margin: 0; color: var(--text-muted); font-size: .72rem; }.upload-zone small { color: var(--text-dim); font-size: .64rem; }
.watermark-layout { display: grid; grid-template-columns: minmax(0, 1fr) 310px; gap: 16px; align-items: start; }.preview-panel { min-width: 0; border: 1px solid var(--border-glass); border-radius: 14px; background: var(--bg-panel); }.preview-panel { overflow: hidden; }.panel-top, .preview-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 15px 17px; }.panel-top { border-bottom: 1px solid var(--border-glass); }.panel-top h3 { margin: 4px 0 0; font-size: .9rem; }.preview-actions, .preview-footer > div { display: flex; gap: 6px; }.preview-actions button, .preview-footer button { border: 1px solid var(--border-glass); border-radius: 7px; padding: 7px 9px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .65rem; font-weight: 700; }.preview-actions button:hover, .preview-footer button:hover { border-color: var(--primary); color: var(--primary); }.preview-footer { border-top: 1px solid var(--border-glass); color: var(--text-dim); font: .64rem var(--font-mono); }.preview-footer button.primary { border-color: var(--primary); background: var(--primary); color: white; }.image-stage { position: relative; display: grid; min-height: 540px; place-items: center; overflow: hidden; padding: 22px; background: repeating-conic-gradient(color-mix(in srgb, var(--bg-input) 80%, white) 0 25%, var(--bg-input) 0 50%) 50% / 20px 20px; }.image-frame { position: relative; max-width: 100%; max-height: 100%; }.preview-image { display: block; width: 100%; height: 100%; object-fit: contain; }.mosaic-layer { position: absolute; inset: 0; width: 100%; height: 100%; cursor: crosshair; }.image-stage:not(.painting) .mosaic-layer { pointer-events: none; }.brush-cursor { position: absolute; z-index: 2; transform: translate(-50%, -50%); border: 1px solid white; border-radius: 50%; box-shadow: 0 0 0 1px #111; pointer-events: none; }.painting-tip { position: absolute; top: 12px; left: 50%; transform: translateX(-50%); padding: 5px 8px; border-radius: 6px; background: rgba(2,8,23,.72); color: white; font-size: .65rem; }
 .edit-tool-buttons { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }.edit-tool-buttons button, .tool-reset { border: 1px solid var(--border-glass); border-radius: 7px; padding: 7px 8px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .65rem; font-weight: 700; }.edit-tool-buttons button.selected, .edit-tool-buttons button:hover, .tool-reset:hover { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); }
 @media (max-width: 900px) { .watermark-layout { grid-template-columns: 1fr; }.settings-panel { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; }.settings-heading { grid-column: 1 / -1; }.extract-block { grid-column: 1 / -1; } }
@media (max-width: 600px) { .tool-intro { align-items: flex-start; flex-direction: column; }.watermark-layout { display: flex; flex-direction: column; }.settings-panel { display: block; width: 100%; box-sizing: border-box; }.image-stage { min-height: 360px; padding: 12px; }.panel-top, .preview-footer { align-items: flex-start; flex-direction: column; }.preview-footer > div { width: 100%; }.preview-footer button { flex: 1; } }
</style>
