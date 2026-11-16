<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatFileSize } from '../utils'

defineProps<{
  percent: number
  uploadedBytes: number
  totalBytes: number
}>()

const { t } = useI18n()
</script>

<template>
  <div class="upload-progress-container">
    <div class="progress-info-row">
      <span class="progress-status-text">{{ t('files.modal.uploading') }}</span>
      <span class="progress-percent-text">{{ percent }}%</span>
    </div>
    <div class="progress-bar-track">
      <div class="progress-bar-fill" :style="{ width: `${percent}%` }"></div>
    </div>
    <div class="progress-bytes-row">
      {{ formatFileSize(uploadedBytes) }} / {{ formatFileSize(totalBytes) }}
    </div>
  </div>
</template>

<style scoped>
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
</style>
