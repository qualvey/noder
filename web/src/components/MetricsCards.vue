<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  nodes: number
  users: number
}>()

const { t } = useI18n()
</script>

<template>
  <div class="metrics-grid desktop-metrics">
    <div class="metric-card">
      <div class="metric-info">
        <h4>{{ t('nav.nodesOnline') }}</h4>
        <div class="value">{{ nodes }}</div>
      </div>
      <div class="metric-icon icon-node">🌐</div>
    </div>
    <div class="metric-card">
      <div class="metric-info">
        <h4>{{ t('nav.usersActive') }}</h4>
        <div class="value">{{ users }}</div>
      </div>
      <div class="metric-icon icon-user">👤</div>
    </div>
  </div>

  <div class="mobile-metrics-strip">
    <div class="mobile-metric-pill">
      <span class="pill-dot node-dot"></span>
      <span class="pill-title">{{ t('nav.nodesOnline') }}</span>
      <span class="pill-val">{{ nodes }}</span>
    </div>
    <div class="mobile-metric-pill">
      <span class="pill-dot user-dot"></span>
      <span class="pill-title">{{ t('nav.usersActive') }}</span>
      <span class="pill-val">{{ users }}</span>
    </div>
  </div>
</template>

<style scoped>
.mobile-metrics-strip {
  display: flex;
  gap: 8px;
  margin: 10px 0 6px;
}
.mobile-metric-pill {
  flex: 1;
  height: 36px;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: 10px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  font-size: 0.8rem;
  backdrop-filter: blur(8px);
}
.pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.node-dot {
  background: var(--primary);
  box-shadow: 0 0 6px var(--primary);
}
.user-dot {
  background: #818cf8;
  box-shadow: 0 0 6px #818cf8;
}
.pill-title {
  color: var(--text-muted);
  margin-left: 6px;
  margin-right: auto;
  font-size: 0.78rem;
}
.pill-val {
  font-weight: 700;
  font-family: var(--font-mono);
  color: var(--text-main);
  font-size: 0.92rem;
}
@media (max-width: 767px) {
  .desktop-metrics {
    display: none !important;
  }
}
@media (min-width: 768px) {
  .mobile-metrics-strip {
    display: none !important;
  }
  .desktop-metrics {
    display: grid !important;
  }
}
</style>
