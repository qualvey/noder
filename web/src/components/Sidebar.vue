<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LocaleType } from '../i18n'

export type TabKey = 'nodes' | 'users' | 'files' | 'help'

const props = defineProps<{
  activeTab: TabKey
  metrics: { nodes: number; users: number }
  adminToken: string
  theme: 'light' | 'dark'
  collapsed?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:activeTab', val: TabKey): void
  (e: 'update:collapsed', val: boolean): void
  (e: 'toggleTheme'): void
  (e: 'toggleLocale'): void
  (e: 'saveToken', val: string): void
}>()

const { t, locale } = useI18n()

// 手动强制折叠状态 (宽屏下可手动折叠成 Rail)
const isManualCollapsed = ref(props.collapsed ?? (localStorage.getItem('noder_sidebar_collapsed') === 'true'))

// 中屏 Rail 模式下的 Hover 悬浮临时展开
const isHovered = ref(false)

// 当前屏幕视口级别
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

// 响应式级别判定：中屏 768px ~ 1199px，宽屏 >= 1200px
const isMidScreen = computed(() => windowWidth.value >= 768 && windowWidth.value < 1200)

// 是否处于收拢的 Rail 状态（中屏默认收拢，或者宽屏用户手动收拢）
const isRail = computed(() => {
  if (isMidScreen.value) return true
  return isManualCollapsed.value
})

// 最终是否处于完整展开显示内容的状态（常驻展开或悬浮展开）
const isExpanded = computed(() => {
  if (!isRail.value) return true
  return isHovered.value
})

function toggleManualCollapse() {
  isManualCollapsed.value = !isManualCollapsed.value
  localStorage.setItem('noder_sidebar_collapsed', String(isManualCollapsed.value))
  emit('update:collapsed', isManualCollapsed.value)
}

function handleMouseEnter() {
  if (isRail.value) {
    isHovered.value = true
  }
}

function handleMouseLeave() {
  if (isRail.value) {
    isHovered.value = false
  }
}

// Token 编辑状态
const isTokenModalOpen = ref(false)
const inputToken = ref(props.adminToken)
const showPassword = ref(false)

function openTokenModal() {
  inputToken.value = props.adminToken
  isTokenModalOpen.value = true
}

function saveToken() {
  emit('saveToken', inputToken.value.trim())
  isTokenModalOpen.value = false
}

// 导航菜单项
const navItems = computed(() => [
  {
    key: 'nodes' as const,
    label: t('nav.nodes'),
    badge: props.metrics.nodes,
    icon: 'nodes',
  },
  {
    key: 'users' as const,
    label: t('nav.users'),
    badge: props.metrics.users,
    icon: 'users',
  },
  {
    key: 'files' as const,
    label: t('nav.files'),
    badge: undefined,
    icon: 'files',
  },
  {
    key: 'help' as const,
    label: t('nav.help'),
    badge: undefined,
    icon: 'help',
  },
])
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
    <!-- 顶部 Brand -->
    <div class="sidebar-header">
      <div class="brand-badge">⚡</div>
      <div class="brand-text" v-show="isExpanded">
        <div class="brand-title">Sing-Box Sub</div>
        <div class="brand-subtitle">
          <span class="pulse-dot"></span>
          <span>{{ t('nav.systemOnline') }}</span>
        </div>
      </div>
    </div>

    <!-- 菜单列表 -->
    <nav class="sidebar-nav">
      <button
        v-for="item in navItems"
        :key="item.key"
        type="button"
        class="nav-item"
        :class="{ active: activeTab === item.key }"
        :title="!isExpanded ? item.label : undefined"
        @click="emit('update:activeTab', item.key)"
      >
        <span class="nav-icon-wrapper">
          <!-- Nodes Icon -->
          <svg v-if="item.icon === 'nodes'" class="nav-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
            <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
            <line x1="6" y1="6" x2="6.01" y2="6"></line>
            <line x1="6" y1="18" x2="6.01" y2="18"></line>
          </svg>
          <!-- Users Icon -->
          <svg v-else-if="item.icon === 'users'" class="nav-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
            <circle cx="9" cy="7" r="4"></circle>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
          </svg>
          <!-- Files Icon -->
          <svg v-else-if="item.icon === 'files'" class="nav-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            <polyline points="14 2 14 8 20 8"></polyline>
            <line x1="16" y1="13" x2="8" y2="13"></line>
            <line x1="16" y1="17" x2="8" y2="17"></line>
            <polyline points="10 9 9 9 8 9"></polyline>
          </svg>
          <!-- Help Icon -->
          <svg v-else class="nav-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"></path>
            <line x1="12" y1="17" x2="12.01" y2="17"></line>
          </svg>
        </span>

        <span class="nav-label" v-show="isExpanded">{{ item.label }}</span>

        <!-- Badge 数量徽标 -->
        <span
          v-if="item.badge !== undefined"
          class="nav-badge"
          :class="{ 'badge-dot': !isExpanded }"
        >
          <span v-if="isExpanded">{{ item.badge }}</span>
        </span>
      </button>
    </nav>

    <!-- 底部控制面板 -->
    <div class="sidebar-footer">
      <div class="footer-actions" :class="{ 'stacked': !isExpanded }">
        <!-- 语言切换 -->
        <button
          type="button"
          class="icon-btn-action"
          :title="t('nav.switchLang')"
          @click="emit('toggleLocale')"
        >
          🌐 <span class="action-text" v-show="isExpanded">{{ locale === 'zh' ? 'EN' : '中文' }}</span>
        </button>

        <!-- 主题切换 -->
        <button
          type="button"
          class="icon-btn-action"
          :title="theme === 'light' ? t('nav.themeDark') : t('nav.themeLight')"
          @click="emit('toggleTheme')"
        >
          <span>{{ theme === 'light' ? '☀️' : '🌙' }}</span>
          <span class="action-text" v-show="isExpanded">{{ theme === 'light' ? 'Light' : 'Dark' }}</span>
        </button>

        <!-- Token 设置入口 -->
        <button
          type="button"
          class="icon-btn-action token-action-btn"
          :title="t('nav.adminToken')"
          @click="openTokenModal"
        >
          🔑 <span class="action-text" v-show="isExpanded">Token</span>
        </button>
      </div>

      <!-- 宽屏下的折叠切换按钮 -->
      <button
        v-if="!isMidScreen"
        type="button"
        class="collapse-toggle-btn"
        :title="isManualCollapsed ? '展开侧边栏' : '收起侧边栏'"
        @click="toggleManualCollapse"
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

    <!-- Admin Token 设置小弹窗 -->
    <div v-if="isTokenModalOpen" class="token-popover-mask" @click.self="isTokenModalOpen = false">
      <div class="token-popover-card">
        <div class="token-popover-header">
          <h4>{{ t('nav.adminTokenTitle') }}</h4>
          <button type="button" class="btn-close" @click="isTokenModalOpen = false">✕</button>
        </div>
        <p class="token-popover-tip">{{ t('nav.adminTokenTip') }}</p>
        <div class="token-popover-input-wrap">
          <input
            :type="showPassword ? 'text' : 'password'"
            v-model="inputToken"
            class="form-control"
            :placeholder="t('nav.adminTokenPlaceholder')"
          />
          <button
            type="button"
            class="eye-toggle"
            @click="showPassword = !showPassword"
          >
            {{ showPassword ? '👁️' : '🔒' }}
          </button>
        </div>
        <div class="token-popover-footer">
          <button type="button" class="btn btn-secondary btn-sm" @click="isTokenModalOpen = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary btn-sm" @click="saveToken">{{ t('common.save') }}</button>
        </div>
      </div>
    </div>
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

