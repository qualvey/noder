<script setup lang="ts">
// 全局 UI 状态：toast 提示 + 鼠标位置删除确认弹窗
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
export type ToastType = 'info' | 'error' | 'success' | 'warning'
export interface ToastItem {
  id: number
  message: string
  type: ToastType
}

export interface PopoverState {
  title: string
  x: number
  y: number
  onConfirm: () => void
}

const toasts = ref<ToastItem[]>([])
const popover = ref<PopoverState | null>(null)
let toastId = 0

function showToast(message: string, type: ToastType = 'info') {
  const id = ++toastId
  toasts.value.push({ id, message, type })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, 3000)
}

function removeToast(id: number) {
  toasts.value = toasts.value.filter((t) => t.id !== id)
}

function showPopover(targetEl: Element, title: string, onConfirm: () => void) {
  const rect = targetEl.getBoundingClientRect()
  const width = 230
  const height = 80
  let left = rect.left + rect.width / 2 - width / 2
  let top = rect.bottom + 8
  if (left < 10) left = 10
  if (left + width > window.innerWidth - 10) left = window.innerWidth - width - 10
  if (top + height > window.innerHeight - 10) top = rect.top - height - 8
  popover.value = { title, x: left, y: top, onConfirm }
}

function hidePopover() {
  popover.value = null
}

function confirmPopover() {
  const p = popover.value
  hidePopover()
  if (p) p.onConfirm()
}

// 暴露给子视图
provide('toast', showToast)
provide('popover', { show: showPopover, hide: hidePopover })

import { useI18n } from 'vue-i18n'
import { setLocale, type LocaleType } from './i18n'

const { t, locale } = useI18n()

function toggleLocale() {
  const next: LocaleType = locale.value === 'zh' ? 'en' : 'zh'
  setLocale(next)
}

// 页面状态
const activeTab = ref<'nodes' | 'users' | 'files' | 'help'>('nodes')
const adminTokenInput = ref(localStorage.getItem('admin_token') || 'admin-secret')
const showToken = ref(false)
const metrics = ref({ nodes: 0, users: 0 })

// 明暗模式（初始值由 main.ts mount 前设置，此处读取实际生效值）
const theme = ref<'light' | 'dark'>(document.documentElement.dataset.theme === 'light' ? 'light' : 'dark')

function toggleTheme() {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  document.documentElement.dataset.theme = theme.value
  localStorage.setItem('noder_theme', theme.value)
}

// Tab 定义 + 滑块位移索引
const tabs = computed(() => [
  { key: 'nodes' as const, label: t('nav.nodes') },
  { key: 'users' as const, label: t('nav.users') },
  { key: 'files' as const, label: t('nav.files') },
  { key: 'help' as const, label: t('nav.help') },
])
const tabIndex = computed(() => tabs.value.findIndex((t) => t.key === activeTab.value))

const stickyAnchorRef = ref<HTMLElement | null>(null)
const tabNavPinned = ref(false)
const isHeightExpanded = ref(false)

function getNavNaturalScrollY(): number {
  if (!stickyAnchorRef.value) return 0
  const anchorRect = stickyAnchorRef.value.getBoundingClientRect()
  return Math.max(0, window.scrollY + anchorRect.top)
}

function updateTabNavSticky() {
  if (!stickyAnchorRef.value) return
  const anchorRect = stickyAnchorRef.value.getBoundingClientRect()
  const currentScrollY = window.scrollY

  // 1. 吸顶视觉状态：当锚点触顶且未在页面绝对顶部时生效
  const isStuck = anchorRect.top <= 1 && currentScrollY > 2
  tabNavPinned.value = isStuck

  // 2. 如果触顶，开启高度支撑，保证后续切 Tab 或向上滑动的平滑过渡
  if (isStuck) {
    isHeightExpanded.value = true
  }

  // 3. 只有当向上滑动直到 Header 完全显示在视口（scrollY <= 2）时，才释放支撑高度
  if (currentScrollY <= 2) {
    isHeightExpanded.value = false
    tabNavPinned.value = false
  }
}

const handleTabNavScroll = () => updateTabNavSticky()

watch(activeTab, async () => {
  const wasSticky = tabNavPinned.value || isHeightExpanded.value
  const targetScrollY = getNavNaturalScrollY()

  if (wasSticky) {
    isHeightExpanded.value = true
    window.scrollTo({ top: targetScrollY, behavior: 'instant' })
  }

  await nextTick()

  if (wasSticky) {
    window.scrollTo({ top: targetScrollY, behavior: 'instant' })
    tabNavPinned.value = true
    isHeightExpanded.value = true
  } else {
    updateTabNavSticky()
  }
})

