<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ConfigDetailItem } from './node-transfer/types'

defineProps<{
  nodeCount: number
  activeNodeIndex: number
  activeTag: string
  activeServer: string
  details: ConfigDetailItem[]
}>()

const emit = defineEmits<{
  (e: 'copy', text: string): void
}>()

const { t } = useI18n()
</script>

<template>
  <section class="transfer-card details-card">
    <div class="card-heading">
      <div>
        <span class="card-kicker">DETAILS</span>
        <h3>
          {{ t('tools.transfer.detailsTitle') }}
          <span v-if="nodeCount > 1" class="details-subtitle">
            — #{{ activeNodeIndex + 1 }} {{ activeTag || activeServer }}
          </span>
        </h3>
      </div>
    </div>
    <div class="details-list">
      <div v-for="(detail, i) in details" :key="i">
        <span>{{ detail.label }}</span>
        <strong :class="{ accent: detail.highlight }">{{ detail.value }}</strong>
        <button v-if="detail.copyable && detail.value !== '-'" @click="emit('copy', String(detail.value))">
          {{ t('tools.transfer.copy') }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.transfer-card {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  background: var(--bg-panel);
}
.card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 15px;
}
.card-heading h3 {
  margin: 0;
  font-size: 0.9rem;
}
.card-kicker {
  color: var(--primary);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.13em;
}
.details-subtitle {
  font-size: 0.76rem;
  font-weight: 500;
  color: var(--text-muted);
}
.details-card {
  display: grid;
  gap: 3px;
}
.details-list {
  overflow: hidden;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
}
.details-list > div {
  display: grid;
  grid-template-columns: minmax(130px, 0.7fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 9px 11px;
  border-bottom: 1px solid var(--border-glass);
  font-size: 0.68rem;
}
.details-list > div:last-child {
  border-bottom: 0;
}
.details-list span {
  color: var(--text-muted);
}
.details-list strong {
  overflow: hidden;
  font: 0.68rem var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.details-list button {
  border: 0;
  background: none;
  color: var(--primary);
  cursor: pointer;
  font: inherit;
  font-size: 0.62rem;
}
.accent {
  color: var(--primary) !important;
}
@media (max-width: 520px) {
  .details-list > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
