import { ref } from 'vue'

/**
 * 图片输入：拖拽 / 点击选择 / 全局粘贴 / file input。
 * 只负责把图片解码为 HTMLImageElement 并持有；解码完成与清空时的
 * 画布副作用（图层初始化、重合成、笔触清理）由调用方通过回调注入，
 * 与马赛克/矩形/水印引擎解耦。
 */
export function useImageIntake(opts: {
  onImageLoaded: () => void
  onReset: () => void
  onInvalidType: () => void
}) {
  const originalImage = ref<HTMLImageElement | null>(null)
  const originalFileName = ref<string>('')
  const isDragging = ref(false)
  const fileInput = ref<HTMLInputElement | null>(null)

  function onDragOver(e: DragEvent) {
    e.preventDefault()
    isDragging.value = true
  }

  function onDragLeave() {
    isDragging.value = false
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    isDragging.value = false
    const droppedFiles = e.dataTransfer?.files
    if (droppedFiles && droppedFiles.length > 0 && droppedFiles[0]) {
      processFile(droppedFiles[0])
    }
  }

  // 图片区域支持 Ctrl+V / 右键粘贴图片
  // 规则：剪贴板含图片时接管；若焦点在文本框且剪贴板同时含文本，则放行让文本框正常粘贴文本
  function onPasteImage(e: ClipboardEvent) {
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

  function triggerFileSelect() {
    fileInput.value?.click()
  }

  function onFileChange(e: Event) {
    const target = e.target as HTMLInputElement
    const files = target.files
    if (files && files.length > 0 && files[0]) {
      processFile(files[0])
    }
  }

  // 处理图片文件
  function processFile(file: File) {
    if (!file.type.startsWith('image/')) {
      opts.onInvalidType()
      return
    }
    originalFileName.value = file.name

    const reader = new FileReader()
    reader.onload = (e) => {
      const img = new Image()
      img.onload = () => {
        originalImage.value = img
        opts.onImageLoaded()
      }
      img.src = e.target?.result as string
    }
    reader.readAsDataURL(file)
  }

  // 移除当前图片
  function clearImage() {
    originalImage.value = null
    originalFileName.value = ''
    if (fileInput.value) {
      fileInput.value.value = ''
    }
    opts.onReset()
  }

  return {
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
    processFile,
    clearImage,
  }
}
