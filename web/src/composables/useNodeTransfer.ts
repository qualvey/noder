import { ref, computed, watch } from 'vue'
import { copyText } from '../utils'
import {
  type TransferDirection,
  type BaseOutboundConfig,
  type ConfigDetailItem,
  type NodeMeta,
  detectDirection,
  tryParseJsonWithArrayFallback,
  parseJsonConfigs,
  parseLinksToConfigs,
  buildShareLink,
  buildShareLinks,
  buildDaeNodeBlock,
  extractConfigDetails,
  getAllHandlers,
  getHandlerByProtocol,
  toNodeMeta,
  toSingBoxConfig,
  buildSingBoxConfig,
  toXrayConfig,
} from '../tools/node-transfer'

export interface TransferNodeResult {
  index: number
  config: BaseOutboundConfig
  meta: NodeMeta
  shareLink: string
  shareJson: string
  singBoxError: string
  daeNode: string
  xrayJson: string
  xrayError: string
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
      return { configs: [], error: '', warnings: [] as string[] }
    }

    if (conversionDirection.value === 'json2link') {
      const { configs, error, warnings } = parseJsonConfigs(text)
      return { configs, error: error || '', warnings }
    } else {
      const { configs, error, warnings } = parseLinksToConfigs(text)
      return { configs, error: error || '', warnings }
    }
  })

  const parsedConfigs = computed(() => parseResult.value.configs)
  const parseError = computed(() => parseResult.value.error)
  const parseWarnings = computed(() => parseResult.value.warnings)
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
    return parsedConfigs.value.map((cfg, idx) => {
      const meta = toNodeMeta(cfg)

      let singBoxConfig: BaseOutboundConfig = cfg
      let singBoxError = ''
      try {
        singBoxConfig = toSingBoxConfig(meta)
      } catch (e: any) {
        singBoxError = e.message || '无法生成 sing-box 配置'
        singBoxConfig = cfg
      }

      let xrayJson = ''
      let xrayError = ''
      try {
        xrayJson = JSON.stringify(toXrayConfig(meta), null, 2)
      } catch (e: any) {
        xrayError = e.message || '无法生成 Xray 配置'
      }

      let shareLink = ''
      try {
        shareLink = buildShareLink(singBoxConfig)
      } catch {
        shareLink = buildShareLink(cfg)
      }

      const daeNode = shareLink
        ? buildDaeNodeBlock([{ ...singBoxConfig, tag: singBoxConfig.tag || cfg.tag || `node-${idx + 1}` }])
        : ''

      return {
        index: idx + 1,
        config: singBoxConfig,
        meta,
        shareLink,
        shareJson: singBoxError
          ? JSON.stringify({ error: singBoxError, note: '该节点可能仅支持 Xray/DAE 输出' }, null, 2)
          : JSON.stringify(singBoxConfig, null, 2),
        singBoxError,
        daeNode,
        xrayJson,
        xrayError,
        details: extractConfigDetails(singBoxConfig),
      }
    })
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

  const allXrayJsonText = computed(() => {
    if (nodeResults.value.length === 0 || parseError.value) return ''
    const outbounds: unknown[] = []
    for (const result of nodeResults.value) {
      if (!result.xrayJson) continue
      try {
        const config = JSON.parse(result.xrayJson)
        outbounds.push(...(config.outbounds || []))
      } catch {
        // 单节点错误已展示在结果项中，这里跳过即可。
      }
    }
    return outbounds.length ? JSON.stringify({ outbounds }, null, 2) : ''
  })

  const allSingBoxJsonText = computed(() => {
    if (nodeResults.value.length === 0 || parseError.value) return ''
    const configs = nodeResults.value
      .filter((result) => !result.singBoxError)
      .map((result) => result.config)
    if (configs.length === 0) {
      return JSON.stringify(
        { error: '当前节点不支持生成 sing-box 完整配置（例如 Xray xhttp）', nodes: nodeResults.value.length },
        null,
        2,
      )
    }
    return JSON.stringify(buildSingBoxConfig(configs), null, 2)
  })

  const allDaeText = computed(() => {
    if (nodeResults.value.length === 0 || parseError.value) return ''
    return buildDaeNodeBlock(nodeResults.value.map((result) => result.config))
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
    allXrayJsonText,
    allSingBoxJsonText,
    allDaeText,
    parseWarnings,
    configDetails,
    handlePasteSample,
    handleClear,
    handleDirectionChange,
    handleFormat,
    handleCopy,
    handleCopyAll,
  }
}
