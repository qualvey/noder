<script setup lang="ts">
import { computed, inject, onMounted, ref } from 'vue'
import { useNodeTransfer } from '../composables/useNodeTransfer'
import SubscriptionShare from '../components/SubscriptionShare.vue'

const toast = inject<(message: string, type?: 'info' | 'error' | 'success' | 'warning') => void>('toast')
const {
  inputText,
  nodeCount,
  parseError,
  parseWarnings,
  allSingBoxJsonText,
  parsedConfigs,
  handlePasteSample,
  handleClear,
  supportedHandlers,
} = useNodeTransfer({
  onError: (message) => toast?.(message, 'error'),
})

interface TemplateOption {
  id: string
  name: string
  file: string
}

const templates = ref<TemplateOption[]>([])
const selectedTemplateId = ref('')
const templateError = ref('')
const templateContent = ref<Record<string, unknown> | null>(null)

const renderedContent = computed(() => {
  if (!allSingBoxJsonText.value || !templateContent.value) return ''
  try {
    const template = JSON.parse(JSON.stringify(templateContent.value)) as Record<string, unknown>
    const templateOutbounds = Array.isArray(template.outbounds) ? template.outbounds : []
    const nodeConfig = JSON.parse(allSingBoxJsonText.value) as { outbounds?: unknown[] }
    const nodeOutbounds = Array.isArray(nodeConfig.outbounds) ? nodeConfig.outbounds : []
    const nodeTags = parsedConfigs.value.map((config) => config.tag).filter(Boolean)

    let proxy = templateOutbounds.find(
      (item): item is Record<string, unknown> =>
        !!item && typeof item === 'object' && (item as Record<string, unknown>).tag === 'Proxy',
    )
    let urltest = templateOutbounds.find(
      (item): item is Record<string, unknown> =>
        !!item && typeof item === 'object' && (item as Record<string, unknown>).tag === 'urltest',
    )

    if (!proxy) {
      proxy = {
        tag: 'Proxy',
        type: 'selector',
        interrupt_exist_connections: true,
        outbounds: ['urltest'],
      }
      templateOutbounds.push(proxy)
    }
    if (!urltest) {
      urltest = { tag: 'urltest', type: 'urltest', outbounds: [] }
      templateOutbounds.push(urltest)
    }

    const addTags = (target: Record<string, unknown>) => {
      if (!target) return
      const existing = Array.isArray(target.outbounds) ? target.outbounds : []
      target.outbounds = [...new Set([...existing, ...nodeTags])]
    }
    addTags(proxy)
    addTags(urltest)

    // 模板自身的控制出站保留，节点出站追加到根 outbounds 末尾。
    template.outbounds = [...templateOutbounds, ...nodeOutbounds]
    return JSON.stringify(template, null, 2)
  } catch {
    return ''
  }
})

async function loadTemplate(id: string) {
  const option = templates.value.find((item) => item.id === id)
  if (!option) return
  templateError.value = ''
  try {
    const response = await fetch(`./assets/templates/${option.file}`)
    if (!response.ok) throw new Error(`模板加载失败 (${response.status})`)
    templateContent.value = (await response.json()) as Record<string, unknown>
  } catch (cause) {
    templateContent.value = null
    templateError.value = cause instanceof Error ? cause.message : '模板加载失败'
  }
}

onMounted(async () => {
  try {
    const response = await fetch('./assets/templates/templates.json')
    if (!response.ok) throw new Error(`模板列表加载失败 (${response.status})`)
    templates.value = (await response.json()) as TemplateOption[]
    selectedTemplateId.value = templates.value[0]?.id || ''
    if (selectedTemplateId.value) await loadTemplate(selectedTemplateId.value)
  } catch (cause) {
    templateError.value = cause instanceof Error ? cause.message : '模板列表加载失败'
  }
})
</script>

