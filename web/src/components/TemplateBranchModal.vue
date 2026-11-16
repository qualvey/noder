<script setup lang="ts">
// 模板分支管理弹窗
// 功能：列表 / 创建分支（从模板复制）/ 编辑内容 / 删除
import { inject, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { TemplateBranch, Template } from '../types'
import { ToastKey, PopoverKey } from '../injectionKeys'

const { t } = useI18n()
const toast = inject(ToastKey)
const popover = inject(PopoverKey)

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)
const branches = ref<TemplateBranch[]>([])
const templates = ref<Template[]>([])
const selectedBranch = ref<TemplateBranch | null>(null)
const isEditing = ref(false)

// 编辑表单
const editForm = reactive({
  name: '',
  content: '',
  content_format: '',
  remark: '',
  is_active: true,
})

// 新建表单
const createForm = reactive({
  name: '',
  base_template_id: '' as string,
  target: 'sing-box',
  content_format: 'json',
  remark: '',
})

async function fetchBranches() {
  loading.value = true
  try {
    branches.value = await api.templateBranches.list()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  } finally {
    loading.value = false
  }
}

async function fetchTemplates() {
  templates.value = []
  try {
    templates.value = await api.templates.list()
  } catch {
    // ignore
  }
}

function openCreate() {
  selectedBranch.value = null
  isEditing.value = true
  Object.assign(createForm, { name: '', base_template_id: '', target: 'sing-box', content_format: 'json', remark: '' })
}

function openEdit(branch: TemplateBranch) {
  selectedBranch.value = branch
  isEditing.value = true
  Object.assign(editForm, {
    name: branch.name,
    content: branch.content,
    content_format: branch.content_format,
    remark: branch.remark || '',
    is_active: branch.is_active,
  })
}

function closeEdit() {
  selectedBranch.value = null
  isEditing.value = false
}

