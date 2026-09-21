<script setup lang="ts">
// 文件分发：先选类型（普通文件 / zip / 文本），按类型展示对应输入
// 普通文件与文本用共享 token 下载（可重置）；zip 按用户个性化渲染，在用户列表右键下载
import { computed, inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile } from '../types'
import { apiLinkPrefix, formatFileSize } from '../utils'

const { t } = useI18n()
const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }

const files = ref<DistFile[]>([])
const showModal = ref(false)
const sharedToken = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

// 选中的本地文件与拖拽交互状态
const selectedFile = ref<File | null>(null)
const isDragging = ref(false)
const regularSourceTab = ref<'local' | 'remote'>('local')

// 上传进度与防重复提交并发锁
const isUploading = ref(false)
const uploadProgress = ref(0)
const uploadedBytes = ref(0)
const totalBytes = ref(0)

// 文本文件编辑弹窗
const showTextModal = ref(false)
const textEdit = ref<DistFile | null>(null)
const textForm = ref({ downloadName: '', remark: '', content: '' })
const textLoading = ref(false)

type FileType = 'apk' | 'zip' | 'text'
const form = ref({
  type: 'apk' as FileType,
  templateName: '',
  downloadName: '',
  remark: '',
  sourceUrl: '',
  contentText: '',
})

const typeOptions = computed(() => [
  { value: 'apk' as const, label: t('files.modal.typeRegular'), desc: t('files.modal.typeRegularDesc') },
  { value: 'zip' as const, label: t('files.modal.typeZip'), desc: t('files.modal.typeZipDesc') },
  { value: 'text' as const, label: t('files.modal.typeText'), desc: t('files.modal.typeTextDesc') },
])

// 步骤计算
const currentStep = computed(() => {
  const isText = form.value.type === 'text'
  const isZip = form.value.type === 'zip'
  const hasFileOrContent = isText
    ? form.value.contentText.trim().length > 0
    : isZip
      ? !!selectedFile.value
      : regularSourceTab.value === 'remote'
        ? form.value.sourceUrl.trim().length > 0
        : !!selectedFile.value
  if (!hasFileOrContent) return 2
  return 3
})

// 防重传 / 同名同大小冲突预警
const duplicateWarning = computed(() => {
  if (!selectedFile.value) return null
  const targetName = form.value.downloadName.trim() || selectedFile.value.name
  const match = files.value.find(
    (f) =>
      f.is_active &&
      (f.original_name === selectedFile.value!.name || (f.download_name && f.download_name === targetName)) &&
      f.size === selectedFile.value!.size
  )
  return match || null
})

async function fetchData() {
  try {
    const [f, s] = await Promise.all([api.files.list(), api.settings.sharedToken()])
    files.value = f
    sharedToken.value = s.token
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

function openCreate() {
  Object.assign(form.value, {
    type: 'apk',
    templateName: '',
    downloadName: '',
    remark: '',
    sourceUrl: '',
    contentText: '',
  })
  selectedFile.value = null
  isDragging.value = false
  regularSourceTab.value = 'local'
  isUploading.value = false
  uploadProgress.value = 0
  uploadedBytes.value = 0
  totalBytes.value = 0
  if (fileInput.value) fileInput.value.value = ''
  showModal.value = true
}

function pickType(t: FileType) {
  form.value.type = t
  selectedFile.value = null
  if (fileInput.value) fileInput.value.value = ''
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}

function onDragLeave(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  const droppedFiles = e.dataTransfer?.files
  if (droppedFiles && droppedFiles.length > 0) {
    handleFileSelected(droppedFiles[0])
  }
}

function onFileInputChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    handleFileSelected(target.files[0])
  }
}

function handleFileSelected(file: File) {
  if (form.value.type === 'zip' && !file.name.toLowerCase().endsWith('.zip')) {
    toast('ZIP 配置包必须为 .zip 压缩文件', 'error')
    return
  }
  selectedFile.value = file
  if (!form.value.downloadName.trim()) {
    form.value.downloadName = file.name
  }
}

