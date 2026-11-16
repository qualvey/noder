<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { User, UserDownload } from '../types'

defineProps<{
  user: User | null
  history: UserDownload[]
  loading: boolean
  formatTime: (value?: string | null) => string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="user" class="modal-overlay active" @mousedown.self="emit('close')">
    <div class="modal download-history-modal">
      <div class="modal-header">
        <div class="modal-title">{{ t('users.downloadHistoryTitle', { name: user.name }) }}</div>
        <button class="modal-close" @click="emit('close')">&times;</button>
      </div>
      <div class="download-history-content">
        <div v-if="loading" class="download-history-empty">{{ t('common.loading') }}</div>
        <div v-else-if="!history.length" class="download-history-empty">{{ t('users.noDownloads') }}</div>
        <div v-else class="download-history-list">
          <div v-for="item in history" :key="item.id" class="download-history-item">
            <span class="download-history-name">{{ item.file_name }}</span>
            <time>{{ formatTime(item.downloaded_at) }}</time>
          </div>
        </div>
        <div v-if="history.length === 100" class="download-history-hint">{{ t('users.historyLimit') }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.download-history-modal { max-width: 620px; }
.download-history-content { padding: 16px 20px 20px; max-height: 65vh; overflow-y: auto; }
.download-history-list { display: flex; flex-direction: column; gap: 8px; }
.download-history-item { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 10px 12px; border: 1px solid var(--border-glass); border-radius: var(--radius-sm); }
.download-history-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.download-history-item time { flex-shrink: 0; color: var(--text-muted); font-size: 0.78rem; }
.download-history-empty, .download-history-hint { color: var(--text-muted); text-align: center; padding: 18px; font-size: 0.85rem; }
.download-history-hint { padding-bottom: 0; font-size: 0.75rem; }
</style>
