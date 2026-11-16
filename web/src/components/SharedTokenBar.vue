<script setup lang="ts">
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { ToastKey } from '../injectionKeys'

const props = defineProps<{
  sharedToken: string
}>()

const emit = defineEmits<{
  (e: 'copy'): void
  (e: 'reset', btn: Element): void
}>()

const { t } = useI18n()
const toast = inject(ToastKey)

async function copySharedToken() {
  try {
    await navigator.clipboard.writeText(props.sharedToken)
    toast?.(t('common.copied'), 'info')
    emit('copy')
  } catch {
    toast?.(t('common.copyFailed'), 'error')
  }
}
</script>

<template>
  <div class="shared-token-bar">
    <span class="shared-token-title">{{ t('files.sharedTokenBar') }}</span>
    <span class="shared-token-tip">{{ t('files.sharedTokenTip') }}</span>
    <code class="shared-token-value">{{ sharedToken }}</code>
    <button class="btn btn-secondary btn-sm" @click="copySharedToken">{{ t('files.copySharedToken') }}</button>
    <button class="btn btn-danger btn-sm" @click="emit('reset', $event.currentTarget as Element)">{{ t('files.resetSharedToken') }}</button>
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

.shared-token-title {
  font-weight: 600;
}
.shared-token-tip {
  color: var(--text-muted);
  font-size: 0.8rem;
}
.shared-token-value {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--primary);
  background: var(--bg-input);
  padding: 3px 8px;
  border-radius: 6px;
}
</style>