async function handleCreate() {
  if (!createForm.name.trim()) {
    toast?.(t('templateBranch.nameRequired'), 'error')
    return
  }
  if (!createForm.target) {
    toast?.(t('templateBranch.targetRequired'), 'error')
    return
  }

  // 如果选择了基础模板，获取其内容
  let content = ''
  if (createForm.base_template_id) {
    const tpl = templates.value.find(t => String(t.id) === String(createForm.base_template_id))
    if (tpl) {
      content = tpl.content || ''
    }
  }

  try {
    await api.templateBranches.create({
      name: createForm.name.trim(),
      base_template_id: createForm.base_template_id ? Number(createForm.base_template_id) : undefined,
      target: createForm.target,
      content_format: createForm.content_format,
      content: content || (createForm.content_format === 'json' ? '{}' : '# empty'),
      remark: createForm.remark || undefined,
    })
    toast?.(t('templateBranch.created'), 'info')
    closeEdit()
    await fetchBranches()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

async function handleUpdate() {
  if (!selectedBranch.value) return
  if (!editForm.name.trim()) {
    toast?.(t('templateBranch.nameRequired'), 'error')
    return
  }

  try {
    await api.templateBranches.update(selectedBranch.value.id, {
      name: editForm.name.trim(),
      content: editForm.content,
      content_format: editForm.content_format,
      remark: editForm.remark || undefined,
      is_active: editForm.is_active,
    })
    toast?.(t('templateBranch.updated'), 'info')
    closeEdit()
    await fetchBranches()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

async function handleDelete(branch: TemplateBranch, el: Element) {
  popover?.show(el, t('templateBranch.deleteConfirm', { name: branch.name }), async () => {
    try {
      await api.templateBranches.remove(branch.id)
      toast?.(t('templateBranch.deleted'), 'info')
      if (selectedBranch.value?.id === branch.id) {
        closeEdit()
      }
      await fetchBranches()
    } catch (e) {
      toast?.((e as Error).message, 'error')
    }
  })
}

async function handleCreateFromTemplate(templateId: string) {
  if (!templateId) return
  const tpl = templates.value.find((item) => String(item.id) === String(templateId))
  if (!tpl) return

  const name = `${tpl.name || t('templateBranch.defaultTemplateName')} ${t('templateBranch.copySuffix')}`
  const content = tpl.content || ''

  try {
    await api.templateBranches.create({
      name,
      base_template_id: Number(templateId),
      target: tpl.target || 'sing-box',
      content_format: tpl.content_format || 'json',
      content,
      remark: t('templateBranch.createdFromTemplate'),
    })
    toast?.(t('templateBranch.created'), 'info')
    await fetchBranches()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

function handleQuickCreateChange(e: Event) {
  const select = e.target as HTMLSelectElement
  const v = select.value
  if (v) handleCreateFromTemplate(v)
  select.value = ''
}

function onOverlayMouseDown(e: MouseEvent) {
  if ((e.target as HTMLElement).classList.contains('modal-overlay')) {
    emit('close')
  }
}

onMounted(() => {
  if (props.open) {
    fetchBranches()
    fetchTemplates()
  }
})

watch(() => props.open, (val) => {
  if (val) {
    fetchBranches()
    fetchTemplates()
    closeEdit()
  }
})
</script>

<template>
  <div v-if="open" class="modal-overlay active" @mousedown="onOverlayMouseDown">
    <div class="modal modal-wide">
      <div class="modal-header">
        <div class="modal-title">{{ t('templateBranch.title') }}</div>
        <button class="modal-close" @click="$emit('close')">&times;</button>
      </div>

      <div v-if="isEditing" class="modal-body">
        <!-- 创建新分支 -->
        <template v-if="!selectedBranch">
          <h3 class="branch-section-title">{{ t('templateBranch.create') }}</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>{{ t('templateBranch.name') }}</label>
              <input v-model="createForm.name" type="text" :placeholder="t('templateBranch.namePlaceholder')" />
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.target') }}</label>
              <select v-model="createForm.target">
                <option value="sing-box">Sing-Box</option>
                <option value="mihomo">Mihomo</option>
                <option value="xray">Xray</option>
              </select>
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.contentFormat') }}</label>
              <select v-model="createForm.content_format">
                <option value="json">JSON</option>
                <option value="yaml">YAML</option>
              </select>
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.baseTemplate') }}</label>
              <select v-model="createForm.base_template_id">
                <option value="">-- {{ t('templateBranch.noTemplate') }} --</option>
                <optgroup :label="t('templateBranch.fromTemplate')">
                  <option v-for="tpl in templates" :key="tpl.id" :value="String(tpl.id)">
                    {{ tpl.name }} ({{ tpl.target }})
                  </option>
                </optgroup>
              </select>
            </div>
            <div class="form-group form-span-full">
              <label>{{ t('templateBranch.remark') }}</label>
              <input v-model="createForm.remark" type="text" :placeholder="t('templateBranch.remarkPlaceholder')" />
            </div>
          </div>
          <div class="branch-form-actions">
            <button class="btn btn-secondary" @click="closeEdit">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="handleCreate">{{ t('common.create') }}</button>
          </div>
        </template>

        <!-- 编辑分支 -->
        <template v-else>
          <h3 class="branch-section-title">{{ t('templateBranch.edit') }}</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>{{ t('templateBranch.name') }}</label>
              <input v-model="editForm.name" type="text" />
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.contentFormat') }}</label>
              <select v-model="editForm.content_format">
                <option value="json">JSON</option>
                <option value="yaml">YAML</option>
              </select>
            </div>
            <div class="form-group form-span-full">
              <label>{{ t('templateBranch.remark') }}</label>
              <input v-model="editForm.remark" type="text" />
            </div>
            <div class="form-group form-span-full checkbox-row">
              <input type="checkbox" id="branch-active" v-model="editForm.is_active" />
              <label for="branch-active" class="checkbox-label">{{ t('common.enabled') }}</label>
            </div>
            <div class="form-group form-span-full">
              <label>{{ t('templateBranch.content') }}</label>
              <textarea
                v-model="editForm.content"
                class="code-textarea"
                :placeholder="t('templateBranch.contentPlaceholder')"
              ></textarea>
            </div>
          </div>
          <div class="branch-form-actions">
            <button class="btn btn-secondary" @click="closeEdit">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="handleUpdate">{{ t('common.save') }}</button>
          </div>
        </template>
      </div>

      <!-- 分支列表 -->
      <div v-else class="modal-body">
        <div class="branch-toolbar">
          <button class="btn btn-primary" @click="openCreate">
            + {{ t('templateBranch.create') }}
          </button>
          <select
            class="branch-quick-create"
            @change="handleQuickCreateChange"
          >
            <option value="">-- {{ t('templateBranch.quickCreate') }} --</option>
            <option v-for="tpl in templates" :key="tpl.id" :value="String(tpl.id)">
              {{ tpl.name }}
            </option>
          </select>
        </div>

        <div v-if="loading" class="branch-state-message">
          {{ t('common.loading') }}
        </div>

        <div v-else-if="!branches.length" class="branch-state-message">
          {{ t('templateBranch.empty') }}
        </div>

        <table v-else class="data-table">
          <thead>
            <tr>
              <th>{{ t('templateBranch.name') }}</th>
              <th>{{ t('templateBranch.target') }}</th>
              <th>{{ t('templateBranch.format') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="branch in branches" :key="branch.id">
              <td>
                <div class="branch-name">{{ branch.name }}</div>
                <div v-if="branch.remark" class="branch-remark">{{ branch.remark }}</div>
              </td>
              <td>
                <span class="badge-protocol">{{ branch.target.toUpperCase() }}</span>
              </td>
              <td>
                <span class="mono-text">{{ branch.content_format.toUpperCase() }}</span>
              </td>
              <td>
                <span class="status-badge" :class="branch.is_active ? 'active' : 'inactive'">
                  <span class="status-dot"></span>
                  {{ branch.is_active ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="btn btn-secondary btn-sm" @click="openEdit(branch)">{{ t('common.edit') }}</button>
                  <button class="btn btn-danger btn-sm" @click="handleDelete(branch, $event.currentTarget as Element)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-wide {
  max-width: 900px;
  width: 90vw;
}
.branch-section-title {
  margin: 0 0 16px;
  font-size: 1rem;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
.form-span-full {
  grid-column: 1 / -1;
}
.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.checkbox-label {
  margin: 0;
}
.branch-form-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 20px;
}
.branch-toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.branch-quick-create {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--border-glass);
  background: var(--bg-elevated);
  color: var(--text-main);
}
.branch-state-message {
  text-align: center;
  padding: 40px;
  color: var(--text-muted);
}
.branch-name {
  font-weight: 600;
}
.branch-remark {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.mono-text {
  font-family: var(--font-mono);
  font-size: 0.8rem;
}
.row-actions {
  display: flex;
  gap: 8px;
}
.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-group label {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-main);
}
.form-group input,
.form-group select,
.form-group textarea {
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--border-glass);
  background: var(--bg-input);
  color: var(--text-main);
  font-size: 0.9rem;
}
.form-group input:focus,
.form-group select:focus,
.form-group textarea:focus {
  outline: none;
  border-color: var(--primary);
}
.code-textarea {
  width: 100%;
  resize: vertical;
}
.badge-protocol {
  font-size: 0.7rem;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(59, 130, 246, 0.2);
  color: #93c5fd;
  font-family: var(--font-mono);
  font-weight: 600;
}
</style>
