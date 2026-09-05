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

function updateMetrics(n: number, u: number) {
  metrics.value = { nodes: n, users: u }
}
provide('metrics', updateMetrics)
</script>

<template>
  <header class="header">
    <div class="container header-wrapper">
      <div class="brand">
        <div class="brand-icon">⚡</div>
        <div>
          <div class="brand-title">Sing-Box Sub Middleman</div>
          <div class="brand-subtitle">{{ t('nav.systemOnline') }} (TUIC / VLESS REALITY / AnyTLS)</div>
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
          <input type="password" id="adminTokenInput" v-model="adminTokenInput" :placeholder="t('nav.adminTokenPlaceholder')" />
          <button class="btn btn-secondary btn-sm" @click="saveToken">{{ t('common.save') }}</button>
        </div>
      </div>
    </div>
  </header>

  <main class="container">
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

    <!-- 锚点：在正常文档流中精准标定 tab-navigation 的起始位置 -->
    <div ref="stickyAnchorRef" class="tab-sticky-anchor"></div>

    <div class="tab-navigation" :class="{ pinned: tabNavPinned }">
      <div
        class="tab-slider"
        :data-index="tabIndex"
        :style="{
          transform: `translateX(${tabIndex * 100}%)`,
          width: `${100 / tabs.length}%`,
        }"
      ></div>
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

    <div class="tab-view-container" :class="{ 'sticky-expanded': isHeightExpanded }">
      <NodesView v-if="activeTab === 'nodes'" />
      <UsersView v-else-if="activeTab === 'users'" />
      <FilesView v-else-if="activeTab === 'files'" />
      <HelpView v-else />
    </div>
  </main>

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
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 18px 0 20px;
  padding: 8px;
  border: 1px solid var(--border-glass);
  border-radius: 18px;
  background: var(--bg-card);
  backdrop-filter: blur(18px);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
  isolation: isolate;
  transition:
    box-shadow 0.24s ease,
    border-color 0.24s ease,
    background 0.24s ease,
    transform 0.24s ease;
}

.tab-navigation.pinned {
  box-shadow: 0 16px 32px rgba(0, 0, 0, 0.35);
  border-color: rgba(99, 102, 241, 0.35);
  background: var(--bg-card-hover);
  transform: translateY(-2px);
}

.tab-slider {
  position: absolute;
  inset: 8px auto 8px 8px;
  width: 25%;
  border-radius: 12px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.95), rgba(56, 189, 248, 0.9));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.28),
    0 8px 20px rgba(79, 70, 229, 0.35);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1), width 0.28s ease;
}

.tab-btn {
  position: relative;
  z-index: 1;
  flex: 1 1 0;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  font-size: 0.93rem;
  letter-spacing: 0.01em;
  padding: 12px 14px;
  border-radius: 12px;
  cursor: pointer;
  transition: color 0.2s ease, transform 0.2s ease;
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
