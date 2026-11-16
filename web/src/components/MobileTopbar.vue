<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  theme: 'light' | 'dark'
}>()

const emit = defineEmits<{
  (e: 'toggle-locale'): void
  (e: 'toggle-theme'): void
  (e: 'open-token'): void
}>()

const { t } = useI18n()
</script>

<template>
  <header class="mobile-topbar">
    <div class="mobile-topbar-inner">
      <div class="mobile-topbar-brand">
        <div class="mobile-brand-icon">⚡</div>
        <div class="mobile-brand-name">Sing-Box Sub</div>
        <span class="mobile-status-dot" :title="t('nav.systemOnline')"></span>
      </div>

      <div class="mobile-topbar-actions">
        <button
          type="button"
          class="mobile-action-btn"
          :title="t('nav.switchLang')"
          @click="emit('toggle-locale')"
        >
          🌐
        </button>
        <button
          type="button"
          class="mobile-action-btn"
          :title="theme === 'light' ? t('nav.themeDark') : t('nav.themeLight')"
          @click="emit('toggle-theme')"
        >
          {{ theme === 'light' ? '☀️' : '🌙' }}
        </button>
        <button
          type="button"
          class="mobile-action-btn"
          :title="t('nav.adminToken')"
          @click="emit('open-token')"
        >
          🔑
        </button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.mobile-topbar {
  display: flex;
  align-items: center;
  height: 50px;
  background: var(--bg-header);
  border-bottom: 1px solid var(--border-glass);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  padding: 0 14px;
}
.mobile-topbar-inner {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.mobile-topbar-brand {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mobile-brand-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(135deg, var(--primary), var(--accent-cyan));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.85rem;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.4);
}
.mobile-brand-name {
  font-weight: 700;
  font-size: 0.95rem;
  letter-spacing: -0.01em;
  color: var(--text-main);
}
.mobile-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-emerald);
  box-shadow: 0 0 6px var(--accent-emerald);
}
.mobile-topbar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.mobile-action-btn {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  background: var(--bg-soft);
  border: 1px solid var(--border-glass);
  color: var(--text-main);
  font-size: 0.85rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
}
.mobile-action-btn:hover {
  background: var(--bg-hover);
  border-color: var(--primary);
}
</style>
