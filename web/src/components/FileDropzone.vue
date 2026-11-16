<script setup lang="ts">
import { ref } from 'vue'
import { formatFileSize } from '../utils'

defineProps<{
  accept?: string
  badgeIcon: string
  dropText: string
  dropSub: string
  selectedFile: File | null
  removeTitle: string
  readyText: string
}>()

const emit = defineEmits<{
  (e: 'file-selected', file: File): void
  (e: 'clear'): void
}>()

const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

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
  const dropped = e.dataTransfer?.files
  if (dropped && dropped.length > 0 && dropped[0]) {
    emit('file-selected', dropped[0])
  }
}

function onInputChange(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (files && files.length > 0 && files[0]) {
    emit('file-selected', files[0])
  }
}

function openPicker() {
  fileInput.value?.click()
}

function clear() {
  if (fileInput.value) fileInput.value.value = ''
  emit('clear')
}

defineExpose({ openPicker, clear })
</script>

<template>
  <div
    v-if="!selectedFile"
    class="dropzone"
    :class="{ 'is-dragging': isDragging }"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
    @click="openPicker"
  >
    <div class="dropzone-icon">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
        <path stroke-linecap="round" stroke-linejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
      </svg>
    </div>
    <div class="dropzone-text">{{ dropText }}</div>
    <div class="dropzone-sub">{{ dropSub }}</div>
    <input
      ref="fileInput"
      type="file"
      :accept="accept"
      class="hidden-file-input"
      @change="onInputChange"
    />
  </div>

  <div v-else class="file-preview-card">
    <div class="file-preview-left">
      <div class="file-badge-icon">{{ badgeIcon }}</div>
      <div class="file-info-col">
        <div class="file-name" :title="selectedFile.name">{{ selectedFile.name }}</div>
        <div class="file-size">{{ formatFileSize(selectedFile.size) }}</div>
      </div>
    </div>
    <div class="file-preview-right">
      <span class="ready-badge">{{ readyText }}</span>
      <button type="button" class="btn-remove-file" :title="removeTitle" @click="clear">&times;</button>
    </div>
  </div>
</template>

<style scoped>
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
.dropzone-sub {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-top: 6px;
}
.hidden-file-input {
  display: none;
}
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
</style>
