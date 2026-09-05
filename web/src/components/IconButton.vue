<script setup lang="ts">
withDefaults(
  defineProps<{
    icon?: 'edit' | 'delete' | 'copy' | 'refresh' | string
    tip?: string
    variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
    size?: 'sm' | 'md'
    disabled?: boolean
    placement?: 'top' | 'bottom'
  }>(),
  {
    icon: '',
    tip: '',
    variant: 'secondary',
    size: 'sm',
    disabled: false,
    placement: 'top'
  }
)
</script>

<template>
  <button
    type="button"
    class="icon-btn"
    :class="[`icon-btn-${variant}`, `icon-btn-${size}`]"
    :disabled="disabled"
    :aria-label="tip"
  >
    <span class="icon-btn-icon">
      <slot>
        <!-- 内置常用矢量图标 -->
        <svg
          v-if="icon === 'edit'"
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
        </svg>
        <svg
          v-else-if="icon === 'delete'"
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <polyline points="3 6 5 6 21 6"></polyline>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          <line x1="10" y1="11" x2="10" y2="17"></line>
          <line x1="14" y1="11" x2="14" y2="17"></line>
        </svg>
        <svg
          v-else-if="icon === 'copy'"
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
        </svg>
      </slot>
    </span>
    <span v-if="tip" class="icon-btn-tooltip" :class="`placement-${placement}`">
      {{ tip }}
    </span>
  </button>
</template>

<style scoped>
.icon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm, 6px);
  cursor: pointer;
  transition: all 0.18s ease;
  border: 1px solid transparent;
  outline: none;
  user-select: none;
  line-height: 1;
}

.icon-btn-sm {
  width: 28px;
  height: 28px;
  padding: 0;
}

.icon-btn-md {
  width: 34px;
  height: 34px;
  padding: 0;
}

.icon-btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Secondary 样式（默认用于编辑等） */
.icon-btn-secondary {
  background: var(--bg-soft, rgba(30, 41, 59, 0.5));
  color: var(--text-muted, #94a3b8);
  border-color: var(--border-glass, rgba(255, 255, 255, 0.1));
}

.icon-btn-secondary:hover:not(:disabled) {
  background: var(--bg-hover, rgba(71, 85, 105, 0.8));
  color: #60a5fa;
  border-color: rgba(59, 130, 246, 0.4);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
}

/* Danger 样式（用于删除等） */
.icon-btn-danger {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border-color: rgba(244, 63, 94, 0.3);
}

.icon-btn-danger:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.32);
  color: #ffffff;
  border-color: rgba(244, 63, 94, 0.55);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(244, 63, 94, 0.3);
}

:global([data-theme='light']) .icon-btn-danger {
  color: #e11d48;
  background: rgba(244, 63, 94, 0.12);
}

:global([data-theme='light']) .icon-btn-danger:hover:not(:disabled) {
  background: rgba(244, 63, 94, 0.25);
  color: #be123c;
}

.icon-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  pointer-events: none;
}

/* Tooltip 浮动提示 */
.icon-btn-tooltip {
  position: absolute;
  left: 50%;
  transform: translateX(-50%) translateY(4px);
  background: rgba(15, 23, 42, 0.95);
  color: #f8fafc;
  font-size: 0.72rem;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 6px;
  white-space: nowrap;
  pointer-events: none;
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.15s ease, transform 0.15s ease, visibility 0.15s;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.14);
  backdrop-filter: blur(8px);
  z-index: 9999;
}

.icon-btn-tooltip.placement-top {
  bottom: calc(100% + 7px);
}

.icon-btn-tooltip.placement-top::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border-width: 4px;
  border-style: solid;
  border-color: rgba(15, 23, 42, 0.95) transparent transparent transparent;
}

.icon-btn-tooltip.placement-bottom {
  top: calc(100% + 7px);
  transform: translateX(-50%) translateY(-4px);
}

.icon-btn-tooltip.placement-bottom::after {
  content: '';
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  border-width: 4px;
  border-style: solid;
  border-color: transparent transparent rgba(15, 23, 42, 0.95) transparent;
}

.icon-btn:hover .icon-btn-tooltip {
  opacity: 1;
  visibility: visible;
  transform: translateX(-50%) translateY(0);
}
</style>