function clearSelectedFile() {
  selectedFile.value = null
  if (fileInput.value) fileInput.value.value = ''
}

async function submitUpload() {
  if (isUploading.value) return // 防重传并发锁

  const isText = form.value.type === 'text'
  const isZip = form.value.type === 'zip'
  const isRegular = form.value.type === 'apk'

  const hasFile = !!selectedFile.value
  const hasUrl = regularSourceTab.value === 'remote' && form.value.sourceUrl.trim().length > 0
  const hasContent = form.value.contentText.trim().length > 0

  if (isText) {
    if (!hasContent) {
      toast(t('files.modal.fillContentFirst'), 'error')
      return
    }
  } else if (isZip) {
    if (!hasFile) {
      toast(t('files.modal.selectZipFirst'), 'error')
      return
    }
  } else if (isRegular) {
    if (regularSourceTab.value === 'local' && !hasFile) {
      toast(t('files.modal.selectFileFirst'), 'error')
      return
    }
    if (regularSourceTab.value === 'remote' && !hasUrl) {
      toast(t('files.modal.fillUrlFirst'), 'error')
      return
    }
  }

  isUploading.value = true
  uploadProgress.value = 0
  uploadedBytes.value = 0
  totalBytes.value = selectedFile.value?.size || 0

  const dlName = form.value.downloadName.trim()

  try {
    if (hasUrl) {
      // Remote files use the dedicated JSON endpoint. /api/files is the
      // multipart local-upload endpoint and requires a file field.
      await api.files.createRemote({
        url: form.value.sourceUrl.trim(),
        name: dlName || undefined,
        download_name: dlName || undefined,
        remark: form.value.remark.trim() || undefined,
        file_type: form.value.type,
      })
    } else {
      const fd = new FormData()
      if (hasFile) fd.append('file', selectedFile.value!)
      if (hasContent) fd.append('content_text', form.value.contentText)
      fd.append('file_type', form.value.type)
      fd.append('template_name', form.value.templateName.trim())
      if (dlName) {
        fd.append('name', dlName)
        fd.append('download_name', dlName)
      }
      fd.append('remark', form.value.remark.trim())

      if (hasFile) {
        await api.files.createWithProgress(fd, (pct, loaded, total) => {
          uploadProgress.value = pct
          uploadedBytes.value = loaded
          totalBytes.value = total
        })
      } else {
        await api.files.create(fd)
      }
    }

    toast(hasUrl ? t('files.successRemoteAdded') : hasContent ? t('files.successTextSaved') : t('files.successUploaded'))
    showModal.value = false
    fetchData()
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    isUploading.value = false
  }
}


// 普通文件/文本：复制共享 token 链接
function copySharedLink(f: DistFile) {
  if (!sharedToken.value) return
  const url = `${location.origin}${apiLinkPrefix()}/dl/${f.id}?token=${encodeURIComponent(sharedToken.value)}`
  navigator.clipboard.writeText(url).then(() => toast(t('common.copied')), () => toast(t('common.copyFailed'), 'error'))
}

async function copySharedToken() {
  await navigator.clipboard.writeText(sharedToken.value)
  toast(t('common.copied'))
}

