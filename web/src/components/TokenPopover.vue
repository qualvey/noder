<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  open: boolean
  adminToken: string
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'save', val: string): void
}>()

const { t } = useI18n()

const inputToken = ref(props.adminToken)
const showPassword = ref(false)

function saveToken() {
  emit('save', inputToken.value.trim())
  emit('update:open', false)
}
</script>

<template>
  <div v-if="open" class="token-popover-mask" @mousedown.self="emit('update:open', false)">
    <div class="token-popover-card">
      <div class="token-popover-header">
        <h4>{{ t('nav.adminTokenTitle') }}</h4>
        <button type="button" class="btn-close" @click="emit('update:open', false)">✕</button>
      </div>
      <p class="token-popover-tip">{{ t('nav.adminTokenTip') }}</p>
      <div class="token-popover-input-wrap">
        <input
          :type="showPassword ? 'text' : 'password'"
          v-model="inputToken"
          class="form-control"
          :placeholder="t('nav.adminTokenPlaceholder')"
        />
        <button
          type="button"
          class="eye-toggle"
          @click="showPassword = !showPassword"
        >
          {{ showPassword ? '👁️' : '🔒' }}
        </button>
      </div>
      <div class="token-popover-footer">
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:open', false)">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary btn-sm" @click="saveToken">
          {{ t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.token-popover-mask {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
}

.token-popover-card {
  width: 320px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: 12px;
  padding: 18px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  animation: popIn 0.18s ease-out;
}

@keyframes popIn {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}

.token-popover-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.token-popover-header h4 {
  font-size: 0.95rem;
  font-weight: 600;
}

.btn-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1rem;
}

.token-popover-tip {
  font-size: 0.78rem;
  color: var(--text-muted);
  margin-bottom: 12px;
  line-height: 1.4;
}

.token-popover-input-wrap {
  position: relative;
  margin-bottom: 16px;
}

.token-popover-input-wrap input {
  padding-right: 36px;
}

.eye-toggle {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 0.9rem;
}

.token-popover-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>