<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { ServiceReleaseLog, ServiceReleaseLogStats } from '../types'
import { copyText } from '../utils'
import { ToastKey } from '../injectionKeys'

const { t } = useI18n()
const toast = inject(ToastKey)

const loading = ref(false)
const items = ref<ServiceReleaseLog[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const totalPages = ref(0)

const stats = ref<ServiceReleaseLogStats | null>(null)

// Filters — strict, typed, synced to query
const filters = ref({
  subscription_id: '',
  endpoint_id: '',
  q: '',
  http_status: '' as string,
  error_code: '',
  delivered: '' as '' | 'true' | 'false',
  agent_online: '' as '' | 'true' | 'false',
  since: '',
  until: '',
})

const detail = ref<ServiceReleaseLog | null>(null)
const showDetail = ref(false)

function formatTime(v?: string | null) {
  if (!v) return '-'
  try { return new Date(v).toLocaleString() } catch { return v }
}

function badgeFor(row: ServiceReleaseLog) {
  if (row.http_status === 200 && row.business_code === 0) return { label: t('releaseLogs.badgeSuccess'), cls: 'active' }
  if (row.error_code === 'BINDING_MISSING') return { label: t('releaseLogs.badgeBindingMissing'), cls: 'inactive' }
  if (row.business_code === 40402) return { label: t('releaseLogs.badgeNotFound'), cls: 'inactive' }
  if (row.http_status >= 500) return { label: `${row.http_status}`, cls: 'inactive' }
  return { label: row.error_code || `${row.http_status}/${row.business_code}`, cls: 'inactive' }
}

const hasFilters = computed(() => Object.values(filters.value).some(v => String(v).trim() !== ''))

async function fetchPage(p = page.value) {
  loading.value = true
  page.value = p
  try {
    const data = await api.releaseLogs.list({
      page: p,
      limit: limit.value,
      subscription_id: filters.value.subscription_id.trim() || undefined,
      endpoint_id: filters.value.endpoint_id.trim() ? Number(filters.value.endpoint_id.trim()) : undefined,
      q: filters.value.q.trim() || undefined,
      http_status: filters.value.http_status ? Number(filters.value.http_status) : undefined,
      error_code: filters.value.error_code.trim() || undefined,
      delivered: filters.value.delivered === '' ? undefined : filters.value.delivered === 'true',
      agent_online: filters.value.agent_online === '' ? undefined : filters.value.agent_online === 'true',
      since: filters.value.since || undefined,
      until: filters.value.until || undefined,
    })
    items.value = data.items ?? []
    total.value = data.total ?? 0
    totalPages.value = data.total_pages ?? Math.ceil(total.value / limit.value)
  } catch (e) { toast?.((e as Error).message, 'error') } finally { loading.value = false }
}

async function fetchStats() {
  try {
    stats.value = await api.releaseLogs.stats({ since: filters.value.since || undefined, until: filters.value.until || undefined })
  } catch { /* ignore */ }
}

function onSearch() { fetchPage(1); fetchStats() }
function onReset() {
  filters.value = { subscription_id: '', endpoint_id: '', q: '', http_status: '', error_code: '', delivered: '', agent_online: '', since: '', until: '' }
  fetchPage(1); fetchStats()
}
function onLimitChange(e: Event) {
  const v = Number((e.target as HTMLSelectElement).value)
  limit.value = v
  fetchPage(1)
}
async function openDetail(row: ServiceReleaseLog) {
  try {
    detail.value = await api.releaseLogs.get(row.id)
    showDetail.value = true
  } catch {
    detail.value = row
    showDetail.value = true
  }
}
async function copyId(v: string) {
  const ok = await copyText(v)
  toast?.(ok ? t('releaseLogs.copySuccess') : t('releaseLogs.copyFailed'), ok ? 'info' : 'error')
}

// debounce q input — mature UX
let qTimer: number | undefined
watch(() => filters.value.q, () => {
  window.clearTimeout(qTimer)
  qTimer = window.setTimeout(() => fetchPage(1), 380)
})

onMounted(() => { fetchPage(1); fetchStats() })
</script>

<template>
  <section class="tab-content logs-tab">
    <div class="section-header">
      <div class="section-title">{{ t('releaseLogs.title') }} <span class="api-path">{{ t('releaseLogs.apiPath') }}</span></div>
      <div class="section-actions">
        <span class="total-text">{{ t('releaseLogs.total', { total, totalPages }) }}</span>
        <button class="btn btn-secondary btn-sm" @click="onSearch">{{ t('releaseLogs.refresh') }}</button>
      </div>
    </div>

    <!-- Stats header — dashboard best practice -->
    <div v-if="stats" class="metrics-grid metrics-grid-flush">
      <div class="metric-card"><div class="metric-info"><h4>{{ t('releaseLogs.statsTotal') }}</h4><div class="value">{{ stats.total }}</div></div><div class="metric-icon icon-status">📊</div></div>
      <div class="metric-card"><div class="metric-info"><h4>{{ t('releaseLogs.statsSuccess') }}</h4><div class="value value-success">{{ stats.success }}</div></div><div class="metric-icon icon-node">✓</div></div>
      <div class="metric-card"><div class="metric-info"><h4>{{ t('releaseLogs.statsDelivered') }}</h4><div class="value">{{ stats.delivered }}</div></div><div class="metric-icon icon-user">🚀</div></div>
      <div class="metric-card"><div class="metric-info"><h4>{{ t('releaseLogs.statsBindingMissing') }}</h4><div class="value value-warning">{{ stats.binding_missing }}</div></div><div class="metric-icon icon-protocol">⚠</div></div>
    </div>

    <!-- Filters — fieldset + accessible labels, responsive grid -->
    <fieldset class="filter-panel">
      <legend class="sr-only">{{ t('releaseLogs.filterLegend') }}</legend>
      <div class="filter-grid">
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterSubscriptionId') }}</span>
          <input v-model="filters.subscription_id" class="form-control" :placeholder="t('releaseLogs.filterSubscriptionIdPlaceholder')" @keydown.enter="onSearch" />
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterEndpointId') }}</span>
          <input v-model="filters.endpoint_id" class="form-control" type="number" min="0" :placeholder="t('releaseLogs.filterEndpointIdPlaceholder')" @keydown.enter="onSearch" />
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterQuery') }}</span>
          <input v-model="filters.q" class="form-control" :placeholder="t('releaseLogs.filterQueryPlaceholder')" />
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterErrorCode') }}</span>
          <input v-model="filters.error_code" class="form-control" :placeholder="t('releaseLogs.filterErrorCodePlaceholder')" list="error-code-list" />
          <datalist id="error-code-list">
            <option value="BINDING_MISSING" />
            <option value="SUBSCRIPTION_NOT_FOUND" />
            <option value="ENDPOINT_NOT_FOUND" />
            <option value="SUBSCRIPTION_REVOKED" />
          </datalist>
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterHttpStatus') }}</span>
          <select v-model="filters.http_status" class="form-control">
            <option value="">{{ t('releaseLogs.filterHttpStatusAll') }}</option>
            <option value="200">{{ t('releaseLogs.filterHttpStatus200') }}</option>
            <option value="409">{{ t('releaseLogs.filterHttpStatus409') }}</option>
            <option value="404">{{ t('releaseLogs.filterHttpStatus404') }}</option>
            <option value="500">{{ t('releaseLogs.filterHttpStatus500') }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterDelivered') }}</span>
          <select v-model="filters.delivered" class="form-control">
            <option value="">{{ t('releaseLogs.filterDeliveredAll') }}</option>
            <option value="true">{{ t('releaseLogs.filterDeliveredTrue') }}</option>
            <option value="false">{{ t('releaseLogs.filterDeliveredFalse') }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterAgentOnline') }}</span>
          <select v-model="filters.agent_online" class="form-control">
            <option value="">{{ t('releaseLogs.filterAgentOnlineAll') }}</option>
            <option value="true">{{ t('releaseLogs.filterAgentOnlineTrue') }}</option>
            <option value="false">{{ t('releaseLogs.filterAgentOnlineFalse') }}</option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterSince') }}</span>
          <input v-model="filters.since" class="form-control" type="date" />
        </label>
        <label class="filter-field">
          <span>{{ t('releaseLogs.filterUntil') }}</span>
          <input v-model="filters.until" class="form-control" type="date" />
        </label>
      </div>
      <div class="filter-actions">
        <button class="btn btn-primary btn-sm" @click="onSearch" :disabled="loading">{{ t('releaseLogs.btnSearch') }}</button>
        <button class="btn btn-secondary btn-sm" @click="onReset" :disabled="loading">{{ t('releaseLogs.btnReset') }}</button>
        <span v-if="hasFilters" class="filter-applied">{{ t('releaseLogs.filterApplied') }}</span>
      </div>
    </fieldset>

    <!-- Table — semantic, scrollable, stable layout -->
    <div class="table-container">
      <table class="data-table" role="table" :aria-label="t('releaseLogs.title')">
        <thead>
          <tr>
            <th scope="col" class="col-id">{{ t('releaseLogs.colId') }}</th>
            <th scope="col" class="col-sub">{{ t('releaseLogs.colSubscriptionId') }}</th>
            <th scope="col" class="col-endpoint">{{ t('releaseLogs.colEndpoint') }}</th>
            <th scope="col" class="col-status">{{ t('releaseLogs.colStatus') }}</th>
            <th scope="col" class="col-narrow col-center">{{ t('releaseLogs.colDelivered') }}</th>
            <th scope="col" class="col-narrow col-center">{{ t('releaseLogs.colAgent') }}</th>
            <th scope="col" class="col-latency">{{ t('releaseLogs.colLatency') }}</th>
            <th scope="col" class="col-time">{{ t('releaseLogs.colTime') }}</th>
            <th scope="col" class="col-action">{{ t('releaseLogs.colAction') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="9" class="state-cell">
              <span class="spinner spinner-inline"></span>
              {{ t('common.loading') }}
            </td>
          </tr>
          <tr v-else-if="!items.length">
            <td colspan="9" class="state-cell-lg">
              {{ t('releaseLogs.noLogs') }}{{ hasFilters ? t('releaseLogs.noLogsWithFilters') : '' }}
            </td>
          </tr>
          <tr v-for="row in items" :key="row.id">
            <td><code class="mono-id">{{ row.id }}</code></td>
            <td>
              <code class="mono-sub" :title="row.subscription_id">{{ row.subscription_id.slice(0,8) }}…</code>
              <button class="btn btn-secondary btn-sm copy-sub-btn" @click="copyId(row.subscription_id)" :aria-label="t('releaseLogs.copySubId')">⎘</button>
            </td>
            <td class="mono-endpoint">{{ row.endpoint_id || '-' }}</td>
            <td><span class="status-badge" :class="badgeFor(row).cls"><span class="status-dot"></span>{{ badgeFor(row).label }}</span><div v-if="row.error_code" class="error-code">{{ row.error_code }}</div></td>
            <td class="col-center"><span :class="row.delivered ? 'text-success' : 'text-muted'">{{ row.delivered ? t('releaseLogs.delivered') : t('releaseLogs.notDelivered') }}</span></td>
            <td class="col-center"><span :class="row.agent_online ? 'text-success' : 'text-muted'">{{ row.agent_online ? t('releaseLogs.online') : t('releaseLogs.offline') }}</span></td>
            <td class="muted-sm">{{ row.latency_ms }}ms</td>
            <td class="muted-xs"><time :datetime="row.created_at">{{ formatTime(row.created_at) }}</time></td>
            <td><button class="btn btn-secondary btn-sm" @click="openDetail(row)">{{ t('releaseLogs.btnDetail') }}</button></td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination — mature: page numbers, limit selector, prev/next -->
    <div class="pagination-bar">
      <div class="pagination-info">{{ t('releaseLogs.pageInfo', { page, totalPages }) }}
        <select :value="limit" @change="onLimitChange" class="form-control limit-select">
          <option :value="20">20</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
      </div>
      <div class="pagination-actions">
        <button class="btn btn-secondary btn-sm" :disabled="page<=1 || loading" @click="fetchPage(page-1)">{{ t('releaseLogs.btnPrev') }}</button>
        <span class="page-text">{{ page }} / {{ totalPages || 1 }}</span>
        <button class="btn btn-secondary btn-sm" :disabled="page>=totalPages || loading" @click="fetchPage(page+1)">{{ t('releaseLogs.btnNext') }}</button>
      </div>
    </div>
  </section>

  <!-- Detail — native dialog pattern via overlay -->
  <div class="modal-overlay" :class="{active: showDetail}" @mousedown.self="showDetail=false" role="dialog" aria-modal="true">
    <div class="modal modal-detail">
      <div class="modal-header">
        <div class="modal-title">{{ t('releaseLogs.detailTitle', { id: detail?.id }) }}</div>
        <button class="modal-close" @click="showDetail=false" :aria-label="t('common.close')">&times;</button>
      </div>
      <div v-if="detail" class="detail-grid">
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailSubId') }}</span><code class="detail-code">{{ detail.subscription_id }}</code></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailEndpoint') }}</span><span>{{ detail.endpoint_id }} / {{ detail.credential_id || '-' }}</span></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailExternalRef') }}</span><span>{{ detail.external_ref || '-' }}</span></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailCaller') }}</span><span>{{ detail.caller_ip }} · {{ detail.request_id || '-' }}</span></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailResult') }}</span><span>HTTP {{ detail.http_status }} / 业务 {{ detail.business_code }} · {{ detail.error_code || t('releaseLogs.ok') }} · {{ detail.delivered ? t('releaseLogs.delivered') : t('releaseLogs.notDelivered') }} · Agent {{ detail.agent_online ? t('releaseLogs.online') : t('releaseLogs.offline') }} · {{ detail.deployment_count }} {{ t('releaseLogs.detailDeployments') }}</span></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailMessage') }}</span><span class="detail-message">{{ detail.message || '-' }}</span></div>
        <div class="detail-row"><span class="detail-label">{{ t('releaseLogs.detailTime') }}</span><span><time :datetime="detail.created_at">{{ formatTime(detail.created_at) }}</time> · {{ detail.latency_ms }}ms</span></div>
        <div class="detail-actions"><button class="btn btn-secondary btn-sm" @click="detail && copyId(detail.subscription_id)">{{ t('releaseLogs.copySubId') }}</button> <button class="btn btn-primary btn-sm" @click="showDetail=false">{{ t('releaseLogs.btnClose') }}</button></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.filter-panel{
  margin:12px 0 16px;
  border:1px solid var(--border-glass);
  border-radius:12px;
  padding:14px;
  background:var(--bg-card);
  backdrop-filter:blur(8px);
}
.filter-grid{
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(180px,1fr));
  gap:10px 12px;
}
.filter-field{ display:flex; flex-direction:column; gap:6px; }
.filter-field span{ font-size:0.72rem; color:var(--text-muted); font-weight:600; }
.filter-actions{ display:flex; gap:8px; align-items:center; margin-top:12px; flex-wrap:wrap; }
.pagination-bar{
  display:flex; justify-content:space-between; align-items:center;
  margin-top:12px; flex-wrap:wrap; gap:10px;
  font-size:0.85rem;
}
.pagination-actions{ display:flex; gap:8px; align-items:center; }
.sr-only{ position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }

