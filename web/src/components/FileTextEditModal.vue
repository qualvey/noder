<script setup lang="ts">
import { inject, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile } from '../types'
import { ToastKey } from '../injectionKeys'

const props = defineProps<{
  open: boolean
  editFile: DistFile | null
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const toast = inject(ToastKey)

const textForm = ref({ downloadName: '', remark: '', content: '' })
const textLoading = ref(false)

watch(
  () => [props.open, props.editFile] as const,
  async ([open, file]) => {
    if (!open || !file) return
    textLoading.value = true
    try {
      const r = await api.files.getContent(file.id)
      textForm.value = {
        downloadName: file.download_name || file.original_name || file.name,
        remark: file.remark || '',
        content: r.content,
      }
    } catch (e) {
      toast?.((e as Error).message, 'error')
      emit('update:open', false)
    } finally {
      textLoading.value = false
    }
  },
  { immediate: true },
)

async function saveTextEdit() {
  if (!props.editFile) return
  try {
    await api.files.updateContent(props.editFile.id, textForm.value.content)
    const dlName = textForm.value.downloadName.trim()
    await api.files.update(props.editFile.id, {
      name: dlName || props.editFile.name,
      download_name: dlName || null,
      remark: textForm.value.remark.trim() || null,
    })
    toast?.(t('files.successTextSaved'), 'info')
    emit('update:open', false)
    emit('saved')
  } catch (e) {
    toast?.((e as Error).message, 'error')
  }
}
</script>

<template>
  <div class="modal-overlay" :class="{ active: open }" @mousedown.self="emit('update:open', false)">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">{{ t('files.textModal.title') }}{{ props.editFile ? `：${props.editFile.download_name || props.editFile.original_name || props.editFile.name}` : '' }}</div>
        <button class="modal-close" @click="emit('update:open', false)">&times;</button>
      </div>
      <form @submit.prevent="saveTextEdit" class="modal-form">
        <div class="form-group form-span">
          <label>{{ t('files.textModal.fileName') }}</label>
          <input v-model="textForm.downloadName" class="form-control" :placeholder="t('files.textModal.fileNamePlaceholder')" />
        </div>
        <div class="form-group form-span">
          <label>{{ t('files.textModal.content') }}</label>
          <textarea v-model="textForm.content" class="form-control code-textarea" rows="10"
            :placeholder="t('common.loading')" :disabled="textLoading"></textarea>
        </div>
        <div class="form-group form-span">
          <label>{{ t('files.textModal.remark') }}</label>
          <input v-model="textForm.remark" class="form-control" :placeholder="t('files.textModal.remarkPlaceholder')" />
        </div>
        <div class="modal-footer modal-footer-end">
          <button type="button" class="btn btn-secondary" @click="emit('update:open', false)">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="textLoading">{{ t('common.save') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.code-textarea {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}
.modal-footer-end {
  justify-content: flex-end;
  gap: 8px;
}
</style>