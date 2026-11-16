<script setup lang="ts">
// 用户新增/编辑表单弹窗
// 包含：用户基础信息 / 快捷凭证提取 / 节点端点 (Endpoints) 灵活配置 / config_override
// 契约：props { open, editing, nodes }，emits { close, saved }
import { inject, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { Node, User } from '../types'
import { parseCredentials, randomPassword, randomUUID } from '../utils'
import { ToastKey, type ToastType } from '../injectionKeys'
import { useStorage } from '../composables/useStorage'
import UserFormCredentialInput from './UserFormCredentialInput.vue'
import UserFormExtractSection from './UserFormExtractSection.vue'
import UserFormEndpoints, { type EndpointFormItem } from './UserFormEndpoints.vue'

const { t } = useI18n()

const { set: storageSet, get: storageGet, remove: storageRemove } = useStorage('local')

const props = defineProps<{
  open: boolean
  editing: User | null
  nodes: Node[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const toast = inject<ToastType extends never ? never : (msg: string, type?: ToastType) => void>(ToastKey)

// 用户页可能在节点管理页之后才打开，不能只依赖父组件首次加载的节点快照。
const availableNodes = ref<Node[]>([...props.nodes])

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
  endpoints: [] as EndpointFormItem[],
  node_ids: [] as number[],
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
  })
  showExtract.value = false
  extractInput.value = ''
}

function nodeDefaultTag(node: Node) {
  return node.node_name || node.tag || `Node-${node.id}`
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
  if (clearDraft) storageRemove(draftKey())
  emit('close')
}

function loadForm(u: User) {
  let eps: EndpointFormItem[] = []
  const legacyIndependentNodeIDs = new Set(u.independent_node_ids || [])
  if (u.endpoints && u.endpoints.length) {
    eps = u.endpoints.map((e) => ({
      id: e.id,
      node_id: e.node_id,
      tag: e.tag,
      tagManuallyEdited: (() => {
        const node = availableNodes.value.find((n) => n.id === e.node_id)
        return Boolean(e.tag && (!node || e.tag !== nodeDefaultTag(node)))
      })(),
      uuid: e.uuid,
      password: e.password || '',
      is_active: e.is_active ?? true,
      is_urltest: e.is_urltest ?? !legacyIndependentNodeIDs.has(e.node_id),
    }))
  } else if (u.node_ids && u.node_ids.length) {
    eps = u.node_ids.map((nid) => {
      const node = props.nodes.find((n) => n.id === nid)
      return {
        node_id: nid,
        tag: node?.node_name || node?.tag || `Node-${nid}`,
        tagManuallyEdited: false,
        uuid: u.uuid || '',
        password: u.password || '',
        is_active: true,
        is_urltest: !legacyIndependentNodeIDs.has(nid),
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
    const draft = storageGet(draftKey())
    if (draft) {
      try {
        Object.assign(form, JSON.parse(draft))
        showExtract.value = false
        extractInput.value = ''
      } catch {
        storageRemove(draftKey())
        if (props.editing) loadForm(props.editing)
        else resetForm()
      }
    } else if (props.editing) loadForm(props.editing)
    else resetForm()
    formInitialized.value = true
  },
)

watch(
  () => props.nodes,
  (nodes) => {
    availableNodes.value = [...nodes]
  },
  { deep: true },
)

watch(form, (value) => {
  if (props.open && formInitialized.value) storageSet(draftKey(), JSON.stringify(value))
}, { deep: true })

function updateEndpoints(next: EndpointFormItem[]) {
  form.endpoints = next
}

function applyExtract() {
  const { uuid, password } = parseCredentials(extractInput.value)
  if (uuid) form.uuid = uuid
  if (password) form.password = password
  toast?.(uuid || password ? t('users.form.extractSuccess') : t('users.form.extractFail'), uuid || password ? 'info' : 'error')
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
      is_urltest: ep.is_urltest ?? true,
      sort_order: idx,
    })),
    node_ids: Array.from(new Set(form.endpoints.map((e) => e.node_id))),
    is_active: true,
  }
  try {
    if (props.editing) {
      await api.users.update(props.editing.id, payload)
      toast?.(t('users.updated'))
    } else {
      await api.users.create(payload)
      toast?.(t('users.created'))
    }
    storageRemove(draftKey())
    emit('close')
    emit('saved')
  } catch (e) {
    toast?.((e as Error).message, 'error')
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
      <form class="modal-form" @submit.prevent="saveUser">
        <UserFormExtractSection
          :show-extract="showExtract"
          :extract-input="extractInput"
          @update:show-extract="(val) => (showExtract = val)"
          @update:extract-input="(val) => (extractInput = val)"
          @apply="applyExtract"
        />

        <div class="form-group">
          <label>{{ t('users.form.name') }}</label>
          <input v-model="form.name" class="form-control" :placeholder="t('users.form.namePlaceholder')" required />
        </div>
        <div class="form-group">
          <label>{{ t('users.form.remark') }}</label>
          <input v-model="form.remark" class="form-control" :placeholder="t('users.form.remarkPlaceholder')" />
        </div>
        <UserFormCredentialInput
          :label="t('users.form.token')"
          v-model="form.token"
          :placeholder="t('users.form.tokenPlaceholder')"
          :random-label="t('users.form.randomGenerate')"
          @random="form.token = randomUUID()"
        />
        <UserFormCredentialInput
          :label="t('users.form.uuid')"
          v-model="form.uuid"
          :placeholder="t('users.form.uuidPlaceholder')"
          :random-label="t('users.form.randomUuid')"
          @random="form.uuid = randomUUID()"
        />
        <UserFormCredentialInput
          :label="t('users.form.password')"
          v-model="form.password"
          :placeholder="t('users.form.passwordPlaceholder')"
          :random-label="t('users.form.randomPassword')"
          @random="form.password = randomPassword()"
        />
        <div class="form-group">
          <label>{{ t('users.form.configOverride') }}</label>
          <textarea
            v-model="form.config_override"
            class="form-control code-textarea"
            rows="3"
            :placeholder="t('users.form.configOverridePlaceholder')"
          ></textarea>
          <div class="field-tip">{{ t('users.form.configOverrideTip') }}</div>
        </div>

        <UserFormEndpoints
          :endpoints="form.endpoints"
          :available-nodes="availableNodes"
          :default-uuid="form.uuid"
          :default-password="form.password"
          @update:endpoints="updateEndpoints"
        />

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
.code-textarea {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}
.field-tip {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-top: 4px;
}
</style>
