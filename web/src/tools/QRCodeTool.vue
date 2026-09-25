<template>
  <section class="qr-tool">
    <div class="qr-mode-switch" role="tablist">
      <button :class="{ active: mode === 'generate' }" @click="handleModeChange('generate')">生成二维码</button>
      <button :class="{ active: mode === 'scan' }" @click="handleModeChange('scan')">扫描二维码</button>
    </div>

    <div v-if="mode === 'generate'" class="qr-layout">
      <div class="qr-panel qr-input-panel">
        <div class="panel-heading">
          <span class="panel-icon">↗</span>
          <div><h3>生成二维码</h3><p>输入文本或链接，生成可分享的二维码</p></div>
        </div>
        <textarea v-model="inputText" rows="7" placeholder="输入内容，按 Enter 生成二维码，Shift + Enter 换行" @keydown.enter.exact.prevent="handleGenerate"></textarea>
        <div class="field-footer"><span>{{ inputText.length }} 个字符</span><div><button class="button button-ghost" @click="handleClear">清空</button><button class="button button-primary" @click="handleGenerate">生成二维码</button></div></div>
      </div>
      <div class="qr-panel qr-preview-panel" :class="{ ready: qrText }">
        <qrcode-vue v-if="qrText" :value="qrText" :size="size" level="L" render-as="svg" class="qr-code" />
        <div v-else class="empty-state"><span class="empty-icon">⌁</span><strong>二维码预览</strong><p>生成后会显示在这里</p></div>
        <p v-if="qrText" class="preview-hint">内容已锁定，修改输入后请重新生成</p>
      </div>
    </div>

    <div v-else class="qr-scan-layout">
      <div class="qr-panel">
        <div class="panel-heading"><span class="panel-icon">⌁</span><div><h3>扫描与解析</h3><p>拖拽、上传或直接粘贴二维码图片</p></div></div>
        <div class="drop-zone" :class="{ dragging: isDragging, filled: scanImageSrc }" @dragover="handleDragOver" @dragleave="handleDragLeave" @drop="handleDrop" @click="triggerFileInput">
          <input ref="fileInput" type="file" accept="image/*" hidden @change="handleFileChange" />
          <template v-if="!scanImageSrc"><span class="upload-icon">↑</span><strong>点击上传或拖拽图片</strong><p>也可以使用 Ctrl + V 粘贴</p></template>
          <template v-else><img :src="scanImageSrc" alt="二维码图片" /><span class="drop-overlay">点击更换图片</span></template>
        </div>
      </div>
      <div v-if="scanResult || scanError" class="qr-panel result-panel">
        <div class="result-heading" :class="{ error: scanError }"><strong>{{ scanResult ? '解析成功' : '解析失败' }}</strong><button v-if="scanResult" @click="copyToClipboard(scanResult)">复制</button></div>
        <textarea v-if="scanResult" readonly rows="5">{{ scanResult }}</textarea>
        <a v-if="scanResult && isLink(scanResult)" :href="scanResult" target="_blank">在新窗口打开链接 →</a>
        <p v-if="scanError" class="error-text">{{ scanError }}</p>
        <button class="button button-ghost full-button" @click="handleClearScan">重新扫描</button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
// The original scanner is intentionally kept as a small browser-only utility.
// @ts-nocheck
import { ref, onMounted, onUnmounted, inject } from 'vue'
import QrcodeVue from 'qrcode.vue'
import jsQR from 'jsqr'
const toast = inject('toast')
const ElMessage = {
  success: (message) => toast?.(message, 'success'),
  error: (message) => toast?.(message, 'error'),
}

const props = defineProps({
  size: { type: Number, default: 220 },
})

const mode = ref('generate') // 'generate' | 'scan'

// --- 二维码生成数据 ---
const inputText = ref('')
const qrText = ref('')

const handleGenerate = () => {
  if (!inputText.value.trim()) return
  qrText.value = inputText.value
}

const handleClear = () => {
  inputText.value = ''
  qrText.value = ''
}

