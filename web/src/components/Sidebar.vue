<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { mainNavItems } from '../navigation'
import { useStorage } from '../composables/useStorage'
import SidebarNavItem from './SidebarNavItem.vue'
import SidebarFooter from './SidebarFooter.vue'
import TokenPopover from './TokenPopover.vue'

const { setBoolean, getBoolean } = useStorage('local')

const props = defineProps<{
  metrics: { nodes: number; users: number }
  adminToken: string
  theme: 'light' | 'dark'
  collapsed?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:collapsed', val: boolean): void
  (e: 'toggleTheme'): void
  (e: 'toggleLocale'): void
  (e: 'saveToken', val: string): void
}>()

const { t } = useI18n()

const isManualCollapsed = ref(props.collapsed ?? getBoolean('noder_sidebar_collapsed'))
const isHovered = ref(false)
const isTokenModalOpen = ref(false)

const windowWidth = ref(window.innerWidth)
const updateWidth = () => {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  window.addEventListener('resize', updateWidth, { passive: true })
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', updateWidth)
})

const isMidScreen = computed(() => windowWidth.value >= 768 && windowWidth.value < 1200)
const isRail = computed(() => {
  if (isMidScreen.value) return true
  return isManualCollapsed.value
})
const isExpanded = computed(() => {
  if (!isRail.value) return true
  return isHovered.value
})

function toggleManualCollapse() {
  isManualCollapsed.value = !isManualCollapsed.value
  setBoolean('noder_sidebar_collapsed', isManualCollapsed.value)
  emit('update:collapsed', isManualCollapsed.value)
}

function handleMouseEnter() {
  if (isRail.value) isHovered.value = true
}

function handleMouseLeave() {
  if (isRail.value) isHovered.value = false
}

const navItems = computed(() =>
  mainNavItems.map((item) => ({
    ...item,
    badge: item.key === 'nodes' ? props.metrics.nodes : item.key === 'users' ? props.metrics.users : undefined,
  })),
)

function openTokenModal() {
  isTokenModalOpen.value = true
}
</script>

<template>
  <aside
    class="app-sidebar"
    :class="{
      'is-rail': isRail,
      'is-expanded': isExpanded,
      'is-floating': isRail && isHovered,
    }"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <div class="sidebar-header">
      <div class="brand-text" v-show="isExpanded">
        <div class="brand-title">欢喜飞机</div>
        <div class="brand-subtitle">
          <span class="pulse-dot"></span>
          <span>{{ t('nav.systemOnline') }}</span>
        </div>
      </div>
    </div>

    <nav class="sidebar-nav">
      <SidebarNavItem
        v-for="item in navItems"
        :key="item.key"
        :item="item"
        :is-expanded="isExpanded"
      />
    </nav>

    <SidebarFooter
      :is-expanded="isExpanded"
      :is-mid-screen="isMidScreen"
      :is-manual-collapsed="isManualCollapsed"
      :theme="theme"
      :open-token-modal="openTokenModal"
      @toggleTheme="emit('toggleTheme')"
      @toggleLocale="emit('toggleLocale')"
      @update:collapsed="toggleManualCollapse"
    />

    <TokenPopover
      :open="isTokenModalOpen"
      :adminToken="props.adminToken"
      @update:open="(val) => (isTokenModalOpen = val)"
      @save="emit('saveToken', $event)"
    />
  </aside>
</template>

<style scoped>
.app-sidebar {
  position: fixed;
  top: 0;
  bottom: 0;
  left: 0;
  width: 240px;
  height: 100vh;
  background: var(--bg-card);
  border-right: 1px solid var(--border-glass);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  z-index: 100;
  display: flex;
  flex-direction: column;
  transition: width 0.24s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.24s ease, transform 0.24s ease;
  user-select: none;
}

.app-sidebar.is-rail {
  width: 68px;
}

.app-sidebar.is-floating {
  width: 240px;
  box-shadow: 12px 0 36px rgba(0, 0, 0, 0.45);
  background: rgba(15, 23, 42, 0.95);
  border-right-color: rgba(99, 102, 241, 0.3);
}

:global([data-theme='light']) .app-sidebar.is-floating {
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 12px 0 36px rgba(0, 0, 0, 0.12);
}

.sidebar-header {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-glass);
  flex-shrink: 0;
  overflow: hidden;
}

.brand-badge {
  width: 36px;
  height: 36px;
  min-width: 36px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--primary), var(--accent-cyan));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.15rem;
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.4);
}

.brand-text {
  overflow: hidden;
  white-space: nowrap;
}

.brand-title {
  font-weight: 700;
  font-size: 1rem;
  color: var(--text-main);
  letter-spacing: -0.01em;
}

.brand-subtitle {
  font-size: 0.72rem;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-emerald);
  box-shadow: 0 0 8px var(--accent-emerald);
}

.sidebar-nav {
  flex: 1;
  padding: 14px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  overflow-x: hidden;
}
</style>