<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  currentStep: number
}>()

const { t } = useI18n()
</script>

<template>
  <div class="step-indicator">
    <div class="step-item" :class="{ active: currentStep >= 1 }">
      <span class="step-num">1</span>
      <span class="step-label">{{ t('files.modal.step1') }}</span>
    </div>
    <div class="step-line" :class="{ filled: currentStep >= 2 }"></div>
    <div class="step-item" :class="{ active: currentStep >= 2 }">
      <span class="step-num">2</span>
      <span class="step-label">{{ t('files.modal.step2') }}</span>
    </div>
    <div class="step-line" :class="{ filled: currentStep >= 3 }"></div>
    <div class="step-item" :class="{ active: currentStep >= 3 }">
      <span class="step-num">3</span>
      <span class="step-label">{{ t('files.modal.step3') }}</span>
    </div>
  </div>
</template>

<style scoped>
.step-indicator {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 18px 0 22px;
  padding: 0 6px;
}
.step-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.82rem;
  color: var(--text-dim);
}
.step-item.active {
  color: var(--text-main);
  font-weight: 600;
}
.step-num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  color: var(--text-dim);
  font-weight: 600;
}
.step-item.active .step-num {
  background: var(--primary);
  color: #fff;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.5);
}
.step-line {
  flex: 1;
  height: 2px;
  background: rgba(255, 255, 255, 0.08);
  margin: 0 12px;
  border-radius: 2px;
}
.step-line.filled {
  background: linear-gradient(90deg, var(--primary), rgba(59, 130, 246, 0.4));
}
</style>
