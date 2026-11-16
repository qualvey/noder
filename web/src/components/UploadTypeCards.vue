<script setup lang="ts">
import { useI18n } from 'vue-i18n'

export type UploadFileType = 'apk' | 'zip' | 'text'

defineProps<{
  current: UploadFileType
}>()

const emit = defineEmits<{
  (e: 'pick', t: UploadFileType): void
}>()

const { t } = useI18n()
</script>

<template>
  <div class="section-label">{{ t('files.modal.chooseType') }}</div>
  <div class="type-cards-grid">
    <div class="type-card" :class="{ active: current === 'apk' }" @click="emit('pick', 'apk')">
      <div class="card-icon-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
        </svg>
      </div>
      <div class="card-title">{{ t('files.modal.typeRegular') }}</div>
      <div class="card-desc">{{ t('files.modal.typeRegularDesc') }}</div>
    </div>

    <div class="type-card" :class="{ active: current === 'zip' }" @click="emit('pick', 'zip')">
      <div class="card-icon-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
        </svg>
      </div>
      <div class="card-title">{{ t('files.modal.typeZip') }}</div>
      <div class="card-desc">{{ t('files.modal.typeZipDesc') }}</div>
    </div>

    <div class="type-card" :class="{ active: current === 'text' }" @click="emit('pick', 'text')">
      <div class="card-icon-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h10" />
        </svg>
      </div>
      <div class="card-title">{{ t('files.modal.typeText') }}</div>
      <div class="card-desc">{{ t('files.modal.typeTextDesc') }}</div>
    </div>
  </div>
</template>

<style scoped>
.section-label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-dim);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 10px;
}
.type-cards-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}
.type-card {
  background: rgba(15, 23, 42, 0.6);
  border: 1.5px solid var(--border-glass);
  border-radius: 14px;
  padding: 16px 12px;
  text-align: center;
  cursor: pointer;
  transition: all 0.22s ease;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}
.type-card:hover {
  border-color: rgba(59, 130, 246, 0.4);
  background: rgba(30, 41, 59, 0.6);
}
.type-card.active {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.1);
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.15);
}
.card-icon-box {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
}
.card-icon-box svg {
  width: 20px;
  height: 20px;
}
.type-card.active .card-icon-box {
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
}
.card-title {
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--text-main);
}
.card-desc {
  font-size: 0.72rem;
  color: var(--text-muted);
}
@media (max-width: 580px) {
  .type-cards-grid {
    grid-template-columns: 1fr;
  }
}
</style>
