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
const popoverElRef = ref<HTMLElement | null>(null)
const confirmBtnRef = ref<HTMLElement | null>(null)
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

async function showPopover(targetEl: Element, title: string, onConfirm: () => void) {
  const el = (targetEl as HTMLElement).closest?.('button, .btn, .icon-btn') || targetEl
  const rect = el.getBoundingClientRect()
  const targetCenterX = rect.left + rect.width / 2

  // 初始预估定位（基于常规宽度与确认按钮相对位置，避免跳跃）
  const estBtnOffset = 185
  const estWidth = 230
  let initLeft = Math.round(targetCenterX - estBtnOffset)
  let initTop = Math.round(rect.bottom + 8)
  if (initLeft < 10) initLeft = 10
  if (initLeft + estWidth > window.innerWidth - 10) initLeft = window.innerWidth - estWidth - 10

  popover.value = { title, x: initLeft, y: initTop, onConfirm }

  await nextTick()
  if (!popover.value || !popoverElRef.value || !confirmBtnRef.value) return

  const pEl = popoverElRef.value
  const bEl = confirmBtnRef.value

  // 计算弹窗内确认/删除按钮中心点相对于弹窗左边缘的精确偏移
  let offsetLeft = 0
  let curr: HTMLElement | null = bEl
  while (curr && curr !== pEl) {
    offsetLeft += curr.offsetLeft
    curr = curr.offsetParent as HTMLElement | null
  }
  const btnCenterInPopover = offsetLeft + bEl.offsetWidth / 2

  // 精准对齐：使弹窗的删除按钮正好处在原触发按钮（鼠标点击处）正下方
  let preciseLeft = Math.round(targetCenterX - btnCenterInPopover)
  let preciseTop = Math.round(rect.bottom + 8)
  const pWidth = pEl.offsetWidth
  const pHeight = pEl.offsetHeight

  // 视口边缘防溢出处理
  if (preciseLeft < 10) {
    preciseLeft = 10
  } else if (preciseLeft + pWidth > window.innerWidth - 10) {
    preciseLeft = window.innerWidth - pWidth - 10
  }

  if (preciseTop + pHeight > window.innerHeight - 10) {
    preciseTop = Math.round(rect.top - pHeight - 8)
  }
  if (preciseTop < 10) preciseTop = 10

  popover.value.x = preciseLeft
  popover.value.y = preciseTop
}

function hidePopover() {
  popover.value = null
}

function confirmPopover() {
  const p = popover.value
  hidePopover()
  if (p) p.onConfirm()
}

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

