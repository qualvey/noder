<script setup lang="ts">
import { computed, ref } from 'vue'
import { copyText } from '../utils'

const props = defineProps<{ content: string; disabled?: boolean }>()
const ttl = ref(0)
const ttlOptions = ref<number[]>([])
const loading = ref(false)
const url = ref('')
const expiresAt = ref('')
const error = ref('')
const copied = ref(false)

const canCreate = computed(() => Boolean(props.content) && !props.disabled && !loading.value)

function formatTtl(seconds: number) {
  if (seconds % (24 * 60 * 60) === 0) return `${seconds / (24 * 60 * 60)} 天`
  if (seconds % (60 * 60) === 0) return `${seconds / (60 * 60)} 小时`
  return `${Math.round(seconds / 60)} 分钟`
}

async function loadTtlOptions() {
  try {
    const response = await fetch('./api/subscriptions')
    if (!response.ok) throw new Error('无法读取订阅配置')
    const result = (await response.json()) as { ttl_options?: number[]; default_ttl?: number }
    ttlOptions.value = result.ttl_options || []
    ttl.value = result.default_ttl && ttlOptions.value.includes(result.default_ttl)
      ? result.default_ttl
      : (ttlOptions.value[0] || 0)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '无法读取订阅配置'
  }
}

loadTtlOptions()

async function createSubscription() {
  if (!canCreate.value) return
  loading.value = true
  error.value = ''
  url.value = ''
  try {
    const response = await fetch('./api/subscriptions', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ content: props.content, ttl: ttl.value }),
    })
    const result = (await response.json()) as { url?: string; expires_at?: string; error?: string }
    if (!response.ok || !result.url) throw new Error(result.error || `创建失败 (${response.status})`)
    url.value = result.url
    expiresAt.value = result.expires_at || ''
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '创建短期订阅失败'
  } finally {
    loading.value = false
  }
}

async function copyUrl() {
  copied.value = await copyText(url.value)
  window.setTimeout(() => (copied.value = false), 1800)
}
</script>

<template>
  <section class="subscription-share">
    <div class="subscription-heading">
      <div>
        <span class="card-kicker">OPTIONAL</span>
        <h3>生成短期订阅链接</h3>
        <p>配置会临时保存到 Cloudflare KV，过期后自动删除。</p>
      </div>
      <div class="subscription-controls">
        <label>有效期
          <select v-model.number="ttl" :disabled="!ttlOptions.length">
            <option v-for="option in ttlOptions" :key="option" :value="option">{{ formatTtl(option) }}</option>
          </select>
        </label>
        <button :disabled="!canCreate" @click="createSubscription">
          {{ loading ? '生成中…' : '生成链接' }}
        </button>
      </div>
    </div>
    <div v-if="url" class="subscription-result">
      <input readonly :value="url" aria-label="短期订阅链接" />
      <button @click="copyUrl">{{ copied ? '已复制' : '复制链接' }}</button>
      <small>过期时间：{{ new Date(expiresAt).toLocaleString() }}</small>
    </div>
    <p v-if="error" class="subscription-error">{{ error }}</p>
  </section>
</template>

<style scoped>
.subscription-share { padding: 18px; border: 1px solid var(--border-glass); border-radius: 14px; background: var(--bg-panel); }
.subscription-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.subscription-heading h3 { margin: 4px 0; font-size: .9rem; }
.subscription-heading p, .subscription-result small { margin: 0; color: var(--text-muted); font-size: .68rem; }
.card-kicker { color: var(--primary); font-size: .62rem; font-weight: 800; letter-spacing: .13em; }
.subscription-controls, .subscription-result { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.subscription-controls label { color: var(--text-muted); font-size: .68rem; }
select, .subscription-result input { border: 1px solid var(--border-glass); border-radius: 6px; padding: 7px 8px; background: var(--bg-input); color: var(--text-main); font: inherit; font-size: .68rem; }
.subscription-controls button, .subscription-result button { border: 1px solid var(--primary); border-radius: 6px; padding: 7px 10px; background: var(--primary); color: white; cursor: pointer; font: inherit; font-size: .68rem; font-weight: 700; }
.subscription-controls button:disabled { cursor: not-allowed; opacity: .45; }
.subscription-result { margin-top: 14px; }
.subscription-result input { flex: 1; min-width: 180px; font-family: var(--font-mono); }
.subscription-result small { flex-basis: 100%; }
.subscription-error { margin: 12px 0 0; color: var(--accent-rose); font-size: .7rem; }
@media (max-width: 600px) { .subscription-heading { align-items: flex-start; flex-direction: column; } }
</style>
