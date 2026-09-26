import { ref, computed } from 'vue'
import { copyText } from '../utils'
import {
  type TransferDirection,
  detectDirection,
  parseJsonConfig,
  parseLinkToConfig,
  buildShareLink,
  extractConfigDetails,
  getAllHandlers,
  getHandlerByProtocol,
} from '../tools/node-transfer'

export interface UseNodeTransferOptions {
  onSuccess?: (message: string) => void
  onError?: (message: string) => void
}

export function useNodeTransfer(options: UseNodeTransferOptions = {}) {
  const { onSuccess, onError } = options

  const direction = ref<TransferDirection>('json2link')
  const inputText = ref('')

  const supportedHandlers = getAllHandlers()

  // 优先根据输入内容自动判断转换方向
  const conversionDirection = computed<TransferDirection>(() =>
    detectDirection(inputText.value, direction.value),
  )

  // 转换解析结果 (包含提取的配置和错误信息)
  const parseResult = computed(() => {
    const text = inputText.value.trim()
    if (!text) {
      return { config: null, error: '' }
    }

    if (conversionDirection.value === 'json2link') {
      const { config, error } = parseJsonConfig(text)
      return { config, error: error || '' }
    } else {
      try {
        const config = parseLinkToConfig(text)
        return { config, error: '' }
      } catch (e: any) {
        return { config: null, error: `链接解析错误: ${e.message}` }
      }
    }
  })

  const parsedConfig = computed(() => parseResult.value.config)
  const parseError = computed(() => parseResult.value.error)

  const shareLink = computed(() => {
    if (!parsedConfig.value || parseError.value) return ''
    return buildShareLink(parsedConfig.value)
  })

  const shareLinkJson = computed(() => {
    if (!parsedConfig.value || parseError.value) return ''
    return JSON.stringify(parsedConfig.value, null, 2)
  })

  const configDetails = computed(() => {
    if (!parsedConfig.value || parseError.value) return []
    return extractConfigDetails(parsedConfig.value)
  })

  const handlePasteSample = (protocol: string) => {
    const handler = getHandlerByProtocol(protocol)
    if (!handler) return
    if (conversionDirection.value === 'json2link') {
      inputText.value = handler.sample.json
    } else {
      inputText.value = handler.sample.link
    }
  }

  const handleClear = () => {
    inputText.value = ''
  }

  const handleDirectionChange = (newDir: TransferDirection) => {
    direction.value = newDir
    inputText.value = ''
  }

  const handleFormat = () => {
    try {
      if (!inputText.value.trim()) return
      const parsed = JSON.parse(inputText.value)
      inputText.value = JSON.stringify(parsed, null, 2)
    } catch (e: any) {
      onError?.(`格式化失败: ${e.message}`)
    }
  }

  const handleCopy = async (text: string) => {
    if (!text) return
    const success = await copyText(text)
    if (success) {
      onSuccess?.('已复制到剪贴板')
    } else {
      onError?.('复制失败，请手动选择复制')
    }
  }

  return {
    direction,
    inputText,
    supportedHandlers,
    conversionDirection,
    parsedConfig,
    parseError,
    shareLink,
    shareLinkJson,
    configDetails,
    handlePasteSample,
    handleClear,
    handleDirectionChange,
    handleFormat,
    handleCopy,
  }
}
