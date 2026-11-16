<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  open: boolean
  token: string
  showPassword: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'update:token', val: string): void
  (e: 'toggle-visibility'): void
  (e: 'save'): void
}>()

const { t } = useI18n()
</script>

<template>
  <div
    v-if="open"
    class="mobile-token-modal-mask"
    @mousedown.self="emit('update:open', false)"
  >
    <div class="mobile-token-modal-card">
      <div class="mobile-token-modal-header">
        <h4>{{ t('nav.adminTokenTitle') }}</h4>
        <button type="button" class="btn-close" @click="emit('update:open', false)">✕</button>
      </div>
      <p class="mobile-token-modal-tip">{{ t('nav.adminTokenTip') }}</p>
      <div class="mobile-token-input-wrap">
        <input
          :type="showPassword ? 'text' : 'password'"
          :value="token"
          class="form-control"
          :placeholder="t('nav.adminTokenPlaceholder')"
          @input="emit('update:token', ($event.target as HTMLInputElement).value)"
        />
        <button
          type="button"
          class="eye-toggle"
          @click="emit('toggle-visibility')"
        >
          {{ showPassword ? '👁️' : '🔒' }}
        </button>
      </div>
      <div class="mobile-token-modal-footer">
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:open', false)">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary btn-sm" @click="emit('save')">
          {{ t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mobile-token-modal-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.mobile-token-modal-card {
  width: min(92vw, 360px);
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  padding: 20px;
  box-shadow: var(--shadow-lg);
  animation: popIn 0.18s ease-out;
}
.mobile-token-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.mobile-token-modal-header h4 {
  font-size: 1rem;
  font-weight: 600;
}
.mobile-token-modal-tip {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-bottom: 14px;
  line-height: 1.4;
}
.mobile-token-input-wrap {
  position: relative;
  margin-bottom: 16px;
}
.mobile-token-input-wrap input {
  padding-right: 36px;
}
.mobile-token-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
@keyframes popIn {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}
</style>