function resetSharedToken(btn: Element) {
  popover.show(btn, t('files.resetSharedConfirm'), async () => {
    try {
      const r = await api.settings.resetSharedToken()
      sharedToken.value = r.token
      toast(t('common.success'))
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  })
}

function toggleActive(f: DistFile) {
  api.files.update(f.id, { is_active: !f.is_active })
    .then(() => {
      toast(f.is_active ? t('files.statusInactive') : t('files.statusActive'))
      fetchData()
    })
    .catch((e) => toast(e.message, 'error'))
}

function refreshRemote(f: DistFile, btn: HTMLButtonElement) {
  btn.disabled = true
  btn.textContent = t('files.refreshing')
  api.files.refresh(f.id)
    .then(() => {
      toast(t('files.successRefreshed'))
      fetchData()
    })
    .catch((e) => {
      toast(e.message, 'error')
      btn.disabled = false
      btn.textContent = '🔄 ' + t('common.refresh')
    })
}

function removeFile(f: DistFile) {
  api.files.remove(f.id)
    .then(() => {
      toast(t('common.deleted'))
      fetchData()
    })
    .catch((e) => toast(e.message, 'error'))
}

// 文本文件编辑：拉取内容 -> 弹窗 -> 保存
async function openTextEdit(f: DistFile) {
  textLoading.value = true
  textEdit.value = f
  showTextModal.value = true
  try {
    const r = await api.files.getContent(f.id)
    textForm.value = { downloadName: f.download_name || f.original_name || f.name, remark: f.remark || '', content: r.content }
  } catch (e) {
    toast((e as Error).message, 'error')
    showTextModal.value = false
  } finally {
    textLoading.value = false
  }
}

async function saveTextEdit() {
  if (!textEdit.value) return
  try {
    // 内容 + 元数据（文件名/备注）
    await api.files.updateContent(textEdit.value.id, textForm.value.content)
    const dlName = textForm.value.downloadName.trim()
    await api.files.update(textEdit.value.id, {
      name: dlName || textEdit.value.name,
      download_name: dlName || null,
      remark: textForm.value.remark.trim() || null,
    })
    toast(t('files.successTextSaved'))
    showTextModal.value = false
    fetchData()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

onMounted(fetchData)
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">{{ t('files.headerTitle') }}</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <button class="btn btn-primary" @click="openCreate"><span>+</span> {{ t('files.uploadBtn') }}</button>
      </div>
    </div>

    <div class="shared-token-bar">
      <span style="font-weight: 600">{{ t('files.sharedTokenBar') }}</span>
      <span style="color: var(--text-muted); font-size: 0.8rem">{{ t('files.sharedTokenTip') }}</span>
      <code class="shared-token-value">{{ sharedToken }}</code>
      <button class="btn btn-secondary btn-sm" @click="copySharedToken">{{ t('files.copySharedToken') }}</button>
      <button class="btn btn-danger btn-sm" @click="resetSharedToken($event.currentTarget as Element)">{{ t('files.resetSharedToken') }}</button>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th>{{ t('files.colId') }}</th>
            <th>{{ t('files.colName') }}</th>
            <th>{{ t('files.colType') }}</th>
            <th>{{ t('files.colSize') }}</th>
            <th>{{ t('files.colSource') }}</th>
            <th>{{ t('files.colTemplate') }}</th>
            <th>{{ t('files.colStatus') }}</th>
            <th>{{ t('files.colLink') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!files.length">
            <td colspan="9" style="text-align: center; color: var(--text-muted); padding: 30px">{{ t('files.emptyText') }}</td>
          </tr>
          <tr v-for="f in files" :key="f.id">
            <td>{{ f.id }}</td>
            <td>
              <code>{{ f.download_name || f.original_name || f.name }}</code>
              <div v-if="f.remark" style="font-size: 0.72rem; color: var(--text-muted); margin-top: 2px">{{ f.remark }}</div>
            </td>
            <td>
              <span class="badge" :class="f.file_type === 'zip' ? 'badge-vless' : f.file_type === 'text' ? 'badge-anytls' : 'badge-tuic'">
                {{ f.file_type === 'apk' ? t('files.typeRegularShort') : f.file_type.toUpperCase() }}
              </span>
            </td>
            <td>{{ formatFileSize(f.size) }}</td>
            <td>
              <span v-if="f.source_url" :title="f.source_url" style="cursor: help">🔗 {{ t('files.sourceRemote') }}</span>
              <span v-else-if="f.file_type === 'text'" style="color: var(--text-muted)">📝 {{ t('files.sourceText') }}</span>
              <span v-else style="color: var(--text-muted)">📁 {{ t('files.sourceLocal') }}</span>
            </td>
            <td>{{ f.file_type === 'zip' ? f.template_name || '-' : '-' }}</td>
            <td>
              <span v-if="f.is_active" style="color: var(--accent-emerald)">{{ t('files.statusActive') }}</span>
              <span v-else style="color: var(--accent-rose)">{{ t('files.statusInactive') }}</span>
            </td>
            <td>
              <template v-if="f.file_type === 'zip'">
                <span style="font-size: 0.75rem; color: var(--text-muted)">{{ t('files.zipPersonalHint') }}</span>
              </template>
              <template v-else>
                <button class="btn btn-secondary btn-sm" @click="copySharedLink(f)">{{ t('files.copySharedLink') }}</button>
              </template>
            </td>
            <td>
              <div style="display: flex; gap: 6px">
                <button v-if="f.file_type === 'text'" class="btn btn-secondary btn-sm" @click="openTextEdit(f)">{{ t('common.edit') }}</button>
                <button v-if="f.source_url" class="btn btn-secondary btn-sm" @click="refreshRemote(f, $event.currentTarget as HTMLButtonElement)">{{ t('files.refreshRemote') }}</button>
                <button class="btn btn-secondary btn-sm" @click="toggleActive(f)">{{ f.is_active ? t('files.toggleInactive') : t('files.toggleActive') }}</button>
                <button class="btn btn-danger btn-sm" @click="popover.show($event.currentTarget as Element, t('files.deleteConfirm'), () => removeFile(f))">{{ t('common.delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div style="margin-top: 14px; background: var(--bg-card); padding: 14px; border-radius: var(--radius-lg); border: 1px solid var(--border-glass); font-size: 0.82rem; color: var(--text-muted)">
      <b>{{ t('files.templateHintTitle') }}</b><code>&#123;&#123;uuid&#125;&#125;</code> <code>&#123;&#123;password&#125;&#125;</code> <code>&#123;&#123;token&#125;&#125;</code> <code>&#123;&#123;name&#125;&#125;</code> <code>&#123;&#123;node_list_yaml&#125;&#125;</code> <code>&#123;&#123;node_list_json&#125;&#125;</code> <code>&#123;&#123;outbounds_yaml&#125;&#125;</code> <code>&#123;&#123;outbounds_json&#125;&#125;</code> <code>&#123;&#123;mihomo_proxies_yaml&#125;&#125;</code> —
      {{ t('files.templateHintDesc') }}
    </div>
  </section>

  <!-- 上传分发文件 Modal (UXPilot 风格) -->
  <div class="modal-overlay" :class="{ active: showModal }" @click.self="!isUploading && (showModal = false)">
    <div class="modal upload-modal">
      <!-- Modal Header -->
      <div class="modal-header">
        <div class="header-left">
          <div class="header-icon-box">
            <svg class="header-cloud-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
          </div>
          <div>
            <div class="modal-title">{{ t('files.modal.title') }}</div>
            <div class="modal-subtitle">{{ t('files.modal.subtitle') }}</div>
          </div>
        </div>
        <button type="button" class="modal-close" :disabled="isUploading" @click="showModal = false">&times;</button>
      </div>

      <!-- Step Indicator -->
      <div class="step-indicator">
        <div class="step-item" :class="{ active: currentStep >= 1 }">
          <span class="step-num">1</span>
          <span class="step-label">{{ t('files.modal.step1') }}</span>
        </div>
        <div class="step-line" :class="{ filled: currentStep >= 2 }"></div>
        <div class="step-item" :class="{ active: currentStep >= 2 }">
          <span class="step-num">2</span>
          <span class="step-label">{{ t('files.modal.step2') }}</span>
        </div>
        <div class="step-line" :class="{ filled: currentStep >= 3 }"></div>
        <div class="step-item" :class="{ active: currentStep >= 3 }">
          <span class="step-num">3</span>
          <span class="step-label">{{ t('files.modal.step3') }}</span>
        </div>
      </div>

      <form @submit.prevent="submitUpload" class="upload-flow-form">
        <!-- CHOOSE FILE TYPE -->
        <div class="section-label">{{ t('files.modal.chooseType') }}</div>
        <div class="type-cards-grid">
          <!-- Regular File -->
          <div
            class="type-card"
            :class="{ active: form.type === 'apk' }"
            @click="pickType('apk')"
          >
            <div class="card-icon-box">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path stroke-linecap="round" stroke-linejoin="round" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
              </svg>
            </div>
            <div class="card-title">{{ t('files.modal.typeRegular') }}</div>
            <div class="card-desc">{{ t('files.modal.typeRegularDesc') }}</div>
          </div>

          <!-- ZIP Config -->
          <div
            class="type-card"
            :class="{ active: form.type === 'zip' }"
            @click="pickType('zip')"
          >
            <div class="card-icon-box">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
              </svg>
            </div>
            <div class="card-title">{{ t('files.modal.typeZip') }}</div>
            <div class="card-desc">{{ t('files.modal.typeZipDesc') }}</div>
          </div>

          <!-- Raw Text -->
          <div
            class="type-card"
            :class="{ active: form.type === 'text' }"
            @click="pickType('text')"
          >
            <div class="card-icon-box">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h10" />
              </svg>
            </div>
            <div class="card-title">{{ t('files.modal.typeText') }}</div>
            <div class="card-desc">{{ t('files.modal.typeTextDesc') }}</div>
          </div>
        </div>

        <!-- ADD DETAILS -->
        <div class="section-label" style="margin-top: 20px;">{{ t('files.modal.addDetails') }}</div>
        <div class="details-panel">
          <!-- Regular File (APK) Details -->
          <template v-if="form.type === 'apk'">
            <div class="subtabs-bar">
              <button
                type="button"
                class="subtab-btn"
                :class="{ active: regularSourceTab === 'local' }"
                @click="regularSourceTab = 'local'"
              >
                {{ t('files.modal.tabLocal') }}
              </button>
              <button
                type="button"
                class="subtab-btn"
                :class="{ active: regularSourceTab === 'remote' }"
                @click="regularSourceTab = 'remote'"
              >
                {{ t('files.modal.tabRemote') }}
              </button>
            </div>

            <!-- Local Dropzone -->
            <div v-if="regularSourceTab === 'local'">
              <div
                v-if="!selectedFile"
                class="dropzone"
                :class="{ 'is-dragging': isDragging }"
                @dragover="onDragOver"
                @dragleave="onDragLeave"
                @drop="onDrop"
                @click="fileInput?.click()"
              >
                <div class="dropzone-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                  </svg>
                </div>
                <div class="dropzone-text">
                  {{ t('files.modal.dropzoneText') }}
                </div>
                <div class="dropzone-sub">
                  {{ t('files.modal.dropzoneSub') }}
                </div>
                <input
                  ref="fileInput"
                  type="file"
                  class="hidden-file-input"
                  @change="onFileInputChange"
                />
              </div>

              <!-- Selected File Preview Card -->
              <div v-else class="file-preview-card">
                <div class="file-preview-left">
                  <div class="file-badge-icon">📄</div>
                  <div class="file-info-col">
                    <div class="file-name" :title="selectedFile.name">{{ selectedFile.name }}</div>
                    <div class="file-size">{{ formatFileSize(selectedFile.size) }}</div>
                  </div>
                </div>
                <div class="file-preview-right">
                  <span class="ready-badge">{{ t('files.modal.readyBadge') }}</span>
                  <button type="button" class="btn-remove-file" :title="t('files.modal.removeFile')" @click="clearSelectedFile">&times;</button>
                </div>
              </div>
            </div>

            <!-- Remote URL input -->
            <div v-else class="form-group" style="margin-top: 12px;">
              <label>{{ t('files.modal.remoteUrlLabel') }}</label>
              <input
                v-model="form.sourceUrl"
                type="url"
                class="form-control"
                :placeholder="t('files.modal.remoteUrlPlaceholder')"
              />
              <div class="input-hint">{{ t('files.modal.remoteUrlHint') }}</div>
            </div>
          </template>

          <!-- ZIP Config Details -->
          <template v-else-if="form.type === 'zip'">
            <div
              v-if="!selectedFile"
              class="dropzone"
              :class="{ 'is-dragging': isDragging }"
              @dragover="onDragOver"
              @dragleave="onDragLeave"
              @drop="onDrop"
              @click="fileInput?.click()"
            >
              <div class="dropzone-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                </svg>
              </div>
              <div class="dropzone-text">
                {{ t('files.modal.dropzoneText') }}
              </div>
              <div class="dropzone-sub">
                {{ t('files.modal.dropzoneZipSub') }}
              </div>
              <input
                ref="fileInput"
                type="file"
                accept=".zip"
                class="hidden-file-input"
                @change="onFileInputChange"
              />
            </div>

            <div v-else class="file-preview-card">
              <div class="file-preview-left">
                <div class="file-badge-icon">📦</div>
                <div class="file-info-col">
                  <div class="file-name" :title="selectedFile.name">{{ selectedFile.name }}</div>
                  <div class="file-size">{{ formatFileSize(selectedFile.size) }}</div>
                </div>
              </div>
              <div class="file-preview-right">
                <span class="ready-badge">{{ t('files.modal.readyBadge') }}</span>
                <button type="button" class="btn-remove-file" :title="t('files.modal.removeFile')" @click="clearSelectedFile">&times;</button>
              </div>
            </div>

            <div class="form-group" style="margin-top: 14px;">
              <label>{{ t('files.modal.templateNameLabel') }}</label>
              <input
                v-model="form.templateName"
                class="form-control"
                :placeholder="t('files.modal.templateNamePlaceholder')"
              />
            </div>
          </template>

          <!-- Raw Text Details -->
          <template v-else>
            <div class="form-group">
              <label>{{ t('files.modal.rawTextLabel') }}</label>
              <textarea
                v-model="form.contentText"
                class="form-control"
                rows="6"
                style="font-family: var(--font-mono); font-size: 0.8rem"
                :placeholder="t('files.modal.rawTextPlaceholder')"
              ></textarea>
            </div>
          </template>

          <!-- Divider -->
          <div class="panel-divider"></div>

          <!-- Target Filename & Remarks (2 Column Grid) -->
          <div class="details-bottom-grid">
            <div class="form-group">
              <label>{{ t('files.modal.targetFilename') }}</label>
              <input
                v-model="form.downloadName"
                class="form-control"
                :placeholder="t('files.modal.targetFilenamePlaceholder')"
              />
              <div class="input-hint">{{ t('files.modal.targetFilenameHint') }}</div>
            </div>

            <div class="form-group">
              <label>{{ t('files.modal.remarks') }}</label>
              <input
                v-model="form.remark"
                class="form-control"
                :placeholder="t('files.modal.remarksPlaceholder')"
              />
            </div>
          </div>

          <!-- Duplicate Warning Badge -->
          <div v-if="duplicateWarning" class="duplicate-warning">
            <span class="warn-icon">⚠️</span>
            <span>{{ t('files.modal.duplicateWarning', { name: duplicateWarning.name }) }}</span>
          </div>

          <!-- Upload Progress Bar -->
          <div v-if="isUploading && totalBytes > 0" class="upload-progress-container">
            <div class="progress-info-row">
              <span class="progress-status-text">{{ t('files.modal.uploading') }}</span>
              <span class="progress-percent-text">{{ uploadProgress }}%</span>
            </div>
            <div class="progress-bar-track">
              <div class="progress-bar-fill" :style="{ width: `${uploadProgress}%` }"></div>
            </div>
            <div class="progress-bytes-row">
              {{ formatFileSize(uploadedBytes) }} / {{ formatFileSize(totalBytes) }}
            </div>
          </div>
        </div>

        <!-- Footer Actions -->
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary btn-cancel" :disabled="isUploading" @click="showModal = false">
            {{ t('files.modal.cancelBtn') }}
          </button>
          <button type="submit" class="btn btn-primary btn-distribute" :disabled="isUploading">
            <span v-if="isUploading" class="btn-spinner"></span>
            <svg v-else class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
            </svg>
            <span>{{ isUploading ? t('files.modal.distributingBtn', { pct: uploadProgress }) : t('files.modal.distributeBtn') }}</span>
          </button>
        </div>
      </form>
    </div>
  </div>

  <!-- 文本文件编辑 Modal -->
  <div class="modal-overlay" :class="{ active: showTextModal }" @click.self="showTextModal = false">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">{{ t('files.textModal.title') }}{{ textEdit ? `：${textEdit.download_name || textEdit.original_name || textEdit.name}` : '' }}</div>
        <button class="modal-close" @click="showTextModal = false">&times;</button>
      </div>
      <form @submit.prevent="saveTextEdit" class="modal-form">
        <div class="form-group form-span">
          <label>{{ t('files.textModal.fileName') }}</label>
          <input v-model="textForm.downloadName" class="form-control" :placeholder="t('files.textModal.fileNamePlaceholder')" />
        </div>
        <div class="form-group form-span">
          <label>{{ t('files.textModal.content') }}</label>
          <textarea v-model="textForm.content" class="form-control" rows="10" style="font-family: var(--font-mono); font-size: 0.78rem"
            :placeholder="t('common.loading')" :disabled="textLoading"></textarea>
        </div>
        <div class="form-group form-span">
          <label>{{ t('files.textModal.remark') }}</label>
          <input v-model="textForm.remark" class="form-control" :placeholder="t('files.textModal.remarkPlaceholder')" />
        </div>
        <div class="modal-footer" style="justify-content: flex-end; gap: 8px;">
          <button type="button" class="btn btn-secondary" @click="showTextModal = false">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="textLoading">{{ t('common.save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.shared-token-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  margin-bottom: 16px;
}

.shared-token-value {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--primary);
  background: var(--bg-input);
  padding: 3px 8px;
  border-radius: 6px;
}

/* UXPilot Upload Modal Styling */
.upload-modal {
  max-width: 660px !important;
  width: min(94vw, 660px) !important;
  padding: 26px 30px !important;
  border-radius: 20px !important;
  border: 1px solid var(--border-glass) !important;
  background: rgba(15, 23, 42, 0.96) !important;
  box-shadow: 0 25px 60px -15px rgba(0, 0, 0, 0.7), 0 0 35px rgba(59, 130, 246, 0.08) !important;
  box-sizing: border-box !important;
}

.upload-flow-form {
  display: flex;
  flex-direction: column;
  width: 100%;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.header-icon-box {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: rgba(59, 130, 246, 0.15);
  border: 1px solid rgba(59, 130, 246, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--primary);
  flex-shrink: 0;
}

.header-cloud-icon {
  width: 22px;
  height: 22px;
}

.modal-subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-top: 3px;
}

/* Step Indicator */
.step-indicator {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 18px 0 22px;
  padding: 0 6px;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.82rem;
  color: var(--text-dim);
}

.step-item.active {
  color: var(--text-main);
  font-weight: 600;
}

.step-num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  color: var(--text-dim);
  font-weight: 600;
}

.step-item.active .step-num {
  background: var(--primary);
  color: #fff;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.5);
}

.step-line {
  flex: 1;
  height: 2px;
  background: rgba(255, 255, 255, 0.08);
  margin: 0 12px;
  border-radius: 2px;
}

.step-line.filled {
  background: linear-gradient(90deg, var(--primary), rgba(59, 130, 246, 0.4));
}

.section-label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-dim);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 10px;
}

/* Type Cards Grid */
.type-cards-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}

.type-card {
  background: rgba(15, 23, 42, 0.6);
  border: 1.5px solid var(--border-glass);
  border-radius: 14px;
  padding: 16px 12px;
  text-align: center;
  cursor: pointer;
  transition: all 0.22s ease;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.type-card:hover {
  border-color: rgba(59, 130, 246, 0.4);
  background: rgba(30, 41, 59, 0.6);
}

.type-card.active {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.1);
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.15);
}

.card-icon-box {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
}

.card-icon-box svg {
  width: 20px;
  height: 20px;
}

.type-card.active .card-icon-box {
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
}

.card-title {
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--text-main);
}

.card-desc {
  font-size: 0.72rem;
  color: var(--text-muted);
}

/* Details Panel */
.details-panel {
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid var(--border-glass);
  border-radius: 16px;
  padding: 18px 20px;
  margin-bottom: 20px;
}

.subtabs-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}

.subtab-btn {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-muted);
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.subtab-btn.active {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.12);
  color: #fff;
}

/* Dropzone */
.dropzone {
  border: 1.5px dashed rgba(255, 255, 255, 0.14);
  border-radius: 14px;
  padding: 30px 20px;
  text-align: center;
  cursor: pointer;
  transition: all 0.22s ease;
  background: rgba(15, 23, 42, 0.4);
}

.dropzone:hover,
.dropzone.is-dragging {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.06);
  transform: scale(1.003);
}