onMounted(() => {
  updateTabNavSticky()
  window.addEventListener('scroll', handleTabNavScroll, { passive: true })
  window.addEventListener('resize', handleTabNavScroll, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', handleTabNavScroll)
  window.removeEventListener('resize', handleTabNavScroll)
})

import { getAdminToken, setAdminToken } from './api'
import NodesView from './views/NodesView.vue'
import UsersView from './views/UsersView.vue'
import FilesView from './views/FilesView.vue'
import HelpView from './views/HelpView.vue'

function saveToken() {
  setAdminToken(adminTokenInput.value.trim())
  showToast(t('nav.adminTokenSaved'))
  window.location.reload()
}

import Sidebar from './components/Sidebar.vue'

const isSidebarCollapsed = ref(localStorage.getItem('noder_sidebar_collapsed') === 'true')

function handleSidebarSaveToken(val: string) {
  adminTokenInput.value = val
  saveToken()
}

function updateMetrics(n: number, u: number) {
  metrics.value = { nodes: n, users: u }
}
provide('metrics', updateMetrics)
</script>

<template>
  <div class="app-layout" :class="{ 'sidebar-collapsed': isSidebarCollapsed }">
    <!-- 桌面/中屏 (>= 768px) 现代化侧边栏 -->
    <div class="desktop-sidebar-container">
      <Sidebar
        v-model:activeTab="activeTab"
        v-model:collapsed="isSidebarCollapsed"
        :metrics="metrics"
        :adminToken="adminTokenInput"
        :theme="theme"
        @toggleTheme="toggleTheme"
        @toggleLocale="toggleLocale"
        @saveToken="handleSidebarSaveToken"
      />
    </div>

    <div class="app-viewport">
      <!-- 仅移动端 (< 768px) 顶栏 Header -->
      <header class="header mobile-only-header">
        <div class="container header-wrapper">
          <div class="brand">
            <div class="brand-icon">⚡</div>
            <div>
              <div class="brand-title">Sing-Box Sub Middleman</div>
              <div class="brand-subtitle"><span class="status-indicator"></span>{{ t('nav.systemOnline') }}</div>
            </div>
          </div>
          <div class="header-controls">
            <button class="btn btn-secondary btn-sm lang-toggle" :title="t('nav.switchLang')" @click="toggleLocale">
              🌐 {{ locale === 'zh' ? 'EN' : '中文' }}
            </button>
            <button class="btn btn-secondary btn-sm theme-toggle" :title="theme === 'light' ? t('nav.themeDark') : t('nav.themeLight')" @click="toggleTheme">
              {{ theme === 'light' ? '☀️' : '🌙' }}
            </button>
            <div class="admin-token-box">
              <label for="adminTokenInput">{{ t('nav.adminToken') }}:</label>
              <input :type="showToken ? 'text' : 'password'" id="adminTokenInput" v-model="adminTokenInput" :placeholder="t('nav.adminTokenPlaceholder')" />
              <button type="button" class="token-eye-btn" :title="showToken ? '隐藏' : '显示'" @click="showToken = !showToken">
                {{ showToken ? '👁️' : '🔒' }}
              </button>
              <button class="btn btn-primary btn-sm" @click="saveToken">{{ t('common.save') }}</button>
            </div>
          </div>
        </div>
      </header>

      <main class="container app-content-container">
        <!-- 概览指标卡片 -->
        <div class="metrics-grid">
          <div class="metric-card">
            <div class="metric-info">
              <h4>{{ t('nav.nodesOnline') }}</h4>
              <div class="value">{{ metrics.nodes }}</div>
            </div>
            <div class="metric-icon icon-node">🌐</div>
          </div>
          <div class="metric-card">
            <div class="metric-info">
              <h4>{{ t('nav.usersActive') }}</h4>
              <div class="value">{{ metrics.users }}</div>
            </div>
            <div class="metric-icon icon-user">👤</div>
          </div>
        </div>

        <!-- 仅移动端 (< 768px) 展示触顶吸附 Tab 导航条 -->
        <div class="mobile-only-tabs">
          <!-- 锚点：在正常文档流中精准标定 tab-navigation 的起始位置 -->
          <div ref="stickyAnchorRef" class="tab-sticky-anchor"></div>

          <div class="tab-navigation" :class="{ pinned: tabNavPinned }">
            <div
              class="tab-slider"
              :style="{
                width: 'calc((100% - 12px) / 4)',
                transform: `translateX(calc(${tabIndex} * 100%))`,
              }"
            >
              <div
                class="tab-slider-inner"
                :style="{
                  transform: `translateX(calc(-${tabIndex} * 25%))`,
                }"
              ></div>
            </div>
            <button
              v-for="t in tabs"
              :key="t.key"
              class="tab-btn"
              :class="{ active: activeTab === t.key }"
              @click="activeTab = t.key"
            >
              {{ t.label }}
            </button>
          </div>
        </div>

        <!-- 页面视图容器 -->
        <div class="tab-view-container" :class="{ 'sticky-expanded': isHeightExpanded }">
          <NodesView v-if="activeTab === 'nodes'" />
          <UsersView v-else-if="activeTab === 'users'" />
          <FilesView v-else-if="activeTab === 'files'" />
          <HelpView v-else />
        </div>
      </main>
    </div>
  </div>

  <!-- Toast 容器 -->
  <div class="toast-container">
    <TransitionGroup name="toast">
      <div
        v-for="item in toasts"
        :key="item.id"
        class="toast"
        :style="{ borderColor: item.type === 'error' ? 'var(--accent-rose)' : 'var(--primary)' }"
        :title="t('common.close') || '点击关闭'"
        @click="removeToast(item.id)"
      >
        <span>{{ item.type === 'error' ? '⚠️' : '✨' }}</span><span>{{ item.message }}</span>
      </div>
    </TransitionGroup>
  </div>

  <!-- 鼠标位置删除确认弹窗 -->
  <div v-if="popover" class="delete-confirm-popover active" :style="{ left: popover.x + 'px', top: popover.y + 'px' }">
    <div class="delete-confirm-title">{{ popover.title }}</div>
    <div class="delete-confirm-actions">
      <button type="button" class="btn btn-secondary btn-sm" @click="hidePopover">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-danger btn-sm" @click="confirmPopover">{{ t('common.confirm') }}</button>
    </div>
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
.delete-confirm-popover {
  position: fixed;
  z-index: 1500;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: 10px;
  padding: 12px 14px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
  width: 230px;
}
.delete-confirm-title {
  font-size: 0.85rem;
  margin-bottom: 10px;
  color: var(--text);
}
.delete-confirm-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

