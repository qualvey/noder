<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { useToast } from './composables/useToast'
import { usePopover } from './composables/usePopover'
import { useTheme } from './composables/useTheme'
import { useAuth } from './composables/useAuth'
import { ToastKey, PopoverKey, MetricsKey } from './injectionKeys'
import { setLocale, type LocaleType } from './i18n'
import { mainNavItems, type TabKey } from './navigation'
import { useStorage } from './composables/useStorage'
import { useTabStickyNav } from './composables/useTabStickyNav'
import Sidebar from './components/Sidebar.vue'
import UnlockScreen from './components/UnlockScreen.vue'
import MobileTopbar from './components/MobileTopbar.vue'
import MetricsCards from './components/MetricsCards.vue'
import TabNavigation from './components/TabNavigation.vue'
import MobileTokenModal from './components/MobileTokenModal.vue'
import ToastHost from './components/ToastHost.vue'
import ConfirmPopover from './components/ConfirmPopover.vue'

const { t, locale } = useI18n()
const route = useRoute()

const { getBoolean } = useStorage('local')

// ============== 全局 UI 状态（通过 composable 管理） ==============
const { toasts, showToast, removeToast } = useToast()
const popoverState = usePopover()
const popover = popoverState.popover
const showPopover = popoverState.showPopover
const hidePopover = popoverState.hidePopover
const confirmPopover = popoverState.confirmPopover
const { theme, toggleTheme } = useTheme()
const { isUnlocked, adminToken, unlock, saveToken, revalidate } = useAuth()

const updateMetrics = ref<(nodes: number, users: number) => void>()

// 向子视图提供类型安全的依赖
provide(ToastKey, showToast)
provide(PopoverKey, { show: showPopover, hide: hidePopover })
provide(MetricsKey, (n: number, u: number) => {
  updateMetrics.value?.(n, u)
})

// ============== 页面状态 ==============
const activeTab = computed<TabKey>(() => {
  const navKey = route.meta.navKey
  return typeof navKey === 'string' ? (navKey as TabKey) : 'nodes'
})

const adminTokenInput = ref(adminToken.value || 'admin-secret')
const showToken = ref(false)
const unlockError = ref(false)
const metrics = ref({ nodes: 0, users: 0 })

const tabs = computed(() =>
  mainNavItems.map((item) => ({ key: item.key, to: item.to, label: t(item.labelKey) })),
)
const { stickyAnchorRef, tabNavPinned, isHeightExpanded } = useTabStickyNav(activeTab)

const isSidebarCollapsed = ref(getBoolean('noder_sidebar_collapsed'))

// ============== 语言切换 ==============
function toggleLocale() {
  const next: LocaleType = locale.value === 'zh' ? 'en' : 'zh'
  setLocale(next)
}

function handleSidebarSaveToken(val: string) {
  adminTokenInput.value = val
  saveToken()
}

function handleUnlockSubmit() {
  unlock(adminTokenInput.value)
}

// ============== 生命周期 ==============
onMounted(async () => {
  await revalidate()

  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('scroll', handleGlobalScrollOrResize, { passive: true })
  window.addEventListener('resize', handleGlobalScrollOrResize, { passive: true })

})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('scroll', handleGlobalScrollOrResize)
  window.removeEventListener('resize', handleGlobalScrollOrResize)
})

function handleGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && popover.value) {
    hidePopover()
  }
}

function handleGlobalScrollOrResize() {
  if (popover.value) {
    hidePopover()
  }
}

// ============== 移动端 Token 弹窗 ==============
const isMobileTokenModalOpen = ref(false)
const mobileTokenInput = ref(adminTokenInput.value)

function openMobileTokenModal() {
  mobileTokenInput.value = adminTokenInput.value
  isMobileTokenModalOpen.value = true
}

function saveMobileToken() {
  adminTokenInput.value = mobileTokenInput.value.trim()
  isMobileTokenModalOpen.value = false
  saveToken()
}

