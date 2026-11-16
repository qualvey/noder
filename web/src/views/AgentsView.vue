<script setup lang="ts">
import { inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { Agent, AgentBinding, Node } from '../types'
import { useFuzzySearch } from '../composables'
import { copyText } from '../utils'
import { ToastKey, PopoverKey } from '../injectionKeys'

const { t } = useI18n()
const toast = inject(ToastKey)
const popover = inject(PopoverKey)

const agents = ref<Agent[]>([])
const nodes = ref<Node[]>([])
const loading = ref(true)
const creating = ref(false)
const editingAgent = ref<Agent | null>(null)
const showModal = ref(false)
const form = ref({ id: '', name: '' })
const tokenModal = ref<{ token: string; id: string } | null>(null)

// bindings
const bindingAgent = ref<Agent | null>(null)
const bindings = ref<AgentBinding[]>([])
const bindingLoading = ref(false)
const showBindingModal = ref(false)
const selectedNodes = ref<Set<number>>(new Set())

const { searchQuery, filteredItems: filteredAgents, clear: clearSearch } = useFuzzySearch(agents, (a) => [
  a.id, a.name, a.status, a.is_online ? 'online' : 'offline', String(a.binding_count),
])

function formatTime(v?: string | null) {
  if (!v) return '-'
  try { return new Date(v).toLocaleString() } catch { return v }
}

async function fetchAgents(silent = false) {
  if (!silent && !agents.value.length) loading.value = true
  try {
    const data = await api.agents.list()
    agents.value = (data as unknown as { items: Agent[] }).items ?? (data as unknown as Agent[]) ?? []
    if (!Array.isArray(agents.value)) agents.value = []
  } catch (e) { toast?.((e as Error).message, 'error') } finally { loading.value = false }
}
async function fetchNodes() {
  try {
    nodes.value = await api.nodes.list()
  } catch {
    // 节点列表用于绑定弹窗展示，失败时允许为空，不阻断 Agent 列表
  }
}

function openCreate() {
  editingAgent.value = null
  form.value = { id: '', name: '' }
  showModal.value = true
}
function openEdit(a: Agent) {
  editingAgent.value = a
  form.value = { id: a.id, name: a.name }
  showModal.value = true
}

interface AgentCreateResponse {
  id?: string
  control_token?: string
  data?: {
    id?: string
    control_token?: string
  }
}

async function submitModal() {
  const name = form.value.name.trim()
  if (!name) { toast?.(t('agents.nameRequired'), 'error'); return }
  creating.value = true
  try {
    if (editingAgent.value) {
      await api.agents.update(editingAgent.value.id, { name })
      toast?.(t('agents.updated'), 'info')
    } else {
      const payload: { id?: string; name: string } = { name }
      if (form.value.id.trim()) payload.id = form.value.id.trim()
      const res = (await api.agents.create(payload)) as AgentCreateResponse
      const token = res.control_token ?? res.data?.control_token
      if (token) tokenModal.value = { token, id: res.id ?? res.data?.id ?? payload.id ?? '' }
      toast?.(t('agents.created'), 'info')
    }
    showModal.value = false
    await fetchAgents(true)
  } catch (e) { toast?.((e as Error).message, 'error') } finally { creating.value = false }
}

async function toggleStatus(a: Agent) {
  const next = a.status === 'disabled' ? 'offline' : 'disabled'
  try {
    await api.agents.update(a.id, { status: next })
    toast?.(next === 'disabled' ? t('agents.disabled') : t('agents.enabled'), 'info')
    await fetchAgents(true)
  } catch (e) { toast?.((e as Error).message, 'error') }
}

async function rotateToken(a: Agent, el: Element) {
  popover?.show(el, t('agents.rotateConfirm'), async () => {
    try {
      const res = (await api.agents.rotateToken(a.id)) as AgentCreateResponse
      const token = res.control_token ?? res.data?.control_token
      tokenModal.value = { token: token ?? '', id: a.id }
      toast?.(t('agents.tokenRotated'), 'info')
      await fetchAgents(true)
    } catch (e) { toast?.((e as Error).message, 'error') }
  })
}

async function removeAgent(a: Agent, el: Element) {
  const hasBinding = a.binding_count > 0
  const title = hasBinding
    ? t('agents.deleteWithBindings', { count: a.binding_count })
    : t('agents.deleteConfirm')
  popover?.show(el, title, async () => {
    try {
      await api.agents.remove(a.id, hasBinding)
      toast?.(t('agents.deleted'), 'info')
      await fetchAgents(true)
    } catch (e) { toast?.((e as Error).message, 'error') }
  })
}

async function openBindings(a: Agent) {
  bindingAgent.value = a
  showBindingModal.value = true
  bindingLoading.value = true
  try {
    bindings.value = await api.agents.listBindings(a.id)
    selectedNodes.value = new Set(bindings.value.map(b => b.endpoint_id))
  } catch (e) { toast?.((e as Error).message, 'error'); bindings.value = [] } finally { bindingLoading.value = false }
}

function toggleNodeSelect(id: number) {
  const s = new Set(selectedNodes.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selectedNodes.value = s
}

async function saveBindings() {
  if (!bindingAgent.value) return
  const current = new Set(bindings.value.map(b => b.endpoint_id))
  const desired = selectedNodes.value
  const toAdd: number[] = []
  const toRemove: number[] = []
  for (const id of desired) if (!current.has(id)) toAdd.push(id)
  for (const id of current) if (!desired.has(id)) toRemove.push(id)
  try {
    if (toAdd.length) await api.agents.addBindings(bindingAgent.value.id, { endpoint_ids: toAdd })
    for (const eid of toRemove) await api.agents.removeBinding(bindingAgent.value.id, eid)
    toast?.(t('agents.bindingsUpdated'), 'info')
    showBindingModal.value = false
    await fetchAgents(true)
  } catch (e) { toast?.((e as Error).message, 'error') }
}

async function copyToken(token: string) {
  const ok = await copyText(token)
  toast?.(ok ? t('agents.copied') : t('agents.copyFailed'), ok ? 'info' : 'error')
}

onMounted(async () => { await Promise.all([fetchAgents(), fetchNodes()]) })
</script>

<template>
  <section class="tab-content agents-tab">
    <div class="section-header">
      <div class="section-title">{{ t('agents.title') }}</div>
      <div class="section-actions">
        <div class="search-row">
          <input v-model="searchQuery" :placeholder="t('agents.searchPlaceholder')" class="form-control search-input" />
          <button v-if="searchQuery" class="btn btn-secondary btn-sm" @click="clearSearch">{{ t('agents.clear') }}</button>
        </div>
        <button class="btn btn-primary" @click="openCreate">{{ t('agents.add') }}</button>
      </div>
    </div>

    <div v-if="loading && !agents.length" class="loading-container">
      <svg class="spinner" viewBox="0 0 24 24" fill="none"><circle class="spinner-track" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="spinner-head" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"/></svg>
      <span class="loading-text">{{ t('agents.loading') }}</span>
    </div>
    <div v-else-if="!agents.length" class="empty-state">{{ t('agents.empty') }}</div>
    <div v-else-if="!filteredAgents.length" class="empty-state">
      <div class="empty-icon">🔍</div><div class="empty-text">{{ t('agents.noMatch') }}</div>
      <button class="btn btn-secondary btn-sm" @click="clearSearch">{{ t('agents.clearSearch') }}</button>
    </div>

    <div v-else class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th class="col-id">{{ t('agents.colId') }}</th>
            <th>{{ t('agents.colName') }}</th>
            <th class="col-narrow col-center">{{ t('agents.colStatus') }}</th>
            <th class="col-narrow col-center">{{ t('agents.colOnline') }}</th>
            <th class="col-heartbeat">{{ t('agents.colHeartbeat') }}</th>
            <th class="col-narrow col-center">{{ t('agents.colBindings') }}</th>
            <th class="col-actions col-center">{{ t('agents.colActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in filteredAgents" :key="a.id">
            <td><code class="mono-sm">{{ a.id }}</code></td>
            <td class="cell-name">{{ a.name }}</td>
            <td class="col-center">
              <span class="status-badge" :class="a.status==='online'?'active':a.status==='disabled'?'inactive':''">
                <span class="status-dot"></span>{{ a.status }}
              </span>
            </td>
            <td class="col-center">
              <span :class="a.is_online ? 'online-text' : 'muted-text'">{{ a.is_online ? t('agents.online') : t('agents.offline') }}</span>
            </td>
            <td class="muted-text text-sm">{{ formatTime(a.last_heartbeat_at) }}</td>
            <td class="col-center"><span class="badge-count">{{ a.binding_count }}</span></td>
            <td class="col-center">
              <div class="row-actions">
                <button class="btn btn-secondary btn-sm" @click="openBindings(a)">{{ t('agents.bind') }}</button>
                <button class="btn btn-secondary btn-sm" @click="openEdit(a)">{{ t('agents.edit') }}</button>
                <button class="btn btn-secondary btn-sm" @click="rotateToken(a, $event.currentTarget as Element)">{{ t('agents.rotateToken') }}</button>
                <button class="btn btn-secondary btn-sm" @click="toggleStatus(a)">{{ a.status==='disabled'?t('agents.enable'):t('agents.disable') }}</button>
                <button class="btn btn-danger btn-sm" @click="removeAgent(a, $event.currentTarget as Element)">{{ t('agents.delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <!-- 创建/编辑 -->
  <div class="modal-overlay" :class="{active: showModal}" @mousedown.self="showModal=false">
    <div class="modal modal-sm">
      <div class="modal-header">
        <div class="modal-title">{{ editingAgent ? t('agents.editTitle') : t('agents.createTitle') }}</div>
        <button class="modal-close" @click="showModal=false">&times;</button>
      </div>
      <form @submit.prevent="submitModal" class="modal-form modal-form-padded">
        <div v-if="!editingAgent" class="form-group">
          <label>{{ t('agents.idOptionalLabel') }}</label>
          <input v-model="form.id" class="form-control" :placeholder="t('agents.idPlaceholder')" />
          <div class="field-hint">{{ t('agents.idHint') }}</div>
        </div>
        <div v-else class="form-group">
          <label>{{ t('agents.idLabel') }}</label>
          <input :value="form.id" class="form-control" disabled />
        </div>
        <div class="form-group">
          <label>{{ t('agents.nameLabel') }}</label>
          <input v-model="form.name" class="form-control" :placeholder="t('agents.namePlaceholder')" required maxlength="64" />
        </div>
        <div class="modal-form-actions">
          <button type="button" class="btn btn-secondary" @click="showModal=false">{{ t('agents.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="creating">{{ creating ? t('agents.submitting') : t('agents.save') }}</button>
        </div>
      </form>
    </div>
  </div>

  <!-- Token 一次性展示 -->
  <div class="modal-overlay" :class="{active: !!tokenModal}" @mousedown.self="tokenModal=null">
    <div class="modal modal-md">
      <div class="modal-header">
        <div class="modal-title">{{ t('agents.tokenOnceTitle') }}</div>
        <button class="modal-close" @click="tokenModal=null">&times;</button>
      </div>
      <div class="modal-body-padded">
        <div class="token-desc">{{ t('agents.tokenFor', { id: tokenModal?.id }) }}</div>
        <code class="token-code">{{ tokenModal?.token }}</code>
        <div class="token-hint">{{ t('agents.tokenHint') }}</div>
        <div class="modal-form-actions">
          <button class="btn btn-secondary" @click="tokenModal=null">{{ t('agents.close') }}</button>
          <button class="btn btn-primary" @click="tokenModal && copyToken(tokenModal.token)">{{ t('agents.copyToken') }}</button>
        </div>
      </div>
    </div>
  </div>

  <!-- 绑定管理 -->
  <div class="modal-overlay" :class="{active: showBindingModal}" @mousedown.self="showBindingModal=false">
    <div class="modal modal-lg-scroll">
      <div class="modal-header">
        <div class="modal-title">{{ t('agents.bindTitle', { id: bindingAgent?.id, name: bindingAgent?.name }) }}</div>
        <button class="modal-close" @click="showBindingModal=false">&times;</button>
      </div>
      <div class="modal-body-padded">
        <div v-if="bindingLoading" class="loading-inline">{{ t('agents.bindLoading') }}</div>
        <div v-else>
          <div v-if="!nodes.length" class="muted-text text-sm">{{ t('agents.bindEmpty') }}</div>
          <div v-else class="binding-list">
            <label v-for="n in nodes" :key="n.id" class="binding-item" :class="{ selected: selectedNodes.has(n.id) }">
              <input type="checkbox" :checked="selectedNodes.has(n.id)" @change="toggleNodeSelect(n.id)" />
              <div class="binding-main">
                <div class="binding-title">{{ n.tag || n.node_name }} <span class="proto-tag">{{ n.protocol }}</span></div>
                <div class="muted-text text-sm">{{ n.server_address }}:{{ n.server_port }} <span v-if="n.is_agentless" class="agentless-text">{{ t('agents.agentless') }}</span></div>
              </div>
              <span v-if="selectedNodes.has(n.id)" class="bound-text">{{ t('agents.bound') }}</span>
            </label>
          </div>
          <div class="modal-form-actions">
            <button class="btn btn-secondary" @click="showBindingModal=false">{{ t('agents.cancel') }}</button>
            <button class="btn btn-primary" @click="saveBindings">{{ t('agents.saveBindings') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.agents-tab { display: block; }
.search-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.search-input { width: 220px; }
.empty-state { text-align: center; padding: 40px; color: var(--text-muted); }
.empty-icon { font-size: 1.6rem; margin-bottom: 8px; }
.empty-text { margin-bottom: 12px; }
.col-id { min-width: 160px; }
.col-narrow { width: 90px; }
.col-center { text-align: center; }
.col-heartbeat { min-width: 150px; }
.col-actions { width: 240px; }
.mono-sm { font-size: 0.78rem; }
.cell-name { font-weight: 600; }
.online-text { color: var(--accent-emerald); }
.muted-text { color: var(--text-muted); }
.text-sm { font-size: 0.78rem; }
.badge-count { background: rgba(99,102,241,0.15); color: #a5b4fc; border: 1px solid rgba(99,102,241,0.3); padding: 2px 8px; border-radius: 10px; }
.row-actions { display: flex; gap: 6px; justify-content: center; flex-wrap: wrap; }
.modal-sm { max-width: 420px; }
.modal-md { max-width: 520px; }
.modal-lg-scroll { max-width: 640px; max-height: 80vh; overflow: auto; }
.modal-form-padded { padding: 16px 20px; }
.modal-body-padded { padding: 16px 20px; }
.modal-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.field-hint { font-size: 0.72rem; color: var(--text-muted); margin-top: 4px; }
.token-desc { font-size: 0.85rem; margin-bottom: 8px; }
.token-code { display: block; padding: 10px; background: var(--bg-input); border-radius: 8px; word-break: break-all; font-size: 0.85rem; }
.token-hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 8px; }
.loading-inline { text-align: center; padding: 20px; color: var(--text-muted); }
.binding-list { display: flex; flex-direction: column; gap: 8px; max-height: 380px; overflow: auto; border: 1px solid var(--border-glass); border-radius: 8px; padding: 10px; }
.binding-item { display: flex; align-items: center; gap: 10px; padding: 8px; border-radius: 6px; cursor: pointer; }
.binding-item.selected { background: rgba(59,130,246,0.12); }
.binding-main { flex: 1; }
.binding-title { font-weight: 600; font-size: 0.85rem; }
.proto-tag { font-size: 0.7rem; padding: 1px 6px; border-radius: 4px; background: rgba(59,130,246,0.15); color: #93c5fd; }
.agentless-text { color: #f59e0b; }
.bound-text { color: var(--primary); font-size: 0.8rem; }
.section-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px;gap:12px;flex-wrap:wrap}
.section-title{font-weight:700;font-size:1.1rem}
.section-actions{display:flex;gap:10px;align-items:center;flex-wrap:wrap}
.table-container{overflow:auto;border:1px solid var(--border-glass);border-radius:12px}
.data-table{width:100%;border-collapse:collapse;font-size:0.85rem}
.data-table th{padding:10px 12px;text-align:left;color:var(--text-muted);font-weight:600;border-bottom:1px solid var(--border-glass);white-space:nowrap}
.data-table td{padding:10px 12px;border-bottom:1px solid rgba(255,255,255,0.04)}
.loading-container{display:flex;align-items:center;gap:10px;justify-content:center;padding:32px;color:var(--text-muted)}
.spinner{width:20px;height:20px;animation:spin 0.8s linear infinite}
.spinner-track{opacity:0.2}
@keyframes spin{to{transform:rotate(360deg)}}
</style>