.dropzone-icon {
  width: 42px;
  height: 42px;
  margin: 0 auto 10px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
}

.dropzone-icon svg {
  width: 32px;
  height: 32px;
}

.dropzone.is-dragging .dropzone-icon {
  color: #60a5fa;
  transform: translateY(-2px);
}

.dropzone-text {
  font-size: 0.92rem;
  color: var(--text-main);
  font-weight: 500;
}

.browse-link {
  color: var(--primary);
  font-weight: 600;
  text-decoration: underline;
}

.dropzone-sub {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-top: 6px;
}

.hidden-file-input {
  display: none;
}

/* File Preview Card */
.file-preview-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: 12px;
  padding: 12px 16px;
}

.file-preview-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.file-badge-icon {
  font-size: 1.5rem;
}

.file-info-col {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.file-name {
  font-weight: 600;
  font-size: 0.88rem;
  color: var(--text-main);
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.file-preview-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ready-badge {
  font-size: 0.72rem;
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 2px 8px;
  border-radius: 6px;
  font-weight: 600;
}

.btn-remove-file {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 1.2rem;
  cursor: pointer;
  padding: 0 4px;
}

.btn-remove-file:hover {
  color: var(--accent-rose);
}

.panel-divider {
  height: 1px;
  background: rgba(255, 255, 255, 0.06);
  margin: 18px 0;
}

.details-bottom-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.details-bottom-grid .form-group {
  margin-bottom: 0;
}

.input-hint {
  font-size: 0.7rem;
  color: var(--text-dim);
  margin-top: 4px;
  letter-spacing: 0.02em;
}

.duplicate-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.25);
  color: #fbbf24;
  font-size: 0.78rem;
  padding: 8px 12px;
  border-radius: 8px;
  margin-top: 14px;
}

