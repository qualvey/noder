<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ToastItem } from '../composables/useToast'

defineProps<{
  toasts: ToastItem[]
}>()

const emit = defineEmits<{
  (e: 'dismiss', id: number): void
}>()

const { t } = useI18n()
</script>

<template>
  <div class="toast-container">
    <TransitionGroup name="toast">
      <div
        v-for="item in toasts"
        :key="item.id"
        class="toast"
        :style="{ borderColor: item.type === 'error' ? 'var(--accent-rose)' : 'var(--primary)' }"
        :title="t('common.close') || '点击关闭'"
        @click="emit('dismiss', item.id)"
      >
        <span>{{ item.type === 'error' ? '⚠️' : '✨' }}</span>
        <span>{{ item.message }}</span>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  bottom: auto;
  left: auto;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
  max-width: calc(100vw - 40px);
}
.toast {
  pointer-events: auto;
  cursor: pointer;
  user-select: none;
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
