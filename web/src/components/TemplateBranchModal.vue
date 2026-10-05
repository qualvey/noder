<script setup lang="ts">
// 模板分支管理弹窗
// 功能：列表 / 创建分支（从模板复制）/ 编辑内容 / 删除
import { inject, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { TemplateBranch, Template } from '../types'
import { ToastType } from '@/App.vue'

const { t } = useI18n()
const toast = inject('toast') as (msg: string, type?: ToastType) => void

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
    toast((e as Error).message, 'error')
  } finally {
    loading.value = false
  }
}

async function fetchTemplates() {
  templates.value = []
  try {
    const res = await fetch('/api/templates', {
      headers: { 'X-Admin-Token': sessionStorage.getItem('noder_admin_token') || '' },
    })
    if (res.ok) {
      templates.value = await res.json()
    }
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
    toast('分支名称不能为空', 'error')
    return
  }
  if (!createForm.target) {
    toast('目标类型不能为空', 'error')
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
    toast('分支创建成功', 'info')
    closeEdit()
    await fetchBranches()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

async function handleUpdate() {
  if (!selectedBranch.value) return
  if (!editForm.name.trim()) {
    toast('分支名称不能为空', 'error')
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
    toast('分支更新成功', 'info')
    closeEdit()
    await fetchBranches()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

async function handleDelete(branch: TemplateBranch) {
  if (!confirm(`确定删除分支 "${branch.name}" 吗？`)) return
  try {
    await api.templateBranches.remove(branch.id)
    toast('分支已删除', 'info')
    if (selectedBranch.value?.id === branch.id) {
      closeEdit()
    }
    await fetchBranches()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

async function handleCreateFromTemplate(templateId: string) {
  if (!templateId) return
  const tpl = templates.value.find(t => String(t.id) === String(templateId))
  if (!tpl) return

  const name = `${(tpl as any).name || '模板'} 的副本`
  const content = (tpl as any).content || ''

  try {
    await api.templateBranches.create({
      name,
      base_template_id: Number(templateId),
      target: (tpl as any).target || 'sing-box',
      content_format: (tpl as any).content_format || 'json',
      content,
      remark: '从模板创建',
    })
    toast('分支创建成功', 'info')
    await fetchBranches()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

function onOverlayClick(e: MouseEvent) {
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
  <div v-if="open" class="modal-overlay active" @click="onOverlayClick">
    <div class="modal" style="max-width: 900px; width: 90vw;">
      <div class="modal-header">
        <div class="modal-title">{{ t('templateBranch.title') || '模板分支管理' }}</div>
        <button class="modal-close" @click="$emit('close')">&times;</button>
      </div>

      <div v-if="isEditing" class="modal-body">
        <!-- 创建新分支 -->
        <template v-if="!selectedBranch">
          <h3 style="margin: 0 0 16px; font-size: 1rem;">{{ t('templateBranch.create') || '创建新分支' }}</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>{{ t('templateBranch.name') || '分支名称' }}</label>
              <input v-model="createForm.name" type="text" :placeholder="t('templateBranch.namePlaceholder') || '输入分支名称'" />
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.target') || '目标类型' }}</label>
              <select v-model="createForm.target">
                <option value="sing-box">Sing-Box</option>
                <option value="mihomo">Mihomo</option>
                <option value="xray">Xray</option>
              </select>
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.contentFormat') || '内容格式' }}</label>
              <select v-model="createForm.content_format">
                <option value="json">JSON</option>
                <option value="yaml">YAML</option>
              </select>
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.baseTemplate') || '基础模板（可选）' }}</label>
              <select v-model="createForm.base_template_id">
                <option value="">-- 不使用模板 --</option>
                <optgroup :label="t('templateBranch.fromTemplate') || '从模板创建'">
                  <option v-for="tpl in templates" :key="tpl.id" :value="String(tpl.id)">
                    {{ tpl.name }} ({{ tpl.target }})
                  </option>
                </optgroup>
              </select>
            </div>
            <div class="form-group" style="grid-column: 1 / -1;">
              <label>{{ t('templateBranch.remark') || '备注' }}</label>
              <input v-model="createForm.remark" type="text" :placeholder="t('templateBranch.remarkPlaceholder') || '可选备注'" />
            </div>
          </div>
          <div style="display: flex; gap: 12px; justify-content: flex-end; margin-top: 20px;">
            <button class="btn btn-secondary" @click="closeEdit">{{ t('common.cancel') || '取消' }}</button>
            <button class="btn btn-primary" @click="handleCreate">{{ t('common.create') || '创建' }}</button>
          </div>
        </template>

        <!-- 编辑分支 -->
        <template v-else>
          <h3 style="margin: 0 0 16px; font-size: 1rem;">{{ t('templateBranch.edit') || '编辑分支' }}</h3>
          <div class="form-grid">
            <div class="form-group">
              <label>{{ t('templateBranch.name') || '分支名称' }}</label>
              <input v-model="editForm.name" type="text" />
            </div>
            <div class="form-group">
              <label>{{ t('templateBranch.contentFormat') || '内容格式' }}</label>
              <select v-model="editForm.content_format">
                <option value="json">JSON</option>
                <option value="yaml">YAML</option>
              </select>
            </div>
            <div class="form-group" style="grid-column: 1 / -1;">
              <label>{{ t('templateBranch.remark') || '备注' }}</label>
              <input v-model="editForm.remark" type="text" />
            </div>
            <div class="form-group" style="grid-column: 1 / -1; display: flex; align-items: center; gap: 8px;">
              <input type="checkbox" id="branch-active" v-model="editForm.is_active" />
              <label for="branch-active" style="margin: 0;">{{ t('common.enabled') || '启用' }}</label>
            </div>
            <div class="form-group" style="grid-column: 1 / -1;">
              <label>{{ t('templateBranch.content') || '模板内容' }}</label>
              <textarea
                v-model="editForm.content"
                class="code-textarea"
                :placeholder="t('templateBranch.contentPlaceholder') || '输入模板内容，使用 {{uuid}} {{password}} {{token}} 等占位符'"
                style="min-height: 300px; font-family: var(--font-mono); font-size: 0.85rem;"
              ></textarea>
            </div>
          </div>
          <div style="display: flex; gap: 12px; justify-content: flex-end; margin-top: 20px;">
            <button class="btn btn-secondary" @click="closeEdit">{{ t('common.cancel') || '取消' }}</button>
            <button class="btn btn-primary" @click="handleUpdate">{{ t('common.save') || '保存' }}</button>
          </div>
        </template>
      </div>

      <!-- 分支列表 -->
      <div v-else class="modal-body">
        <div style="display: flex; gap: 12px; margin-bottom: 16px;">
          <button class="btn btn-primary" @click="openCreate">
            + {{ t('templateBranch.create') || '创建分支' }}
          </button>
          <select
            style="padding: 8px 12px; border-radius: 6px; border: 1px solid var(--border-glass); background: var(--bg-elevated); color: var(--text-main);"
            @change="(e) => { const v = (e.target as HTMLSelectElement).value; if (v) handleCreateFromTemplate(v); (e.target as HTMLSelectElement).value = ''; }"
          >
            <option value="">-- {{ t('templateBranch.quickCreate') || '快速从模板创建' }} --</option>
            <option v-for="tpl in templates" :key="tpl.id" :value="String(tpl.id)">
              {{ tpl.name }}
            </option>
          </select>
        </div>

        <div v-if="loading" style="text-align: center; padding: 40px; color: var(--text-muted);">
          {{ t('common.loading') || '加载中...' }}
        </div>

        <div v-else-if="!branches.length" style="text-align: center; padding: 40px; color: var(--text-muted);">
          {{ t('templateBranch.empty') || '暂无模板分支' }}
        </div>

        <table v-else class="data-table">
          <thead>
            <tr>
              <th>{{ t('templateBranch.name') || '名称' }}</th>
              <th>{{ t('templateBranch.target') || '目标' }}</th>
              <th>{{ t('templateBranch.format') || '格式' }}</th>
              <th>{{ t('common.status') || '状态' }}</th>
              <th>{{ t('common.actions') || '操作' }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="branch in branches" :key="branch.id">
              <td>
                <div style="font-weight: 600;">{{ branch.name }}</div>
                <div v-if="branch.remark" style="font-size: 0.75rem; color: var(--text-muted);">{{ branch.remark }}</div>
              </td>
              <td>
                <span class="badge-protocol">{{ branch.target.toUpperCase() }}</span>
              </td>
              <td>
                <span style="font-family: var(--font-mono); font-size: 0.8rem;">{{ branch.content_format.toUpperCase() }}</span>
              </td>
              <td>
                <span class="status-badge" :class="branch.is_active ? 'active' : 'inactive'">
                  <span class="status-dot"></span>
                  {{ branch.is_active ? (t('common.enabled') || '启用') : (t('common.disabled') || '停用') }}
                </span>
              </td>
              <td>
                <div style="display: flex; gap: 8px;">
                  <button class="btn btn-secondary btn-sm" @click="openEdit(branch)">{{ t('common.edit') || '编辑' }}</button>
                  <button class="btn btn-danger btn-sm" @click="handleDelete(branch)">{{ t('common.delete') || '删除' }}</button>
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
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
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
