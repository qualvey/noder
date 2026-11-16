<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  isExpanded: boolean
  isMidScreen: boolean
  isManualCollapsed: boolean
  theme: 'light' | 'dark'
  openTokenModal: () => void
}>()

const emit = defineEmits<{
  (e: 'toggleTheme'): void
  (e: 'toggleLocale'): void
  (e: 'update:collapsed', val: boolean): void
}>()

const { t, locale } = useI18n()
</script>

<template>
  <div class="sidebar-footer">
    <div class="footer-actions" :class="{ stacked: !isExpanded }">
      <button
        type="button"
        class="icon-btn-action"
        :title="t('nav.switchLang')"
        @click="emit('toggleLocale')"
      >
        🌐
        <span class="action-text" v-show="isExpanded">{{ locale === 'zh' ? 'EN' : '中文' }}</span>
      </button>

      <button
        type="button"
        class="icon-btn-action"
        :title="theme === 'light' ? t('nav.themeDark') : t('nav.themeLight')"
        @click="emit('toggleTheme')"
      >
        <span>{{ theme === 'light' ? '☀️' : '🌙' }}</span>
        <span class="action-text" v-show="isExpanded">{{ theme === 'light' ? 'Light' : 'Dark' }}</span>
      </button>

      <button
        type="button"
        class="icon-btn-action token-action-btn"
        :title="t('nav.adminToken')"
        @click="openTokenModal"
      >
        🔑
        <span class="action-text" v-show="isExpanded">Token</span>
      </button>
    </div>

    <button
      v-if="!isMidScreen"
      type="button"
      class="collapse-toggle-btn"
      :title="isManualCollapsed ? '展开侧边栏' : '收起侧边栏'"
      @click="emit('update:collapsed', !isManualCollapsed)"
    >
      <svg
        class="toggle-svg"
        :class="{ 'rotate-180': isManualCollapsed }"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <polyline points="11 17 6 12 11 7"></polyline>
        <polyline points="18 17 13 12 18 7"></polyline>
      </svg>
      <span class="collapse-text" v-show="isExpanded">{{ isManualCollapsed ? '展开导航' : '收起导航' }}</span>
    </button>
  </div>
</template>

<style scoped>
.sidebar-footer {
  padding: 12px 10px;
  border-top: 1px solid var(--border-glass);
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.footer-actions.stacked {
  flex-direction: column;
}

.icon-btn-action {
  flex: 1;
  height: 36px;
  min-width: 36px;
  padding: 0 8px;
  background: var(--bg-soft);
  border: 1px solid var(--border-glass);
  border-radius: 8px;
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.icon-btn-action:hover {
  background: var(--bg-hover);
  border-color: var(--primary);
  transform: translateY(-1px);
}

.collapse-toggle-btn {
  width: 100%;
  height: 32px;
  background: transparent;
  border: 1px dashed var(--border-glass);
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 0.76rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition: all 0.2s ease;
}

.collapse-toggle-btn:hover {
  background: var(--bg-soft);
  color: var(--text-main);
  border-style: solid;
}

.toggle-svg {
  width: 14px;
  height: 14px;
  transition: transform 0.24s ease;
}

.rotate-180 {
  transform: rotate(180deg);
}
</style>