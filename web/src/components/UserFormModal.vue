<script setup lang="ts">
// 用户新增/编辑表单弹窗
// 包含：用户基础信息 / 快捷凭证提取 / 节点端点 (Endpoints) 灵活配置 / config_override
// 契约：props { open, editing, nodes }，emits { close, saved }
import { inject, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { Node, User } from '../types'
import { parseCredentials, randomPassword, randomUUID } from '../utils'

const { t } = useI18n()

const props = defineProps<{
  open: boolean
  editing: User | null
  nodes: Node[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void

// 用户页可能在节点管理页之后才打开，不能只依赖父组件首次加载的节点快照。
const availableNodes = ref<Node[]>([...props.nodes])
const activeNodeSelector = ref<string | null>(null)
const nodeSearch = reactive<Record<string, string>>({})

const showExtract = ref(false)
const extractInput = ref('')
const formInitialized = ref(false)
const draftKey = () => `noder.user-form.${props.editing?.id ?? 'new'}`

interface EndpointFormItem {
  id?: number
  node_id: number
  tag: string
  tagManuallyEdited?: boolean
  uuid: string
  password?: string
  is_active: boolean
}

const form = reactive({
  name: '',
  remark: '',
  token: '',
  uuid: '',
  password: '',
  config_override: '',
  endpoints: [] as EndpointFormItem[],
  node_ids: [] as number[],
  independent_node_ids: [] as number[],
})

function resetForm() {
  Object.assign(form, {
    name: '',
    remark: '',
    token: '',
    uuid: '',
    password: '',
    config_override: '',
    endpoints: [],
    node_ids: [],
    independent_node_ids: [],
  })
  showExtract.value = false
  extractInput.value = ''
}

function endpointKey(ep: EndpointFormItem, index: number) {
  return String(ep.id ?? `new-${index}`)
}

function nodeDefaultTag(node: Node) {
  return node.node_name || node.tag || `Node-${node.id}`
}

function nodeLabel(node: Node) {
  return `${nodeDefaultTag(node)} (${node.protocol.toUpperCase()})`
}

function filteredNodes(ep: EndpointFormItem, index: number) {
  const query = (nodeSearch[endpointKey(ep, index)] || '').trim().toLowerCase()
  if (!query) return availableNodes.value
  return availableNodes.value.filter(node => [
    node.node_name,
    node.tag,
    node.protocol,
    node.server_address,
    node.server_port,
  ].some(value => String(value ?? '').toLowerCase().includes(query)))
}

function selectedNodeLabel(ep: EndpointFormItem) {
  const node = availableNodes.value.find(n => n.id === ep.node_id)
  return node ? nodeLabel(node) : `Node-${ep.node_id}`
}

function focusNodeSelector(ep: EndpointFormItem, index: number) {
  const key = endpointKey(ep, index)
  activeNodeSelector.value = key
  nodeSearch[key] = ''
}

function chooseNode(ep: EndpointFormItem, node: Node, index: number) {
  ep.node_id = node.id
  onNodeChange(ep)
  nodeSearch[endpointKey(ep, index)] = ''
  activeNodeSelector.value = null
}

function chooseFirstMatchingNode(ep: EndpointFormItem, index: number) {
  const firstNode = filteredNodes(ep, index)[0]
  if (firstNode) chooseNode(ep, firstNode, index)
}

async function refreshNodes() {
  try {
    availableNodes.value = await api.nodes.list()
  } catch {
    // 父组件已有节点列表时仍可正常编辑，不因刷新失败阻断表单。
    availableNodes.value = [...props.nodes]
  }
}

function closeModal(clearDraft = false) {
  if (clearDraft) localStorage.removeItem(draftKey())
  emit('close')
}

function loadForm(u: User) {
  let eps: EndpointFormItem[] = []
  if (u.endpoints && u.endpoints.length) {
    eps = u.endpoints.map(e => ({
      id: e.id,
      node_id: e.node_id,
      tag: e.tag,
      tagManuallyEdited: (() => {
        const node = availableNodes.value.find(n => n.id === e.node_id)
        return Boolean(e.tag && (!node || e.tag !== nodeDefaultTag(node)))
      })(),
      uuid: e.uuid,
      password: e.password || '',
      is_active: e.is_active ?? true,
    }))
  } else if (u.node_ids && u.node_ids.length) {
    eps = u.node_ids.map(nid => {
      const node = props.nodes.find(n => n.id === nid)
      return {
        node_id: nid,
        tag: node?.node_name || node?.tag || `Node-${nid}`,
        tagManuallyEdited: false,
        uuid: u.uuid || '',
        password: u.password || '',
        is_active: true,
      }
    })
  }
  Object.assign(form, {
    name: u.name,
    remark: u.remark || '',
    token: u.token,
    uuid: u.uuid || '',
    password: u.password || '',
    config_override: u.config_override || '',
    endpoints: eps,
    node_ids: [...u.node_ids],
    independent_node_ids: [...(u.independent_node_ids || [])],
  })
  showExtract.value = false
  extractInput.value = ''
}

// 打开弹窗时初始化表单（编辑回填 / 新增重置）
watch(
  () => props.open,
  async (open) => {
    if (!open) return
    await refreshNodes()
    const draft = localStorage.getItem(draftKey())
    if (draft) {
      try {
        Object.assign(form, JSON.parse(draft))
        showExtract.value = false
        extractInput.value = ''
      } catch {
        localStorage.removeItem(draftKey())
        if (props.editing) loadForm(props.editing)
        else resetForm()
      }
    } else if (props.editing) loadForm(props.editing)
    else resetForm()
    formInitialized.value = true
  },
)

watch(() => props.nodes, (nodes) => {
  availableNodes.value = [...nodes]
}, { deep: true })

watch(form, (value) => {
  if (props.open && formInitialized.value) localStorage.setItem(draftKey(), JSON.stringify(value))
}, { deep: true })

function addEndpoint(nodeId?: number) {
  let targetNode = availableNodes.value[0]
  if (nodeId) {
    const found = availableNodes.value.find(n => n.id === nodeId)
    if (found) targetNode = found
  }
  if (!targetNode) return

  form.endpoints.push({
    node_id: targetNode.id,
    tag: nodeDefaultTag(targetNode),
    tagManuallyEdited: false,
    // Endpoint UUIDs follow the user's UUID by default. The backend also
    // applies this default when the user UUID is generated on save.
    uuid: form.uuid || '',
    password: form.password || '',
    is_active: true,
  })
}

function addAllNodes() {
  for (const n of availableNodes.value) {
    form.endpoints.push({
      node_id: n.id,
      tag: nodeDefaultTag(n),
      tagManuallyEdited: false,
      uuid: form.uuid || '',
      password: form.password || '',
      is_active: true,
    })
  }
}

function onNodeChange(ep: EndpointFormItem) {
  const node = availableNodes.value.find(n => n.id === ep.node_id)
  if (node && !ep.tagManuallyEdited) {
    ep.tag = nodeDefaultTag(node)
  }
}

function onEndpointTagInput(ep: EndpointFormItem) {
  ep.tagManuallyEdited = true
}

function toggleIndependentNode(id: number) {
  const idx = form.independent_node_ids.indexOf(id)
  if (idx >= 0) form.independent_node_ids.splice(idx, 1)
  else form.independent_node_ids.push(id)
}

function moveEndpointToTop(index: number) {
  if (index <= 0 || index >= form.endpoints.length) return
  const [target] = form.endpoints.splice(index, 1)
  form.endpoints.unshift(target)
}

function moveEndpointUp(index: number) {
  if (index <= 0) return
  const temp = form.endpoints[index]
  form.endpoints[index] = form.endpoints[index - 1]
  form.endpoints[index - 1] = temp
}

function moveEndpointDown(index: number) {
  if (index >= form.endpoints.length - 1) return
  const temp = form.endpoints[index]
  form.endpoints[index] = form.endpoints[index + 1]
  form.endpoints[index + 1] = temp
}

function removeEndpoint(index: number) {
  form.endpoints.splice(index, 1)
}

function applyExtract() {
  const { uuid, password } = parseCredentials(extractInput.value)
  if (uuid) form.uuid = uuid
  if (password) form.password = password
  toast(uuid || password ? t('users.form.extractSuccess') : t('users.form.extractFail'), uuid || password ? 'info' : 'error')
}

async function saveUser() {
  const payload = {
    name: form.name.trim(),
    remark: form.remark.trim() || null,
    config_override: form.config_override.trim() || null,
    token: form.token || undefined,
    uuid: form.uuid || undefined,
    password: form.password || undefined,
    endpoints: form.endpoints.map((ep, idx) => ({
      id: ep.id,
      node_id: ep.node_id,
      tag: ep.tag.trim(),
      uuid: ep.uuid.trim(), // 留空则后端使用用户 UUID
      password: ep.password?.trim() || undefined,
      is_active: ep.is_active,
      sort_order: idx,
    })),
    node_ids: Array.from(new Set(form.endpoints.map(e => e.node_id))),
    independent_node_ids: form.independent_node_ids,
    is_active: true,
  }
  try {
    if (props.editing) {
      await api.users.update(props.editing.id, payload)
      toast(t('users.updated'))
    } else {
      await api.users.create(payload)
      toast(t('users.created'))
    }
    localStorage.removeItem(draftKey())
    emit('close')
    emit('saved')
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}
</script>

<template>
  <div class="modal-overlay" :class="{ active: open }" @mousedown.self="closeModal()">
    <div class="modal modal-lg">
      <div class="modal-header">
        <div class="modal-title">{{ editing ? t('users.form.editTitle') : t('users.form.addTitle') }}</div>
        <button class="modal-close" @click="closeModal(true)">&times;</button>
      </div>
      <form @submit.prevent="saveUser" class="modal-form">
        <div class="form-span" style="margin-bottom: 14px; background: rgba(59,130,246,0.1); padding: 10px; border-radius: 8px; border: 1px dashed rgba(59,130,246,0.4)">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <span style="font-size: 0.8rem; color: var(--primary); font-weight: 600">{{ t('users.form.extractTitle') }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="showExtract = !showExtract">{{ t('users.form.toggleExtract') }}</button>
          </div>
          <div v-if="showExtract" style="margin-top: 10px">
            <textarea v-model="extractInput" class="form-control" rows="3" style="font-family: var(--font-mono); font-size: 0.78rem"
              :placeholder="t('users.form.extractPlaceholder')"></textarea>
            <button type="button" class="btn btn-primary btn-sm" style="margin-top: 8px; width: 100%" @click="applyExtract">{{ t('users.form.extractBtn') }}</button>
          </div>
        </div>

        <div class="form-group">
          <label>{{ t('users.form.name') }}</label>
          <input v-model="form.name" class="form-control" :placeholder="t('users.form.namePlaceholder')" required />
        </div>
        <div class="form-group">
          <label>{{ t('users.form.remark') }}</label>
          <input v-model="form.remark" class="form-control" :placeholder="t('users.form.remarkPlaceholder')" />
        </div>
        <div class="form-group">
          <label>{{ t('users.form.token') }}</label>
          <div class="credential-row">
            <input v-model="form.token" class="form-control" :placeholder="t('users.form.tokenPlaceholder')" />
            <button type="button" class="btn btn-secondary" @click="form.token = randomUUID()">{{ t('users.form.randomGenerate') }}</button>
          </div>
        </div>
        <div class="form-group">
          <label>{{ t('users.form.uuid') }}</label>
          <div class="credential-row">
            <input v-model="form.uuid" class="form-control" :placeholder="t('users.form.uuidPlaceholder')" />
            <button type="button" class="btn btn-secondary" @click="form.uuid = randomUUID()">{{ t('users.form.randomUuid') }}</button>
          </div>
        </div>
        <div class="form-group">
          <label>{{ t('users.form.password') }}</label>
          <div class="credential-row">
            <input v-model="form.password" class="form-control" :placeholder="t('users.form.passwordPlaceholder')" />
            <button type="button" class="btn btn-secondary" @click="form.password = randomPassword()">{{ t('users.form.randomPassword') }}</button>
          </div>
        </div>
        <div class="form-group">
          <label>{{ t('users.form.configOverride') }}</label>
          <textarea v-model="form.config_override" class="form-control" rows="3" style="font-family: var(--font-mono); font-size: 0.78rem"
            :placeholder="t('users.form.configOverridePlaceholder')"></textarea>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">{{ t('users.form.configOverrideTip') }}</div>
        </div>
        <div class="form-group form-span">
          <label>{{ t('users.form.independentNodes') }}</label>
          <div class="checkboxes-group">
            <label v-for="node in nodes" :key="`independent-${node.id}`" class="checkbox-label">
              <input type="checkbox" :checked="form.independent_node_ids.includes(node.id)" @change="toggleIndependentNode(node.id)" />
              {{ node.node_name }} <span style="color: var(--text-muted); font-size: 0.72rem">({{ node.protocol }})</span>
            </label>
          </div>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">{{ t('users.form.independentNodesTip') }}</div>
        </div>

        <!-- 节点端点 (Endpoints) 区域 -->
        <div class="form-group form-span endpoints-section">
          <div class="endpoints-header">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="endpoints-title">{{ t('users.form.endpoints') }}</span>
              <span class="endpoints-count">{{ form.endpoints.length }}</span>
            </div>
            <div style="display: flex; gap: 8px; flex-wrap: wrap;">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="!availableNodes.length" @click="addAllNodes">
                {{ t('users.form.addAllNodes') }}
              </button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="!availableNodes.length" @click="addEndpoint()">
                {{ t('users.form.addEndpoint') }}
              </button>
            </div>
          </div>
          <div class="endpoint-tip">{{ t('users.form.endpointTip') }}</div>

          <div v-if="!form.endpoints.length" class="empty-endpoints">
            {{ t('users.form.noEndpoints') }}
          </div>

          <div v-else class="endpoints-list">
            <div
              v-for="(ep, idx) in form.endpoints"
              :key="idx"
              class="endpoint-card"
              :class="{
                'is-primary': idx === 0,
                'is-node-select-open': activeNodeSelector === endpointKey(ep, idx),
              }"
            >
              <div class="endpoint-card-row1">
                <div style="display: flex; align-items: center; gap: 6px;">
                  <span class="endpoint-rank" :class="{ 'rank-first': idx === 0 }">#{{ idx + 1 }}</span>
                  <span v-if="idx === 0" class="primary-badge">{{ t('users.form.primaryNodeBadge') }}</span>
                </div>

                <!-- 选择物理入口 -->
                <div class="endpoint-node-select" @keydown.esc="activeNodeSelector = null">
                  <input
                    class="form-control form-control-sm"
                    :value="activeNodeSelector === endpointKey(ep, idx) ? (nodeSearch[endpointKey(ep, idx)] || '') : selectedNodeLabel(ep)"
                    :placeholder="t('users.form.endpointNodePlaceholder')"
                    @focus="focusNodeSelector(ep, idx)"
                    @input="nodeSearch[endpointKey(ep, idx)] = ($event.target as HTMLInputElement).value"
                    @keydown.enter.prevent="chooseFirstMatchingNode(ep, idx)"
                  />
                  <div
                    v-if="activeNodeSelector === endpointKey(ep, idx)"
                    class="endpoint-node-options"
                  >
                    <button
                      v-for="node in filteredNodes(ep, idx)"
                      :key="node.id"
                      type="button"
                      class="endpoint-node-option"
                      @mousedown.prevent="chooseNode(ep, node, idx)"
                    >
                      {{ nodeLabel(node) }}
                    </button>
                    <span v-if="!filteredNodes(ep, idx).length" class="endpoint-node-empty">
                      {{ t('common.noResults') }}
                    </span>
                  </div>
                </div>

                <!-- 节点 Tag -->
                <div class="endpoint-tag-input">
                  <input
                    v-model="ep.tag"
                    class="form-control form-control-sm"
                    :placeholder="t('users.form.endpointTagPlaceholder')"
                    required
                    @input="onEndpointTagInput(ep)"
                  />
                </div>

                <!-- 操作按钮 -->
                <div class="endpoint-actions">
                  <button
                    v-if="idx > 0"
                    type="button"
                    class="btn-sort-action btn-pin"
                    :title="t('users.form.pinToTop')"
                    @click="moveEndpointToTop(idx)"
                  >
                    📌
                  </button>
                  <button
                    type="button"
                    class="btn-sort-action"
                    :disabled="idx === 0"
                    :title="t('users.form.moveUp')"
                    @click="moveEndpointUp(idx)"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    class="btn-sort-action"
                    :disabled="idx === form.endpoints.length - 1"
                    :title="t('users.form.moveDown')"
                    @click="moveEndpointDown(idx)"
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    class="btn-sort-action btn-remove"
                    :title="t('users.form.removeNode')"
                    @click="removeEndpoint(idx)"
                  >
                    ✕
                  </button>
                </div>
              </div>

              <div class="endpoint-card-row2">
                <div class="endpoint-uuid-wrap">
                  <span class="field-label-sm">UUID</span>
                  <input
                    v-model="ep.uuid"
                    class="form-control form-control-sm code-font"
                    :placeholder="t('users.form.endpointUuidPlaceholder')"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm btn-dice"
                    @click="ep.uuid = randomUUID()"
                    :title="t('users.form.randomUuid')"
                  >
                    🎲
                  </button>
                </div>

                <div class="endpoint-pwd-wrap">
                  <span class="field-label-sm">PWD</span>
                  <input
                    v-model="ep.password"
                    class="form-control form-control-sm code-font"
                    :placeholder="t('users.form.endpointPassword')"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="closeModal()">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary">{{ t('users.form.saveUser') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.modal.modal-lg {
  max-width: 780px;
}

.credential-row {
  display: flex;
  gap: 8px;
}

.credential-row .form-control {
  flex: 1;
  min-width: 0;
}

/* 端点列表面板 */
.endpoints-section {
  background: rgba(15, 23, 42, 0.4);
  border: 1px solid var(--border-glass);
  border-radius: 10px;
  padding: 14px;
  margin-top: 4px;
}

.endpoints-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  flex-wrap: wrap;
  gap: 8px;
}

.endpoints-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--primary);
}

