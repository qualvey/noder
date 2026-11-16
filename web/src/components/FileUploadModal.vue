<script setup lang="ts">
import { inject, ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import FileDropzone from './FileDropzone.vue'
import UploadStepIndicator from './UploadStepIndicator.vue'
import UploadProgressBar from './UploadProgressBar.vue'
import UploadTypeCards, { type UploadFileType } from './UploadTypeCards.vue'
import { ToastKey } from '../injectionKeys'

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'uploaded'): void
}>()

const { t } = useI18n()
const toast = inject(ToastKey)


const form = ref({
  type: 'apk' as UploadFileType,
  templateName: '',
  downloadName: '',
  remark: '',
  sourceUrl: '',
  contentText: '',
})

const selectedFile = ref<File | null>(null)
const regularSourceTab = ref<'local' | 'remote'>('local')
const dropzones = ref<InstanceType<typeof FileDropzone>[]>([])

const isUploading = ref(false)
const uploadProgress = ref(0)
const uploadedBytes = ref(0)
const totalBytes = ref(0)

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

function pickType(t: UploadFileType) {
  form.value.type = t
  selectedFile.value = null
  dropzones.value.forEach((d) => d?.clear())
}

function handleFileSelected(file: File) {
  if (form.value.type === 'zip' && !file.name.toLowerCase().endsWith('.zip')) {
    toast?.(t('files.modal.selectZipFirst'), 'error')
    return
  }
  selectedFile.value = file
  if (!form.value.downloadName.trim()) {
    form.value.downloadName = file.name
  }
}

function clearSelectedFile() {
  selectedFile.value = null
  dropzones.value.forEach((d) => d?.clear())
}

async function submitUpload() {
  if (isUploading.value) return

  const isText = form.value.type === 'text'
  const isZip = form.value.type === 'zip'
  const isRegular = form.value.type === 'apk'

  const hasFile = !!selectedFile.value
  const hasUrl = regularSourceTab.value === 'remote' && form.value.sourceUrl.trim().length > 0
  const hasContent = form.value.contentText.trim().length > 0

  if (isText) {
    if (!hasContent) {
      toast?.(t('files.modal.fillContentFirst'), 'error')
      return
    }
  } else if (isZip) {
    if (!hasFile) {
      toast?.(t('files.modal.selectZipFirst'), 'error')
      return
    }
  } else if (isRegular) {
    if (regularSourceTab.value === 'local' && !hasFile) {
      toast?.(t('files.modal.selectFileFirst'), 'error')
      return
    }
    if (regularSourceTab.value === 'remote' && !hasUrl) {
      toast?.(t('files.modal.fillUrlFirst'), 'error')
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

    toast?.(hasUrl ? t('files.successRemoteAdded') : hasContent ? t('files.successTextSaved') : t('files.successUploaded'), 'info')
    emit('update:open', false)
    emit('uploaded')
  } catch (e) {
    toast?.((e as Error).message, 'error')
  } finally {
    isUploading.value = false
  }
}
</script>

<template>
  <div class="modal-overlay" :class="{ active: open }" @mousedown.self="!isUploading && (emit('update:open', false))">
    <div class="modal upload-modal">
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
        <button type="button" class="modal-close" :disabled="isUploading" @click="emit('update:open', false)">&times;</button>
      </div>

      <UploadStepIndicator :current-step="currentStep" />

      <form @submit.prevent="submitUpload" class="upload-flow-form">
        <UploadTypeCards :current="form.type" @pick="pickType" />

        <div class="section-label section-label-spaced">{{ t('files.modal.addDetails') }}</div>
        <div class="details-panel">
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

            <div v-if="regularSourceTab === 'local'">
              <FileDropzone
                ref="dropzones"
                badge-icon="📄"
                :drop-text="t('files.modal.dropzoneText')"
                :drop-sub="t('files.modal.dropzoneSub')"
                :selected-file="selectedFile"
                :remove-title="t('files.modal.removeFile')"
                :ready-text="t('files.modal.readyBadge')"
                @file-selected="handleFileSelected"
                @clear="clearSelectedFile"
              />
            </div>

            <div v-else class="form-group remote-url-group">
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

          <template v-else-if="form.type === 'zip'">
            <FileDropzone
              ref="dropzones"
              accept=".zip"
              badge-icon="📦"
              :drop-text="t('files.modal.dropzoneText')"
              :drop-sub="t('files.modal.dropzoneZipSub')"
              :selected-file="selectedFile"
              :remove-title="t('files.modal.removeFile')"
              :ready-text="t('files.modal.readyBadge')"
              @file-selected="handleFileSelected"
              @clear="clearSelectedFile"
            />

            <div class="form-group template-name-group">
              <label>{{ t('files.modal.templateNameLabel') }}</label>
              <input
                v-model="form.templateName"
                class="form-control"
                :placeholder="t('files.modal.templateNamePlaceholder')"
              />
            </div>
          </template>

          <template v-else>
            <div class="form-group">
              <label>{{ t('files.modal.rawTextLabel') }}</label>
              <textarea
                v-model="form.contentText"
                class="form-control code-textarea"
                rows="6"
                :placeholder="t('files.modal.rawTextPlaceholder')"
              ></textarea>
            </div>
          </template>

          <div class="panel-divider"></div>

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

          <UploadProgressBar
            v-if="isUploading && totalBytes > 0"
            :percent="uploadProgress"
            :uploaded-bytes="uploadedBytes"
            :total-bytes="totalBytes"
          />
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary btn-cancel" :disabled="isUploading" @click="emit('update:open', false)">
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
</template>

<style scoped>
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

.section-label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-dim);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 10px;
}

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

.input-hint {
  font-size: 0.7rem;
  color: var(--text-dim);
  margin-top: 4px;
  letter-spacing: 0.02em;
}

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
  .details-bottom-grid {
    grid-template-columns: 1fr;
  }
}

.section-label-spaced {
  margin-top: 20px;
}
.remote-url-group {
  margin-top: 12px;
}
.template-name-group {
  margin-top: 14px;
}
.code-textarea {
  font-family: var(--font-mono);
  font-size: 0.8rem;
}
</style>