function updateMetricsCallback(n: number, u: number) {
  metrics.value = { nodes: n, users: u }
}
updateMetrics.value = updateMetricsCallback
</script>

<template>
  <UnlockScreen v-if="!isUnlocked" :token="adminTokenInput" :show-password="showToken" :unlock-error="unlockError"
    @update:token="(val) => (adminTokenInput = val)" @toggle-visibility="showToken = !showToken"
    @clear-error="unlockError = false" @submit="handleUnlockSubmit" />

  <!-- 已解锁的主布局 -->
  <template v-else>
    <div class="app-layout" :class="{ 'sidebar-collapsed': isSidebarCollapsed }">
      <!-- 桌面/中屏 (>= 768px) 现代化侧边栏 -->
      <div class="desktop-sidebar-container">
        <Sidebar v-model:collapsed="isSidebarCollapsed" :metrics="metrics" :adminToken="adminTokenInput" :theme="theme"
          @toggleTheme="toggleTheme" @toggleLocale="toggleLocale" @saveToken="handleSidebarSaveToken" />
      </div>

      <div class="app-viewport">
        <MobileTopbar :theme="theme" @toggle-locale="toggleLocale" @toggle-theme="toggleTheme"
          @open-token="openMobileTokenModal" />

        <main class="container app-content-container">
          <MetricsCards v-if="activeTab !== 'tools'" :nodes="metrics.nodes" :users="metrics.users" />

          <!-- 移动端吸附 Tab 导航栏 -->
          <div ref="stickyAnchorRef" class="tab-sticky-anchor mobile-nav-anchor"></div>

          <TabNavigation :tabs="tabs" :active-tab="activeTab" :pinned="tabNavPinned" />

          <!-- 页面视图容器 -->
          <div class="tab-view-container" :class="{ 'sticky-expanded': isHeightExpanded }">
            <RouterView />
          </div>
        </main>
      </div>
    </div>

    <MobileTokenModal :open="isMobileTokenModalOpen" :token="mobileTokenInput" :show-password="showToken"
      @update:open="(val) => (isMobileTokenModalOpen = val)" @update:token="(val) => (mobileTokenInput = val)"
      @toggle-visibility="showToken = !showToken" @save="saveMobileToken" />

    <ToastHost :toasts="toasts" @dismiss="removeToast" />

    <ConfirmPopover :popover="popover" @hide="hidePopover" @confirm="confirmPopover" />
  </template>
</template>

<style scoped>
/* 响应式骨架 */
.app-layout {
  min-height: 100vh;
  position: relative;
}

.desktop-sidebar-container {
  display: none;
}

/* 桌面端断点与隐藏 */
@media (max-width: 767px) {
  .desktop-metrics {
    display: none !important;
  }

  .app-content-container {
    padding: 0 12px;
  }
}

@media (min-width: 768px) {

  .mobile-topbar,
  .mobile-metrics-strip,
  .mobile-tab-nav,
  .mobile-nav-anchor {
    display: none !important;
  }

  .desktop-sidebar-container {
    display: block;
  }

  .desktop-metrics {
    display: grid !important;
  }

  .app-viewport {
    margin-left: 68px;
    min-height: 100vh;
    transition: margin-left 0.24s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .app-content-container {
    padding-top: 24px;
    padding-bottom: 48px;
  }
}

@media (min-width: 1200px) {
  .app-layout:not(.sidebar-collapsed) .app-viewport {
    margin-left: 240px;
  }

  .app-layout.sidebar-collapsed .app-viewport {
    margin-left: 68px;
  }

  .app-content-container {
    max-width: 1440px;
    padding-left: 32px;
    padding-right: 32px;
  }
}

.tab-sticky-anchor {
  height: 0;
  margin: 0;
  padding: 0;
  pointer-events: none;
  visibility: hidden;
}

.tab-view-container {
  min-height: auto;
}

.tab-view-container.sticky-expanded {
  min-height: calc(100dvh + 200px);
}
</style>