.endpoints-count {
  font-size: 0.72rem;
  background: rgba(59, 130, 246, 0.2);
  color: var(--primary);
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 600;
}

.endpoint-tip {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-bottom: 12px;
  line-height: 1.4;
}

.empty-endpoints {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
  font-size: 0.82rem;
  border: 1px dashed rgba(255, 255, 255, 0.1);
  border-radius: 8px;
}

.endpoints-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 340px;
  overflow-y: auto;
  padding-right: 4px;
}

.endpoint-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.2s ease;
}

.endpoint-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.15);
}

.endpoint-card.is-primary {
  border-color: rgba(59, 130, 246, 0.45);
  background: rgba(59, 130, 246, 0.07);
}

/* 打开节点下拉框时，将当前端点提升到其它端点之上。 */
.endpoint-card.is-node-select-open {
  position: relative;
  z-index: 100;
}

.endpoint-card-row1 {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.endpoint-rank {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  width: 22px;
}

.endpoint-rank.rank-first {
  color: var(--primary);
  font-weight: 700;
}

.primary-badge {
  font-size: 0.68rem;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  white-space: nowrap;
}

.endpoint-node-select {
  flex: 1.2;
  min-width: 170px;
  position: relative;
}

.endpoint-node-options {
  position: absolute;
  z-index: 1000;
  top: calc(100% + 3px);
  left: 0;
  right: 0;
  max-height: 220px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  background: var(--bg-card, #172033);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.35);
}

.endpoint-node-option {
  display: block;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
  text-align: left;
  font-size: 0.78rem;
  cursor: pointer;
}

.endpoint-node-option:hover {
  background: rgba(59, 130, 246, 0.2);
}

.endpoint-node-empty {
  display: block;
  padding: 8px;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.endpoint-tag-input {
  flex: 1.3;
  min-width: 170px;
}

.endpoint-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}

.endpoint-card-row2 {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 4px;
  border-top: 1px dashed rgba(255, 255, 255, 0.05);
  flex-wrap: wrap;
}

.endpoint-uuid-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1.8;
  min-width: 240px;
}

.endpoint-pwd-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 130px;
}

.field-label-sm {
  font-size: 0.68rem;
  font-weight: 700;
  color: var(--text-muted);
  width: 28px;
  flex-shrink: 0;
}

.code-font {
  font-family: var(--font-mono);
  font-size: 0.75rem !important;
}

.form-control-sm {
  padding: 4px 8px;
  font-size: 0.8rem;
  height: 28px;
}

.btn-dice {
  padding: 2px 7px;
  font-size: 0.8rem;
  height: 28px;
  line-height: 1;
  flex-shrink: 0;
}

.btn-sort-action {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-base);
  border-radius: 4px;
  padding: 2px 7px;
  font-size: 0.75rem;
  height: 26px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-sort-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
}

.btn-sort-action:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.btn-pin {
  background: rgba(59, 130, 246, 0.2);
  border-color: rgba(59, 130, 246, 0.4);
  color: #93c5fd;
  font-weight: 600;
}

.btn-pin:hover {
  background: rgba(59, 130, 246, 0.35) !important;
  color: #fff !important;
}

.btn-remove {
  color: var(--accent-rose);
}

.btn-remove:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  border-color: rgba(239, 68, 68, 0.4) !important;
}
</style>