// --- 二维码扫描数据 ---
const fileInput = ref(null)
const isDragging = ref(false)
const scanImageSrc = ref('')
const scanResult = ref('')
const scanError = ref('')

const handleModeChange = (targetMode) => {
  mode.value = targetMode
}

const triggerFileInput = () => {
  if (fileInput.value) {
    fileInput.value.click()
  }
}

const handleFileChange = (e) => {
  const files = e.target.files
  if (files && files.length > 0) {
    processFile(files[0])
  }
}

const handleDragOver = (e) => {
  e.preventDefault()
  isDragging.value = true
}

const handleDragLeave = () => {
  isDragging.value = false
}

const handleDrop = (e) => {
  e.preventDefault()
  isDragging.value = false
  const files = e.dataTransfer.files
  if (files && files.length > 0) {
    processFile(files[0])
  }
}

// 处理全局粘贴事件
const handleGlobalPaste = (e) => {
  if (mode.value !== 'scan') return
  const items = e.clipboardData?.items
  if (!items) return
  for (const item of items) {
    if (item.type.indexOf('image') !== -1) {
      const file = item.getAsFile()
      if (file) {
        processFile(file)
        e.preventDefault()
      }
    }
  }
}

const processFile = (file) => {
  if (!file.type.startsWith('image/')) {
    ElMessage.error('请上传有效的图片文件')
    return
  }

  scanResult.value = ''
  scanError.value = ''

  const reader = new FileReader()
  reader.onload = (e) => {
    const dataUrl = e.target.result
    scanImageSrc.value = dataUrl

    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        scanError.value = '内部错误：无法创建 Canvas 上下文'
        return
      }
      canvas.width = img.width
      canvas.height = img.height
      ctx.drawImage(img, 0, 0)

      try {
        const imageData = ctx.getImageData(0, 0, img.width, img.height)
        const code = jsQR(imageData.data, imageData.width, imageData.height)

        if (code) {
          scanResult.value = code.data
          ElMessage.success('解析成功！')
        } else {
          scanError.value = '未在图片中检测到有效的二维码，请确保二维码清晰且无遮挡'
        }
      } catch (err) {
        scanError.value = `解析错误: ${err.message}`
      }
    }
    img.onerror = () => {
      scanError.value = '加载图片失败，请重试'
    }
    img.src = dataUrl
  }
  reader.readAsDataURL(file)
}