<template>
  <section class="subscription-tool">
    <header class="tool-intro">
      <div>
        <span class="eyebrow">SUBSCRIPTION PUBLISHER</span>
        <h2>短期订阅生成</h2>
        <p>粘贴 VLESS / TUIC 链接，在浏览器完成转换后生成临时 sing-box 订阅地址。</p>
      </div>
      <label class="template-picker">模板
        <select v-model="selectedTemplateId" @change="loadTemplate(selectedTemplateId)">
          <option v-for="template in templates" :key="template.id" :value="template.id">{{ template.name }}</option>
        </select>
      </label>
    </header>

    <section class="subscription-input card">
      <div class="card-heading">
        <div><span class="eyebrow">INPUT</span><h3>节点分享链接</h3></div>
        <div class="actions">
          <button v-for="handler in supportedHandlers" :key="handler.protocol" @click="handlePasteSample(handler.protocol)">
            {{ handler.displayName }} 示例
          </button>
          <button :disabled="!inputText" @click="handleClear">清空</button>
        </div>
      </div>
      <textarea v-model="inputText" placeholder="粘贴一条或多条 vless://、tuic:// 链接，每行一个…"></textarea>
      <footer><span>{{ inputText.length }} 个字符</span><span>支持多节点，重复 tag 会自动处理</span></footer>
    </section>

    <div v-if="parseWarnings.length" class="warning">{{ parseWarnings.join('；') }}</div>
    <div v-if="parseError" class="error">{{ parseError }}</div>
    <div v-if="templateError" class="error">{{ templateError }}</div>

    <!-- <section class="subscription-output card">
      <div class="card-heading"><div><span class="eyebrow">PREVIEW</span><h3>生成内容</h3></div><strong v-if="nodeCount">{{ nodeCount }} 个节点</strong></div>
      <textarea readonly :value="renderedContent" placeholder="输入有效节点后，这里会显示套用模板的 sing-box 配置…"></textarea>
    </section> -->

    <SubscriptionShare :content="renderedContent" :disabled="nodeCount === 0 || !!parseError || !templateContent" />
  </section>
</template>

<style scoped>
.subscription-tool { display: grid; gap: 16px; max-width: 980px; margin: 0 auto; color: var(--text-main); }
.tool-intro, .card-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.eyebrow { color: var(--primary); font-size: .62rem; font-weight: 800; letter-spacing: .14em; }
.tool-intro h2 { margin: 5px 0 4px; font-size: 1.25rem; }.tool-intro p { margin: 0; color: var(--text-muted); font-size: .74rem; }
.local-badge { padding: 6px 9px; border: 1px solid var(--border-glass); border-radius: 999px; color: var(--accent-emerald); font-size: .65rem; }
.template-picker { display: flex; align-items: center; gap: 8px; color: var(--text-muted); font-size: .68rem; }.template-picker select { border: 1px solid var(--border-glass); border-radius: 7px; padding: 7px 9px; background: var(--bg-input); color: var(--text-main); font: inherit; }
.card { padding: 18px; border: 1px solid var(--border-glass); border-radius: 14px; background: var(--bg-panel); }
.card-heading h3 { margin: 4px 0 0; font-size: .9rem; }.card-heading strong { color: var(--primary); font-size: .7rem; }
.actions { display: flex; flex-wrap: wrap; gap: 6px; }.actions button { border: 1px solid var(--border-glass); border-radius: 6px; padding: 6px 8px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .66rem; }
.actions button:hover:not(:disabled) { border-color: var(--primary); color: var(--primary); }.actions button:disabled { opacity: .4; cursor: not-allowed; }
textarea { display: block; width: 100%; box-sizing: border-box; resize: vertical; min-height: 190px; margin-top: 14px; padding: 12px; border: 1px solid var(--border-glass); border-radius: 9px; outline: none; background: var(--bg-input); color: var(--text-main); font: .72rem/1.65 var(--font-mono); }
textarea:focus { border-color: var(--primary); }.subscription-output textarea { min-height: 260px; }.card footer { display: flex; justify-content: space-between; margin-top: 9px; color: var(--text-dim); font-size: .64rem; }
.warning, .error { padding: 10px 12px; border-radius: 8px; font-size: .7rem; overflow-wrap: anywhere; }.warning { background: color-mix(in srgb, var(--accent-amber) 10%, transparent); color: var(--accent-amber); }.error { background: color-mix(in srgb, var(--accent-rose) 10%, transparent); color: var(--accent-rose); }
@media (max-width: 600px) { .tool-intro, .card-heading { align-items: flex-start; flex-direction: column; }.actions { justify-content: flex-start; }.card { padding: 14px; } }
</style>
