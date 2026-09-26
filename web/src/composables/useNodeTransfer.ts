import { ref, computed, watch } from 'vue'
import { copyText } from '../utils'
import {
  type TransferDirection,
  type BaseOutboundConfig,
  type ConfigDetailItem,
  detectDirection,
  tryParseJsonWithArrayFallback,
  parseJsonConfigs,
  parseLinksToConfigs,
  buildShareLink,
  buildShareLinks,
  extractConfigDetails,
  getAllHandlers,
  getHandlerByProtocol,
} from '../tools/node-transfer'

export interface TransferNodeResult {
  index: number
  config: BaseOutboundConfig
  shareLink: string
  shareJson: string
  details: ConfigDetailItem[]
}

export interface UseNodeTransferOptions {
  onSuccess?: (message: string) => void
  onError?: (message: string) => void
}

export function useNodeTransfer(options: UseNodeTransferOptions = {}) {
  const { onSuccess, onError } = options

  const direction = ref<TransferDirection>('json2link')
  const inputText = ref('')
  const activeNodeIndex = ref(0)

  const supportedHandlers = getAllHandlers()

  // 优先根据输入内容自动判断转换方向
  const conversionDirection = computed<TransferDirection>(() =>
    detectDirection(inputText.value, direction.value),
  )

  // 转换解析结果 (支持多节点列表)
  const parseResult = computed(() => {
    const text = inputText.value.trim()
    if (!text) {
      return { configs: [], error: '' }
    }

    if (conversionDirection.value === 'json2link') {
      const { configs, error } = parseJsonConfigs(text)
      return { configs, error: error || '' }
    } else {
      const { configs, error } = parseLinksToConfigs(text)
      return { configs, error: error || '' }
    }
  })

  const parsedConfigs = computed(() => parseResult.value.configs)
  const parseError = computed(() => parseResult.value.error)
  const nodeCount = computed(() => parsedConfigs.value.length)

  // 当解析出的节点数量变化时，确保 activeNodeIndex 不越界
  watch(
    () => parsedConfigs.value.length,
    (len) => {
      if (activeNodeIndex.value >= len) {
        activeNodeIndex.value = Math.max(0, len - 1)
      }
    },
  )

  // 每个独立节点的转换结果（文本 + 二维码 + 详情）
  const nodeResults = computed<TransferNodeResult[]>(() => {
    if (parsedConfigs.value.length === 0 || parseError.value) return []
    return parsedConfigs.value.map((cfg, idx) => ({
      index: idx + 1,
      config: cfg,
      shareLink: buildShareLink(cfg),
      shareJson: JSON.stringify(cfg, null, 2),
      details: extractConfigDetails(cfg),
    }))
  })

  // 当前激活选中的节点（用于底部查看完整参数列表）
  const activeConfig = computed(() => {
    if (parsedConfigs.value.length === 0) return null
    return parsedConfigs.value[activeNodeIndex.value] || parsedConfigs.value[0]
  })

  // 输出文本：所有节点的分享链接 (换行隔开)
  const allShareLinksText = computed(() => {
    if (parsedConfigs.value.length === 0 || parseError.value) return ''
    return buildShareLinks(parsedConfigs.value)
  })

  // 输出文本：所有节点的 JSON
  const allShareJsonText = computed(() => {
    if (parsedConfigs.value.length === 0 || parseError.value) return ''
    if (parsedConfigs.value.length === 1) {
      return JSON.stringify(parsedConfigs.value[0], null, 2)
    }
    return JSON.stringify(parsedConfigs.value, null, 2)
  })

  // 当前选中节点的配置参数详情
  const configDetails = computed(() => {
    if (!activeConfig.value || parseError.value) return []
    return extractConfigDetails(activeConfig.value)
  })

  const handlePasteSample = (protocol: string) => {
    const handler = getHandlerByProtocol(protocol)
    if (!handler) return
    activeNodeIndex.value = 0
    if (conversionDirection.value === 'json2link') {
      inputText.value = handler.sample.json
    } else {
      inputText.value = handler.sample.link
    }
  }

  const handleClear = () => {
    inputText.value = ''
    activeNodeIndex.value = 0
  }

  const handleDirectionChange = (newDir: TransferDirection) => {
    direction.value = newDir
    inputText.value = ''
    activeNodeIndex.value = 0
  }

  const handleFormat = () => {
    try {
      const text = inputText.value.trim()
      if (!text) return
      const parsed = tryParseJsonWithArrayFallback(text)
      inputText.value = JSON.stringify(parsed, null, 2)
    } catch (e: any) {
      onError?.(`格式化失败: ${e.message}`)
    }
  }

  const handleCopy = async (text: string, customSuccessMsg?: string) => {
    if (!text) return
    const success = await copyText(text)
    if (success) {
      onSuccess?.(customSuccessMsg || '已复制到剪贴板')
    } else {
      onError?.('复制失败，请手动选择复制')
    }
  }

  const handleCopyAll = () => {
    const text = conversionDirection.value === 'json2link' ? allShareLinksText.value : allShareJsonText.value
    handleCopy(text, `已复制全部 ${nodeCount.value} 个节点`)
  }

  return {
    direction,
    inputText,
    activeNodeIndex,
    nodeCount,
    supportedHandlers,
    conversionDirection,
    parsedConfigs,
    nodeResults,
    activeConfig,
    parseError,
    allShareLinksText,
    allShareJsonText,
    configDetails,
    handlePasteSample,
    handleClear,
    handleDirectionChange,
    handleFormat,
    handleCopy,
    handleCopyAll,
  }
}
