<script setup lang="ts">
import { provide, ref } from 'vue'
import ToolsView from './views/ToolsView.vue'

type ToastType = 'info' | 'error' | 'success' | 'warning'
interface ToastItem {
  id: number
  message: string
  type: ToastType
}

const toasts = ref<ToastItem[]>([])
let nextToastId = 0

function showToast(message: string, type: ToastType = 'info') {
  const id = ++nextToastId
  toasts.value.push({ id, message, type })
  window.setTimeout(() => {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }, 3000)
}

provide('toast', showToast)
</script>

<template>
  <div class="tools-shell">
    <header class="tools-shell-header">
      <div class="tools-shell-brand">
        <span class="brand-icon">✦</span>
        <div>
          <strong>Noder 工具箱</strong>
            <small>本地工具与短期订阅</small>
        </div>
      </div>
      <span class="tools-shell-status"><i /> Tools</span>
    </header>

    <main class="tools-shell-main">
      <ToolsView />
    </main>

    <div class="toast-stack" aria-live="polite">
      <div v-for="toast in toasts" :key="toast.id" class="toast" :class="`toast-${toast.type}`">
        {{ toast.message }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.tools-shell {
  min-height: 100vh;
}

.tools-shell-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  max-width: 1280px;
  margin: 0 auto;
  padding: 18px 24px;
  border-bottom: 1px solid var(--border-glass);
}

.tools-shell-brand,
.tools-shell-status {
  display: flex;
  align-items: center;
  gap: 10px;
}

.tools-shell-brand strong,
.tools-shell-brand small {
  display: block;
}

.tools-shell-brand strong {
  font-size: 1rem;
}

.tools-shell-brand small,
.tools-shell-status {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.brand-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 9px;
  background: linear-gradient(135deg, var(--primary), var(--accent-cyan));
  color: white;
}

.tools-shell-status i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent-emerald);
}

.tools-shell-main {
  max-width: 1280px;
  margin: 0 auto;
  padding: 24px;
}

.toast-stack {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 10;
  display: grid;
  gap: 8px;
}

.toast {
  max-width: min(360px, calc(100vw - 36px));
  padding: 10px 14px;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  background: var(--bg-elevated);
  color: var(--text-main);
  box-shadow: var(--shadow-lg);
  font-size: 0.78rem;
}

.toast-success { border-color: var(--accent-emerald); }
.toast-error { border-color: var(--accent-rose); }
.toast-warning { border-color: var(--accent-amber); }

@media (max-width: 600px) {
  .tools-shell-header,
  .tools-shell-main { padding: 16px 12px; }
  .tools-shell-status { display: none; }
}
</style>
