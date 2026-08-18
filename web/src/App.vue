<script setup lang="ts">
// 全局 UI 状态：toast 提示 + 鼠标位置删除确认弹窗
import { provide, ref } from 'vue'

export interface ToastItem {
  id: number
  message: string
  type: 'info' | 'error'
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

function showToast(message: string, type: 'info' | 'error' = 'info') {
  const id = ++toastId
  toasts.value.push({ id, message, type })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, 3000)
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

// 页面状态
const activeTab = ref<'nodes' | 'users' | 'files' | 'help'>('nodes')
const adminTokenInput = ref(localStorage.getItem('admin_token') || 'admin-secret')
const metrics = ref({ nodes: 0, users: 0 })

import { getAdminToken, setAdminToken } from './api'
import NodesView from './views/NodesView.vue'
import UsersView from './views/UsersView.vue'
import FilesView from './views/FilesView.vue'
import HelpView from './views/HelpView.vue'

function saveToken() {
  setAdminToken(adminTokenInput.value.trim())
  showToast('Admin Token 已保存')
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
          <div class="brand-subtitle">节点管理与动态订阅生成系统 (TUIC / VLESS REALITY / AnyTLS 版)</div>
        </div>
      </div>
      <div class="header-controls">
        <div class="admin-token-box">
          <label for="adminTokenInput">Admin Token:</label>
          <input type="password" id="adminTokenInput" v-model="adminTokenInput" placeholder="输入密钥" />
          <button class="btn btn-secondary btn-sm" @click="saveToken">保存</button>
        </div>
      </div>
    </div>
  </header>

  <main class="container">
    <div class="metrics-grid">
      <div class="metric-card">
        <div class="metric-info">
          <h4>托管节点总数</h4>
          <div class="value">{{ metrics.nodes }}</div>
        </div>
        <div class="metric-icon icon-node">🌐</div>
      </div>
      <div class="metric-card">
        <div class="metric-info">
          <h4>活跃订阅用户</h4>
          <div class="value">{{ metrics.users }}</div>
        </div>
        <div class="metric-icon icon-user">👤</div>
      </div>
      <div class="metric-card">
        <div class="metric-info">
          <h4>支持核心协议</h4>
          <div class="value">TUIC / VLESS / AnyTLS</div>
        </div>
        <div class="metric-icon icon-protocol">🔒</div>
      </div>
      <div class="metric-card">
        <div class="metric-info">
          <h4>服务 API 状态</h4>
          <div class="value" style="color: var(--accent-emerald)">运行中</div>
        </div>
        <div class="metric-icon icon-status">✅</div>
      </div>
    </div>

    <div class="tab-navigation">
      <button class="tab-btn" :class="{ active: activeTab === 'nodes' }" @click="activeTab = 'nodes'">节点管理 (Nodes)</button>
      <button class="tab-btn" :class="{ active: activeTab === 'users' }" @click="activeTab = 'users'">用户管理 (Users)</button>
      <button class="tab-btn" :class="{ active: activeTab === 'files' }" @click="activeTab = 'files'">文件分发 (Files)</button>
      <button class="tab-btn" :class="{ active: activeTab === 'help' }" @click="activeTab = 'help'">使用指引与 API</button>
    </div>

    <NodesView v-if="activeTab === 'nodes'" />
    <UsersView v-else-if="activeTab === 'users'" />
    <FilesView v-else-if="activeTab === 'files'" />
    <HelpView v-else />
  </main>

  <!-- Toast 容器 -->
  <div class="toast-container">
    <TransitionGroup name="toast">
      <div v-for="t in toasts" :key="t.id" class="toast" :style="{ borderColor: t.type === 'error' ? 'var(--accent-rose)' : 'var(--primary)' }">
        <span>{{ t.type === 'error' ? '⚠️' : '✨' }}</span><span>{{ t.message }}</span>
      </div>
    </TransitionGroup>
  </div>

  <!-- 鼠标位置删除确认弹窗 -->
  <div v-if="popover" class="delete-confirm-popover active" :style="{ left: popover.x + 'px', top: popover.y + 'px' }">
    <div class="delete-confirm-title">{{ popover.title }}</div>
    <div class="delete-confirm-actions">
      <button type="button" class="btn btn-secondary btn-sm" @click="hidePopover">取消</button>
      <button type="button" class="btn btn-danger btn-sm" @click="confirmPopover">确定删除</button>
    </div>
  </div>
</template>

<style scoped>
.toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  gap: 8px;
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
