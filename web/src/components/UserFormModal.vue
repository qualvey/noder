<script setup lang="ts">
// 用户新增/编辑表单弹窗
// 自包含：凭证生成 / 快捷提取 / 节点多选 / config_override
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

const showExtract = ref(false)
const extractInput = ref('')
const formInitialized = ref(false)
const draftKey = () => `noder.user-form.${props.editing?.id ?? 'new'}`

const form = reactive({
  name: '',
  remark: '',
  token: '',
  uuid: '',
  password: '',
  config_override: '',
  node_ids: [] as number[],
})

function resetForm() {
  Object.assign(form, { name: '', remark: '', token: '', uuid: '', password: '', config_override: '', node_ids: [] })
  showExtract.value = false
  extractInput.value = ''
}

function closeModal(clearDraft = false) {
  if (clearDraft) localStorage.removeItem(draftKey())
  emit('close')
}

function loadForm(u: User) {
  Object.assign(form, {
    name: u.name, remark: u.remark || '', token: u.token, uuid: u.uuid || '',
    password: u.password || '', config_override: u.config_override || '', node_ids: [...u.node_ids],
  })
  showExtract.value = false
  extractInput.value = ''
}

// 打开弹窗时初始化表单（编辑回填 / 新增重置）
watch(
  () => props.open,
  (open) => {
    if (!open) return
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

watch(form, (value) => {
  if (props.open && formInitialized.value) localStorage.setItem(draftKey(), JSON.stringify(value))
}, { deep: true })

function toggleNode(id: number) {
  const idx = form.node_ids.indexOf(id)
  if (idx >= 0) {
    form.node_ids.splice(idx, 1)
  } else {
    form.node_ids.push(id)
  }
}

function moveNodeToTop(index: number) {
  if (index <= 0 || index >= form.node_ids.length) return
  const [target] = form.node_ids.splice(index, 1)
  form.node_ids.unshift(target)
}

function moveNodeUp(index: number) {
  if (index <= 0) return
  const temp = form.node_ids[index]
  form.node_ids[index] = form.node_ids[index - 1]
  form.node_ids[index - 1] = temp
}

function moveNodeDown(index: number) {
  if (index >= form.node_ids.length - 1) return
  const temp = form.node_ids[index]
  form.node_ids[index] = form.node_ids[index + 1]
  form.node_ids[index + 1] = temp
}

function getNodeDisplayName(id: number) {
  const node = props.nodes.find((n) => n.id === id)
  return node?.node_name || node?.tag || `#${id}`
}

function getNodeProtocol(id: number) {
  const node = props.nodes.find((n) => n.id === id)
  return node?.protocol?.toUpperCase() || ''
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
    node_ids: form.node_ids,
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
  <div class="modal-overlay" :class="{ active: open }" @click.self="closeModal()">
    <div class="modal">
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
        <div class="form-group form-span">
          <label>{{ t('users.form.configOverride') }}</label>
          <textarea v-model="form.config_override" class="form-control" rows="4" style="font-family: var(--font-mono); font-size: 0.78rem"
            :placeholder="t('users.form.configOverridePlaceholder')"></textarea>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">{{ t('users.form.configOverrideTip') }}</div>
        </div>
        <div class="form-group form-span">
          <label>{{ t('users.form.boundNodes') }}</label>
          <div class="checkboxes-group">
            <label v-for="node in nodes" :key="node.id" class="checkbox-label">
              <input type="checkbox" :checked="form.node_ids.includes(node.id)" @change="toggleNode(node.id)" />
              {{ node.node_name }} <span style="color: var(--text-muted); font-size: 0.72rem">({{ node.protocol }})</span>
            </label>
            <div v-if="!nodes.length" style="color: var(--text-muted); font-size: 0.8rem">{{ t('users.form.noNodes') }}</div>
          </div>
        </div>

        <!-- 节点排序与置顶编排面板 (当选定节点时展示) -->
        <div v-if="form.node_ids.length" class="form-group form-span ordered-nodes-container">
          <div class="ordered-nodes-header">
            <span class="ordered-nodes-title">{{ t('users.form.nodeOrdering') }}</span>
            <span class="ordered-nodes-count">{{ form.node_ids.length }}</span>
          </div>
          <div class="ordered-nodes-list">
            <div
              v-for="(nid, idx) in form.node_ids"
              :key="nid"
              class="ordered-node-item"
              :class="{ 'is-primary': idx === 0 }"
            >
              <div class="node-info">
                <span class="node-rank" :class="{ 'rank-first': idx === 0 }">#{{ idx + 1 }}</span>
                <span v-if="idx === 0" class="primary-badge">{{ t('users.form.primaryNodeBadge') }}</span>
                <span class="node-name">{{ getNodeDisplayName(nid) }}</span>
                <span class="node-protocol">{{ getNodeProtocol(nid) }}</span>
              </div>
              <div class="node-actions">
                <button
                  v-if="idx > 0"
                  type="button"
                  class="btn-sort-action btn-pin"
                  :title="t('users.form.pinToTop')"
                  @click="moveNodeToTop(idx)"
                >
                  📌 {{ t('users.form.pinToTop') }}
                </button>
                <button
                  type="button"
                  class="btn-sort-action"
                  :disabled="idx === 0"
                  :title="t('users.form.moveUp')"
                  @click="moveNodeUp(idx)"
                >
                  ↑
                </button>
                <button
                  type="button"
                  class="btn-sort-action"
                  :disabled="idx === form.node_ids.length - 1"
                  :title="t('users.form.moveDown')"
                  @click="moveNodeDown(idx)"
                >
                  ↓
                </button>
                <button
                  type="button"
                  class="btn-sort-action btn-remove"
                  :title="t('users.form.removeNode')"
                  @click="toggleNode(nid)"
                >
                  ✕
                </button>
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
/* 凭证输入 + 随机按钮一行，输入框弹性伸缩 */
.credential-row {
  display: flex;
  gap: 8px;
}

.credential-row .form-control {
  flex: 1;
  min-width: 0;
}

/* 节点多选：宽度充足时多列排布 */
.checkboxes-group {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 6px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.82rem;
  background: rgba(255, 255, 255, 0.02);
  transition: all 0.2s ease;
}

.checkbox-label:hover {
  background: rgba(59, 130, 246, 0.08);
  border-color: rgba(59, 130, 246, 0.3);
}

/* 节点排序面板 */
.ordered-nodes-container {
  background: rgba(15, 23, 42, 0.35);
  border: 1px solid var(--border-glass);
  border-radius: 8px;
  padding: 12px;
  margin-top: 4px;
}

.ordered-nodes-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.ordered-nodes-title {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--primary);
}

.ordered-nodes-count {
  font-size: 0.72rem;
  background: rgba(59, 130, 246, 0.2);
  color: var(--primary);
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 600;
}

.ordered-nodes-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 220px;
  overflow-y: auto;
  padding-right: 4px;
}

.ordered-node-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  transition: all 0.2s ease;
}

.ordered-node-item:hover {
  background: rgba(255, 255, 255, 0.06);
}

.ordered-node-item.is-primary {
  border-color: rgba(59, 130, 246, 0.5);
  background: rgba(59, 130, 246, 0.1);
}

.node-info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.node-rank {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--text-muted);
  width: 22px;
}

.node-rank.rank-first {
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

.node-name {
  font-size: 0.82rem;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-protocol {
  font-size: 0.7rem;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.06);
  padding: 1px 5px;
  border-radius: 4px;
}

.node-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.btn-sort-action {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-base);
  border-radius: 4px;
  padding: 2px 7px;
  font-size: 0.75rem;
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
