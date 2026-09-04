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
  const s = new Set(form.node_ids)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  form.node_ids = [...s]
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
  padding: 4px 8px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.82rem;
}
</style>
