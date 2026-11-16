<script setup lang="ts">
import { inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile } from '../types'
import { useFuzzySearch } from '../composables'
import { ToastKey, PopoverKey } from '../injectionKeys'
import SearchInput from '../components/SearchInput.vue'
import SharedTokenBar from '../components/SharedTokenBar.vue'
import FileTable from '../components/FileTable.vue'
import FileUploadModal from '../components/FileUploadModal.vue'
import FileTextEditModal from '../components/FileTextEditModal.vue'

const { t } = useI18n()
const toast = inject(ToastKey)
const popover = inject(PopoverKey)

const files = ref<DistFile[]>([])
const sharedToken = ref('')

const { searchQuery, filteredItems: filteredFiles, clear: clearSearch } = useFuzzySearch(files, (f) => [
  f.download_name,
  f.original_name,
  f.name,
  f.remark,
  f.file_type,
  f.source_url,
  f.template_name,
  f.is_active ? t('files.statusActive') : t('files.statusInactive'),
  String(f.id),
])

const showModal = ref(false)
const showTextModal = ref(false)
const textEdit = ref<DistFile | null>(null)

async function fetchData() {
  try {
    const [f, s] = await Promise.all([api.files.list(), api.settings.sharedToken()])
    files.value = f
    sharedToken.value = s.token
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

function openCreate() {
  showModal.value = true
}

async function toggleActive(f: DistFile) {
  try {
    await api.files.update(f.id, { is_active: !f.is_active })
    toast?.(f.is_active ? t('files.statusInactive') : t('files.statusActive'), 'info')
    await fetchData()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

async function refreshRemote(f: DistFile, btn: HTMLButtonElement) {
  btn.disabled = true
  const originalText = btn.textContent
  btn.textContent = t('files.refreshing')
  try {
    await api.files.refresh(f.id)
    toast?.(t('files.successRefreshed'), 'info')
    await fetchData()
  } catch (e) {
    toast?.((e as Error).message, 'error')
    btn.disabled = false
    btn.textContent = originalText
  }
}

async function removeFile(f: DistFile) {
  try {
    await api.files.remove(f.id)
    toast?.(t('common.deleted'), 'info')
    await fetchData()
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}

function openTextEdit(f: DistFile) {
  textEdit.value = f
  showTextModal.value = true
}

function deleteFile(el: Element, f: DistFile) {
  popover?.show(el, t('files.deleteConfirm'), () => removeFile(f))
}

function copySharedToken() {
  navigator.clipboard.writeText(sharedToken.value).then(
    () => toast?.(t('common.copied'), 'info'),
    () => toast?.(t('common.copyFailed'), 'error'),
  )
}

function resetSharedToken(btn: Element) {
  popover?.show(btn, t('files.resetSharedConfirm'), async () => {
    try {
      const r = await api.settings.resetSharedToken()
      sharedToken.value = r.token
      toast?.(t('common.success'), 'info')
    } catch (e) {
      toast?.((e as Error).message, 'error')
    }
  })
}

onMounted(fetchData)
</script>

<template>
  <section class="tab-content files-tab">
    <div class="section-header">
      <div class="section-title">{{ t('files.headerTitle') }}</div>
      <div class="section-actions">
        <SearchInput
          v-model="searchQuery"
          :placeholder="t('files.searchPlaceholder')"
          :total-count="files.length"
          :filtered-count="filteredFiles.length"
          :disabled="!files.length"
        />
        <button class="btn btn-primary" @click="openCreate"><span>+</span> {{ t('files.uploadBtn') }}</button>
      </div>
    </div>

    <SharedTokenBar
      :shared-token="sharedToken"
      @copy="copySharedToken"
      @reset="resetSharedToken"
    />

    <div class="table-container">
      <FileTable
        :files="filteredFiles"
        :shared-token="sharedToken"
        @toggle-active="toggleActive"
        @refresh-remote="refreshRemote"
        @open-text-edit="openTextEdit"
        @delete-file="deleteFile"
      />
      <div v-if="files.length && !filteredFiles.length" class="no-match">
        <div class="empty-icon">🔍</div>
        <div class="empty-text">{{ t('files.noMatchingFiles') }}</div>
        <button class="btn btn-secondary btn-sm" @click="clearSearch">{{ t('common.clearFilter') }}</button>
      </div>
    </div>

    <div class="template-hint">
      <b>{{ t('files.templateHintTitle') }}</b>
      <code>&#123;&#123;uuid&#125;&#125;</code>
      <code>&#123;&#123;password&#125;&#125;</code>
      <code>&#123;&#123;token&#125;&#125;</code>
      <code>&#123;&#123;name&#125;&#125;</code>
      <code>&#123;&#123;node_list_yaml&#125;&#125;</code>
      <code>&#123;&#123;node_list_json&#125;&#125;</code>
      <code>&#123;&#123;outbounds_yaml&#125;&#125;</code>
      <code>&#123;&#123;outbounds_json&#125;&#125;</code>
      <code>&#123;&#123;mihomo_proxies_yaml&#125;&#125;</code>
      — {{ t('files.templateHintDesc') }}
    </div>
  </section>

  <FileUploadModal :open="showModal" @update:open="(val) => (showModal = val)" @uploaded="fetchData" />

  <FileTextEditModal :open="showTextModal" :edit-file="textEdit" @update:open="(val) => (showTextModal = val)" @saved="fetchData" />
</template>

<style scoped>
.files-tab {
  display: block;
}
.section-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}
.table-container {
  overflow: auto;
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
}
.no-match {
  text-align: center;
  color: var(--text-muted);
  padding: 36px;
}
.empty-icon {
  font-size: 1.8rem;
  margin-bottom: 8px;
}
.empty-text {
  margin-bottom: 12px;
}
.template-hint {
  margin-top: 14px;
  background: var(--bg-card);
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-glass);
  font-size: 0.82rem;
  color: var(--text-muted);
}
</style>