/* Upload Progress Container */
.upload-progress-container {
  margin-top: 16px;
  background: rgba(15, 23, 42, 0.85);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: 10px;
  padding: 12px 14px;
}

.progress-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.82rem;
}

.progress-status-text {
  color: var(--text-muted);
}

.progress-percent-text {
  color: var(--primary);
  font-weight: 700;
  font-family: var(--font-mono);
}

.progress-bar-track {
  height: 6px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
  margin: 8px 0;
}

.progress-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--primary), var(--accent-cyan));
  border-radius: 9999px;
  transition: width 0.2s ease;
}

.progress-bytes-row {
  font-size: 0.72rem;
  color: var(--text-dim);
  font-family: var(--font-mono);
  text-align: right;
}

/* Modal Footer */
.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 10px;
}

.btn-cancel {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-weight: 500;
  cursor: pointer;
  padding: 8px 14px;
}

.btn-cancel:hover {
  color: var(--text-main);
}

.btn-distribute {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: #2563eb;
  color: #fff;
  border: none;
  padding: 10px 22px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 0.92rem;
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.4);
  transition: all 0.2s ease;
}

.btn-distribute:hover:not(:disabled) {
  background: #1d4ed8;
  transform: translateY(-1px);
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.5);
}

.btn-distribute:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.btn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

@media (max-width: 580px) {
  .upload-modal {
    padding: 20px 16px !important;
  }
  .type-cards-grid {
    grid-template-columns: 1fr;
  }
  .details-bottom-grid {
    grid-template-columns: 1fr;
  }
}
</style>