.logs-tab {
  display: block;
}
.api-path {
  font-weight: 400;
  color: var(--text-muted);
  font-size: 0.82rem;
  margin-left: 8px;
}
.total-text {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.metrics-grid-flush {
  margin-top: 0;
}
.value-success {
  color: var(--accent-emerald);
}
.value-warning {
  color: var(--accent-amber);
}
.filter-applied {
  font-size: 0.75rem;
  color: var(--accent-amber);
}
.col-id { min-width: 78px; }
.col-sub { min-width: 160px; }
.col-endpoint { width: 84px; }
.col-status { width: 92px; }
.col-narrow { width: 72px; }
.col-center { text-align: center; }
.col-latency { width: 88px; }
.col-time { min-width: 150px; }
.col-action { width: 88px; }
.state-cell {
  text-align: center;
  padding: 28px;
  color: var(--text-muted);
}
.state-cell-lg {
  text-align: center;
  padding: 36px;
  color: var(--text-muted);
}
.spinner-inline {
  display: inline-block;
  width: 18px;
  height: 18px;
  border: 2px solid var(--bg-soft);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  vertical-align: middle;
  margin-right: 8px;
}
.mono-id { font-size: 0.75rem; }
.mono-sub {
  font-size: 0.72rem;
  word-break: break-all;
}
.copy-sub-btn {
  margin-left: 6px;
  padding: 2px 6px;
}
.mono-endpoint {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}
.error-code {
  font-size: 0.68rem;
  color: var(--text-muted);
  margin-top: 2px;
}
.text-success { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.muted-sm {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.muted-xs {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.limit-select {
  width: 72px;
  display: inline-block;
  padding: 4px 6px;
}
.page-text {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.modal-detail { max-width: 640px; }
.detail-grid {
  padding: 4px 0 8px;
  display: grid;
  gap: 10px;
  font-size: 0.85rem;
}
.detail-row {
  display: grid;
  grid-template-columns: 110px 1fr;
  gap: 6px;
}
.detail-label { color: var(--text-muted); }
.detail-code { word-break: break-all; }
.detail-message {
  word-break: break-all;
  color: var(--text-muted);
}
.detail-actions {
  text-align: right;
  margin-top: 8px;
}
</style>
