<script setup lang="ts">
// 通用右键菜单：fixed 定位在光标处，视口边缘自动收拢
// 契约：props { x, y, items, title? }，emits { close }
// 打开方需在触发元素上 @contextmenu.prevent + stopPropagation，避免被文档级监听立即关闭
import { nextTick, onMounted, onUnmounted, ref } from 'vue'

export interface ContextMenuItem {
  label: string
  icon?: string
  danger?: boolean
  onClick: () => void
}

const props = withDefaults(
  defineProps<{
    x: number
    y: number
    items: ContextMenuItem[]
    title?: string
  }>(),
  { title: '' },
)

const emit = defineEmits<{ close: [] }>()

const menuEl = ref<HTMLElement | null>(null)
const pos = ref({ x: props.x, y: props.y })

// 视口边缘收拢（贴右/下边）
function clamp() {
  const el = menuEl.value
  if (!el) return
  const r = el.getBoundingClientRect()
  const pad = 8
  pos.value = {
    x: Math.min(props.x, window.innerWidth - r.width - pad),
    y: Math.min(props.y, window.innerHeight - r.height - pad),
  }
}

function onDocClick() {
  emit('close')
}

function onDocContext(e: MouseEvent) {
  if (!menuEl.value?.contains(e.target as Node)) emit('close')
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

function onScroll() {
  emit('close')
}

onMounted(async () => {
  await nextTick()
  clamp()
  document.addEventListener('click', onDocClick)
  document.addEventListener('contextmenu', onDocContext)
  window.addEventListener('keydown', onKey)
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', onScroll)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('contextmenu', onDocContext)
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', onScroll)
})
</script>

<template>
  <div
    ref="menuEl"
    class="context-menu"
    :style="{ left: pos.x + 'px', top: pos.y + 'px' }"
    @contextmenu.prevent
  >
    <div v-if="title" class="context-menu-title">{{ title }}</div>
    <button
      v-for="item in items"
      :key="item.label"
      type="button"
      class="context-menu-item"
      :class="{ danger: item.danger }"
      @click="item.onClick()"
    >
      <span v-if="item.icon" class="context-menu-icon">{{ item.icon }}</span>
      {{ item.label }}
    </button>
  </div>
</template>

<style scoped>
.context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 200px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 6px;
  box-shadow: var(--shadow-lg);
}

.context-menu-title {
  font-size: 0.72rem;
  color: var(--text-muted);
  padding: 4px 10px 8px;
  border-bottom: 1px solid var(--border-glass);
  margin-bottom: 4px;
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.context-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  color: var(--text-main);
  padding: 8px 10px;
  font-size: 0.85rem;
  border-radius: 6px;
  cursor: pointer;
}

.context-menu-item:hover {
  background: rgba(59, 130, 246, 0.15);
}

.context-menu-item.danger {
  color: var(--accent-rose);
}

.context-menu-icon {
  font-size: 0.9rem;
}
</style>
