<script setup lang="ts">
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DistFile } from '../types'
import { formatFileSize, apiLinkPrefix } from '../utils'
import { ToastKey } from '../injectionKeys'

defineProps<{
  files: DistFile[]
  sharedToken: string
}>()

const emit = defineEmits<{
  (e: 'toggle-active', f: DistFile): void
  (e: 'refresh-remote', f: DistFile, btn: HTMLButtonElement): void
  (e: 'open-text-edit', f: DistFile): void
  (e: 'delete-file', el: Element, f: DistFile): void
}>()

const { t } = useI18n()
const toast = inject(ToastKey)

function copySharedLink(f: DistFile, sharedToken: string) {
  if (!sharedToken) return
  const url = `${location.origin}${apiLinkPrefix()}/dl/${f.id}?token=${encodeURIComponent(sharedToken)}`
  navigator.clipboard.writeText(url).then(
    () => toast?.(t('common.copied'), 'info'),
    () => toast?.(t('common.copyFailed'), 'error'),
  )
}
</script>

<template>
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
        <td colspan="9" class="empty-cell">
          <div class="empty-icon">📁</div>
          <div>{{ t('files.emptyText') }}</div>
        </td>
      </tr>
      <tr v-for="f in files" :key="f.id">
        <td>{{ f.id }}</td>
        <td>
          <code>{{ f.download_name || f.original_name || f.name }}</code>
          <div v-if="f.remark" class="remark-text">{{ f.remark }}</div>
        </td>
        <td>
          <span class="badge" :class="f.file_type === 'zip' ? 'badge-vless' : f.file_type === 'text' ? 'badge-anytls' : 'badge-tuic'">
            {{ f.file_type === 'apk' ? t('files.typeRegularShort') : f.file_type.toUpperCase() }}
          </span>
        </td>
        <td>{{ formatFileSize(f.size) }}</td>
        <td>
          <span v-if="f.source_url" :title="f.source_url" class="source-remote">🔗 {{ t('files.sourceRemote') }}</span>
          <span v-else-if="f.file_type === 'text'" class="muted-text">📝 {{ t('files.sourceText') }}</span>
          <span v-else class="muted-text">📁 {{ t('files.sourceLocal') }}</span>
        </td>
        <td>{{ f.file_type === 'zip' ? f.template_name || '-' : '-' }}</td>
        <td>
          <span v-if="f.is_active" class="status-active">{{ t('files.statusActive') }}</span>
          <span v-else class="status-inactive">{{ t('files.statusInactive') }}</span>
        </td>
        <td>
          <template v-if="f.file_type === 'zip'">
            <span class="zip-hint">{{ t('files.zipPersonalHint') }}</span>
          </template>
          <template v-else>
            <button class="btn btn-secondary btn-sm" @click="copySharedLink(f, sharedToken)">{{ t('files.copySharedLink') }}</button>
          </template>
        </td>
        <td>
          <div class="row-actions">
            <button v-if="f.file_type === 'text'" class="btn btn-secondary btn-sm" @click="emit('open-text-edit', f)">{{ t('common.edit') }}</button>
            <button v-if="f.source_url" class="btn btn-secondary btn-sm" @click="emit('refresh-remote', f, $event.currentTarget as HTMLButtonElement)">{{ t('files.refreshRemote') }}</button>
            <button class="btn btn-secondary btn-sm" @click="emit('toggle-active', f)">{{ f.is_active ? t('files.toggleInactive') : t('files.toggleActive') }}</button>
            <button class="btn btn-danger btn-sm" @click="emit('delete-file', $event.currentTarget as Element, f)">{{ t('common.delete') }}</button>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
</template>

<style scoped>
.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.data-table th {
  padding: 10px 12px;
  text-align: left;
  color: var(--text-muted);
  font-weight: 600;
  border-bottom: 1px solid var(--border-glass);
  white-space: nowrap;
}

.data-table td {
  padding: 10px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.badge {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 500;
}
.empty-cell {
  text-align: center;
  color: var(--text-muted);
  padding: 30px;
}
.empty-icon {
  font-size: 1.8rem;
  margin-bottom: 8px;
}
.remark-text {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-top: 2px;
}
.source-remote {
  cursor: help;
}
.muted-text {
  color: var(--text-muted);
}
.status-active {
  color: var(--accent-emerald);
}
.status-inactive {
  color: var(--accent-rose);
}
.zip-hint {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.row-actions {
  display: flex;
  gap: 6px;
}
</style>