/* Rail 紧凑窄条模式 */
.app-sidebar.is-rail {
  width: 68px;
}

/* 中屏 Hover 悬浮展开态 */
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

/* 顶部 Brand */
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

/* 导航区 */
.sidebar-nav {
  flex: 1;
  padding: 14px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  overflow-x: hidden;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 44px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  white-space: nowrap;
}

.nav-item:hover {
  background: var(--bg-hover-weak);
  color: var(--text-main);
  border-color: rgba(255, 255, 255, 0.04);
}

.nav-item.active {
  background: linear-gradient(90deg, rgba(59, 130, 246, 0.16), rgba(99, 102, 241, 0.08));
  color: var(--primary);
  border-color: rgba(59, 130, 246, 0.3);
  font-weight: 600;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

:global([data-theme='light']) .nav-item.active {
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.12), rgba(99, 102, 241, 0.05));
  border-color: rgba(37, 99, 235, 0.25);
}

.nav-icon-wrapper {
  width: 20px;
  height: 20px;
  min-width: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-svg {
  width: 18px;
  height: 18px;
}

.nav-label {
  flex: 1;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-badge {
  background: var(--bg-soft);
  color: var(--text-dim);
  font-size: 0.72rem;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: 10px;
  border: 1px solid var(--border-glass);
}

.nav-item.active .nav-badge {
  background: rgba(59, 130, 246, 0.2);
  color: var(--primary);
  border-color: rgba(59, 130, 246, 0.35);
}

.nav-badge.badge-dot {
  width: 6px;
  height: 6px;
  padding: 0;
  border-radius: 50%;
  background: var(--primary);
  position: absolute;
  top: 10px;
  right: 10px;
}

/* 底部面板 */
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

/* Token Popover 弹窗 */
.token-popover-mask {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
}

.token-popover-card {
  width: 320px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: 12px;
  padding: 18px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  animation: popIn 0.18s ease-out;
}

@keyframes popIn {
  from { opacity: 0; transform: scale(0.95); }
  to { opacity: 1; transform: scale(1); }
}

.token-popover-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.token-popover-header h4 {
  font-size: 0.95rem;
  font-weight: 600;
}

.btn-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1rem;
}

.token-popover-tip {
  font-size: 0.78rem;
  color: var(--text-muted);
  margin-bottom: 12px;
  line-height: 1.4;
}

.token-popover-input-wrap {
  position: relative;
  margin-bottom: 16px;
}

.token-popover-input-wrap input {
  padding-right: 36px;
}

.eye-toggle {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 0.9rem;
}

.token-popover-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