/* 响应式骨架 */
.app-layout {
  min-height: 100vh;
  position: relative;
}

.desktop-sidebar-container {
  display: none;
}

.mobile-only-header,
.mobile-only-tabs {
  display: block;
}

@media (min-width: 768px) {
  .desktop-sidebar-container {
    display: block;
  }
  .mobile-only-header,
  .mobile-only-tabs {
    display: none !important;
  }
  .app-viewport {
    margin-left: 68px; /* 中屏 Rail 紧凑窄条宽度 */
    min-height: 100vh;
    transition: margin-left 0.24s cubic-bezier(0.4, 0, 0.2, 1);
  }
  .app-content-container {
    padding-top: 24px;
    padding-bottom: 48px;
  }
}

@media (min-width: 1200px) {
  /* 宽屏未折叠时 */
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
  min-height: calc(100vh + 200px);
}

.tab-navigation {
  position: sticky;
  top: 0;
  z-index: 110;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  margin: 18px 0 20px;
  padding: 6px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.025);
  backdrop-filter: blur(12px);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.2);
  isolation: isolate;
  transition:
    box-shadow 0.24s ease,
    border-color 0.24s ease,
    background 0.24s ease;
}

.tab-navigation.pinned {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  border-color: rgba(99, 102, 241, 0.3);
  background: rgba(15, 23, 42, 0.92);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

:global([data-theme='light']) .tab-navigation {
  background: rgba(0, 0, 0, 0.03);
  border-color: rgba(0, 0, 0, 0.06);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.04);
}

:global([data-theme='light']) .tab-navigation.pinned {
  background: rgba(255, 255, 255, 0.85);
  border-color: rgba(37, 99, 235, 0.2);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
}

.tab-slider {
  position: absolute;
  top: 6px;
  bottom: 6px;
  left: 6px;
  border-radius: 12px;
  overflow: hidden;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.3),
    0 4px 16px rgba(79, 70, 229, 0.35);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
  pointer-events: none;
  z-index: 0;
}

.tab-slider-inner {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 400%;
  height: 100%;
  background: linear-gradient(90deg, #6366f1 0%, #8b5cf6 30%, #38bdf8 70%, #06b6d4 100%);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.28);
}

.tab-btn {
  position: relative;
  z-index: 1;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  font-size: 0.93rem;
  letter-spacing: 0.01em;
  padding: 10px 14px;
  border-radius: 12px;
  text-align: center;
  cursor: pointer;
  transition: color 0.2s ease;
}

.tab-btn:hover {
  color: var(--text-main);
}

.tab-btn.active {
  color: #fff;
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