onMounted(() => {
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('scroll', handleGlobalScrollOrResize, { passive: true })
  window.addEventListener('resize', handleGlobalScrollOrResize, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('scroll', handleGlobalScrollOrResize)
  window.removeEventListener('resize', handleGlobalScrollOrResize)
})

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

// 移动端专用 Token 弹窗
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
      <!-- 移动端 (< 768px) 超轻量单行 Header (高度仅 48px，告别冗余) -->
      <header class="mobile-topbar">
        <div class="mobile-topbar-inner">
          <div class="mobile-topbar-brand">
            <div class="mobile-brand-icon">⚡</div>
            <div class="mobile-brand-name">Sing-Box Sub</div>
            <span class="mobile-status-dot" :title="t('nav.systemOnline')"></span>
          </div>

          <div class="mobile-topbar-actions">
            <!-- 语言切换 -->
            <button
              type="button"
              class="mobile-action-btn"
              :title="t('nav.switchLang')"
              @click="toggleLocale"
            >
              🌐
            </button>
            <!-- 主题切换 -->
            <button
              type="button"
              class="mobile-action-btn"
              :title="theme === 'light' ? t('nav.themeDark') : t('nav.themeLight')"
              @click="toggleTheme"
            >
              {{ theme === 'light' ? '☀️' : '🌙' }}
            </button>
            <!-- Token 弹窗快捷设置 -->
            <button
              type="button"
              class="mobile-action-btn"
              :title="t('nav.adminToken')"
              @click="openMobileTokenModal"
            >
              🔑
            </button>
          </div>
        </div>
      </header>

      <main class="container app-content-container">
        <!-- 桌面端 (>= 768px) 宽幅卡片 -->
        <div class="metrics-grid desktop-metrics">
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

        <!-- 移动端 (< 768px) 紧凑单行胶囊条 (超薄 34px，极省空间) -->
        <div class="mobile-metrics-strip">
          <div class="mobile-metric-pill">
            <span class="pill-dot node-dot"></span>
            <span class="pill-title">{{ t('nav.nodesOnline') }}</span>
            <span class="pill-val">{{ metrics.nodes }}</span>
          </div>
          <div class="mobile-metric-pill">
            <span class="pill-dot user-dot"></span>
            <span class="pill-title">{{ t('nav.usersActive') }}</span>
            <span class="pill-val">{{ metrics.users }}</span>
          </div>
        </div>

        <!-- 移动端吸附 Tab 导航栏 (必须直接作为 main 的子元素以保证 sticky 正常生效！) -->
        <div ref="stickyAnchorRef" class="tab-sticky-anchor mobile-nav-anchor"></div>

        <div class="tab-navigation mobile-tab-nav" :class="{ pinned: tabNavPinned }">
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

  <!-- 移动端 Token 弹窗 -->
  <div v-if="isMobileTokenModalOpen" class="mobile-token-modal-mask" @click.self="isMobileTokenModalOpen = false">
    <div class="mobile-token-modal-card">
      <div class="mobile-token-modal-header">
        <h4>{{ t('nav.adminTokenTitle') }}</h4>
        <button type="button" class="btn-close" @click="isMobileTokenModalOpen = false">✕</button>
      </div>
      <p class="mobile-token-modal-tip">{{ t('nav.adminTokenTip') }}</p>
      <div class="mobile-token-input-wrap">
        <input
          :type="showToken ? 'text' : 'password'"
          v-model="mobileTokenInput"
          class="form-control"
          :placeholder="t('nav.adminTokenPlaceholder')"
        />
        <button
          type="button"
          class="eye-toggle"
          @click="showToken = !showToken"
        >
          {{ showToken ? '👁️' : '🔒' }}
        </button>
      </div>
      <div class="mobile-token-modal-footer">
        <button type="button" class="btn btn-secondary btn-sm" @click="isMobileTokenModalOpen = false">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary btn-sm" @click="saveMobileToken">{{ t('common.save') }}</button>
      </div>
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
  <div v-if="popover" class="delete-confirm-mask" @click="hidePopover" />
  <div
    v-if="popover"
    ref="popoverElRef"
    class="delete-confirm-popover active"
    :style="{ left: popover.x + 'px', top: popover.y + 'px' }"
  >
    <div class="delete-confirm-title">{{ popover.title }}</div>
    <div class="delete-confirm-actions">
      <button type="button" class="btn btn-secondary btn-sm" @click="hidePopover">{{ t('common.cancel') }}</button>
      <button ref="confirmBtnRef" type="button" class="btn btn-danger btn-sm" @click="confirmPopover">{{ t('common.confirm') }}</button>
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

/* 响应式骨架 */
.app-layout {
  min-height: 100vh;
  position: relative;
}

.desktop-sidebar-container {
  display: none;
}

/* 移动端 (< 768px) 超薄顶栏 */
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

/* 移动端紧凑指标胶囊条 */
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

/* 移动端 Token 弹窗 */
.mobile-token-modal-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.mobile-token-modal-card {
  width: min(92vw, 360px);
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  padding: 20px;
  box-shadow: var(--shadow-lg);
  animation: popIn 0.18s ease-out;
}

.mobile-token-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.mobile-token-modal-header h4 {
  font-size: 1rem;
  font-weight: 600;
}

.mobile-token-modal-tip {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-bottom: 14px;
  line-height: 1.4;
}

.mobile-token-input-wrap {
  position: relative;
  margin-bottom: 16px;
}

.mobile-token-input-wrap input {
  padding-right: 36px;
}

.mobile-token-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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