const handleClearScan = () => {
  scanImageSrc.value = ''
  scanResult.value = ''
  scanError.value = ''
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

const copyToClipboard = (text) => {
  if (!text) return
  navigator.clipboard.writeText(text).then(
    () => ElMessage.success('已复制到剪贴板'),
    () => ElMessage.error('复制失败，请手动选择复制'),
  )
}

const isLink = (text) => {
  if (!text) return false
  return text.startsWith('http://') || text.startsWith('https://')
}

// 绑定与解绑全局粘贴监听
onMounted(() => {
  window.addEventListener('paste', handleGlobalPaste)
})

onUnmounted(() => {
  window.removeEventListener('paste', handleGlobalPaste)
})
</script>
<style scoped>
.qr-tool { display: grid; gap: 20px; max-width: 980px; margin: 0 auto; color: var(--text-main); }
.qr-mode-switch { display: inline-flex; justify-self: start; padding: 4px; border: 1px solid var(--border-glass); border-radius: 10px; background: var(--bg-soft); }
.qr-mode-switch button { border: 0; border-radius: 7px; padding: 9px 18px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .76rem; font-weight: 700; }
.qr-mode-switch button.active { background: var(--primary); color: #fff; box-shadow: 0 4px 12px rgba(37,99,235,.22); }
.qr-layout { display: grid; grid-template-columns: minmax(0, 1.05fr) minmax(280px, .95fr); gap: 16px; }
.qr-scan-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(280px, .8fr); gap: 16px; align-items: start; }
.qr-panel { min-width: 0; padding: 20px; border: 1px solid var(--border-glass); border-radius: 14px; background: var(--bg-panel); }
.panel-heading { display: flex; align-items: center; gap: 11px; margin-bottom: 18px; }
.panel-icon { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 9px; background: color-mix(in srgb, var(--primary) 13%, transparent); color: var(--primary); font-size: 1.2rem; font-weight: 800; }
.panel-heading h3 { margin: 0 0 3px; font-size: 1rem; }
.panel-heading p { margin: 0; color: var(--text-muted); font-size: .72rem; }
.qr-panel textarea { display: block; width: 100%; box-sizing: border-box; resize: vertical; border: 1px solid var(--border-glass); border-radius: 9px; padding: 12px; outline: none; background: var(--bg-input); color: var(--text-main); font: .76rem/1.65 var(--font-mono); }
.qr-panel textarea:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent); }
.field-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 12px; color: var(--text-dim); font-size: .7rem; }
.field-footer > div { display: flex; gap: 8px; }
.button { min-height: 34px; border: 1px solid var(--border-glass); border-radius: 8px; padding: 7px 13px; cursor: pointer; font: inherit; font-size: .72rem; font-weight: 700; transition: .2s; }
.button-ghost { background: transparent; color: var(--text-muted); }
.button-ghost:hover { background: var(--bg-hover-weak); color: var(--text-main); }
.button-primary { border-color: var(--primary); background: var(--primary); color: white; }
.button-primary:hover { background: var(--primary-hover); }
.qr-preview-panel { display: grid; min-height: 300px; place-items: center; align-content: center; gap: 14px; border-style: dashed; text-align: center; }
.qr-preview-panel.ready { border-style: solid; background: var(--bg-elevated); }
.qr-code { padding: 12px; border-radius: 10px; background: white; box-shadow: 0 12px 30px rgba(2,8,23,.18); }
.empty-state { display: grid; justify-items: center; gap: 6px; color: var(--text-muted); }
.empty-icon { display: grid; place-items: center; width: 54px; height: 54px; margin-bottom: 6px; border: 1px solid var(--border-glass); border-radius: 14px; color: var(--primary); font-size: 1.8rem; }
.empty-state strong { font-size: .8rem; color: var(--text-main); }.empty-state p, .preview-hint { margin: 0; font-size: .68rem; color: var(--text-dim); }
.preview-hint { width: 100%; padding-top: 10px; border-top: 1px solid var(--border-glass); text-align: center; }
.drop-zone { position: relative; display: grid; min-height: 260px; place-items: center; align-content: center; gap: 7px; overflow: hidden; border: 1px dashed var(--border-glass); border-radius: 11px; background: var(--bg-input); text-align: center; cursor: pointer; }
.drop-zone.dragging { border-color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, var(--bg-input)); }.drop-zone.filled { display: flex; }.drop-zone img { max-width: 100%; max-height: 300px; object-fit: contain; }.drop-overlay { position: absolute; inset: 0; display: grid; place-items: center; background: rgba(2,8,23,.58); color: white; opacity: 0; transition: .2s; }.drop-zone:hover .drop-overlay { opacity: 1; }
.upload-icon { color: var(--primary); font-size: 2rem; }.drop-zone strong { font-size: .8rem; }.drop-zone p { margin: 0; color: var(--text-muted); font-size: .7rem; }
.result-panel { display: grid; gap: 12px; }.result-heading { display: flex; justify-content: space-between; color: var(--accent-emerald); font-size: .8rem; }.result-heading.error { color: var(--accent-rose); }.result-heading button, .result-panel a { border: 0; background: none; color: var(--primary); cursor: pointer; font: inherit; font-size: .7rem; font-weight: 700; }.error-text { margin: 0; color: var(--accent-rose); font-size: .74rem; line-height: 1.6; }.full-button { width: 100%; }
@media (max-width: 720px) { .qr-layout, .qr-scan-layout { grid-template-columns: 1fr; } .qr-panel { padding: 15px; } .qr-preview-panel { min-height: 240px; } }
@media (max-width: 420px) { .field-footer { align-items: stretch; flex-direction: column; }.field-footer > div { justify-content: stretch; }.field-footer .button { flex: 1; } }
</style>
