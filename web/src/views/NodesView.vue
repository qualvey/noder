<script setup lang="ts">
// 节点管理：卡片列表 / 全选批量删除（新增/编辑表单在 NodeFormModal 组件）
import { computed, inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { Node } from '../types'
import NodeFormModal from '../components/NodeFormModal.vue'

const { t } = useI18n()
const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }
const updateMetrics = inject('metrics') as (nodes: number, users: number) => void

const nodes = ref<Node[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingNode = ref<Node | null>(null)
const loading = ref(true)
const searchQuery = ref('')

function matchesFuzzy(text: string, term: string): boolean {
  if (text.includes(term)) return true
  const cleanText = text.replace(/[-_\s.:/]/g, '')
  const cleanTerm = term.replace(/[-_\s.:/]/g, '')
  return cleanTerm.length > 0 && cleanText.includes(cleanTerm)
}

const filteredNodes = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return nodes.value

  const terms = query.split(/\s+/).filter(Boolean)

  return nodes.value.filter((node) => {
    const fields: string[] = [
      node.tag,
      node.node_name,
      node.protocol,
      node.server_address,
      node.server_port != null ? String(node.server_port) : '',
      node.security,
      node.sni,
      node.transport_type,
      node.path,
      node.congestion_control,
      node.remark,
    ]
      .filter((v): v is string => Boolean(v))
      .map((v) => v.toLowerCase())

    const fullText = fields.join(' ')

    return terms.every((term) => fields.some((f) => matchesFuzzy(f, term)) || matchesFuzzy(fullText, term))
  })
})

const isAllFilteredSelected = computed(() => {
  if (!filteredNodes.value.length) return false
  return filteredNodes.value.every((n) => selected.value.has(n.id))
})

async function fetchNodes(silent = false) {
  if (!silent && !nodes.value.length) {
    loading.value = true
  }
  try {
    nodes.value = await api.nodes.list()
    updateMetrics(nodes.value.length, nodes.value.length) // users 由 UsersView 覆盖
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingNode.value = null
  showModal.value = true
}

function openEdit(n: Node) {
  editingNode.value = n
  showModal.value = true
}

async function removeNode(id: number) {
  const prevNodes = [...nodes.value]
  // 乐观更新：直接剔除节点，触发 FLIP 平滑过渡与移位
  nodes.value = nodes.value.filter((n) => n.id !== id)
  if (selected.value.has(id)) {
    const s = new Set(selected.value)
    s.delete(id)
    selected.value = s
  }
  updateMetrics(nodes.value.length, nodes.value.length)

  try {
    await api.nodes.remove(id)
    toast(t('nodes.deleted'))
    // 静默对齐后端最新数据，不触发全屏加载与重绘跳动
    await fetchNodes(true)
  } catch (e) {
    // 异常时优雅回滚
    nodes.value = prevNodes
    updateMetrics(prevNodes.length, prevNodes.length)
    toast((e as Error).message, 'error')
  }
}

function toggleSelect(id: number) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleSelectAll() {
  const s = new Set(selected.value)
  if (isAllFilteredSelected.value) {
    for (const n of filteredNodes.value) {
      s.delete(n.id)
    }
  } else {
    for (const n of filteredNodes.value) {
      s.add(n.id)
    }
  }
  selected.value = s
}

function bulkDelete(e: MouseEvent) {
  const count = selected.value.size
  if (!count) return
  popover.show(e.currentTarget as Element, t('nodes.bulkDeleteConfirm', { count }), async () => {
    const idsToDelete = Array.from(selected.value)
    const prevNodes = [...nodes.value]

    // 乐观更新：批量从视图移除
    nodes.value = nodes.value.filter((n) => !selected.value.has(n.id))
    selected.value = new Set()
    updateMetrics(nodes.value.length, nodes.value.length)

    try {
      for (const id of idsToDelete) {
        try {
          await api.nodes.remove(id)
        } catch {
          /* 继续删下一个 */
        }
      }
      toast(t('nodes.bulkDeleteSuccess', { count }))
      await fetchNodes(true)
    } catch {
      nodes.value = prevNodes
      updateMetrics(prevNodes.length, prevNodes.length)
    }
  })
}

// 离开动画钩子：记录元素离开瞬间的绝对几何尺寸与位移，保证兄弟卡片丝滑重排 (FLIP)
function onBeforeLeave(el: Element) {
  const htmlEl = el as HTMLElement
  const rect = htmlEl.getBoundingClientRect()
  const parentRect = htmlEl.parentElement?.getBoundingClientRect()
  if (parentRect) {
    htmlEl.style.left = `${rect.left - parentRect.left}px`
    htmlEl.style.top = `${rect.top - parentRect.top}px`
    htmlEl.style.width = `${rect.width}px`
    htmlEl.style.height = `${rect.height}px`
  }
}

onMounted(() => fetchNodes())
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">{{ t('nodes.headerTitle') }}</div>
      <div class="section-actions">
        <div class="search-box">
          <svg class="search-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            class="search-input"
            :placeholder="t('nodes.searchPlaceholder')"
            @keydown.esc="searchQuery = ''"
          />
          <button
            v-if="searchQuery"
            type="button"
            class="search-clear-btn"
            :title="t('nodes.clearFilter')"
            @click="searchQuery = ''"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
          <span v-if="searchQuery.trim()" class="search-badge">
            {{ filteredNodes.length }}/{{ nodes.length }}
          </span>
        </div>

        <label
          style="display: flex; align-items: center; gap: 6px; font-size: 0.85rem; color: var(--text-muted); cursor: pointer; user-select: none">
          <input type="checkbox" :checked="isAllFilteredSelected"
            @change="toggleSelectAll" /> {{ t('nodes.selectAll') }}
        </label>
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">
          🗑️ {{ t('nodes.bulkDelete', { count: selected.size }) }}
        </button>
        <button class="btn btn-primary" @click="openCreate">
          <span style="font-size: 1.1rem; line-height: 1">+</span> {{ t('nodes.addNode') }}
        </button>
      </div>
    </div>

    <div v-if="loading && !nodes.length" class="loading-container">
      <!-- SVG 转圈 -->
      <svg class="spinner" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
        <circle class="spinner-track" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="spinner-head" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
      </svg>
      <span class="loading-text">{{ t('nodes.loadingData') }}</span>
    </div>
    <div v-else-if="!nodes.length"
      style="grid-column: 1/-1; text-align: center; padding: 40px; color: var(--text-muted)">
      {{ t('nodes.emptyText') }}
    </div>
    <div v-else-if="!filteredNodes.length"
      style="text-align: center; padding: 48px 20px; color: var(--text-muted)">
      <div style="font-size: 1.8rem; margin-bottom: 8px">🔍</div>
      <div style="font-size: 0.95rem; margin-bottom: 12px">{{ t('nodes.noMatchingNodes') }}</div>
      <button class="btn btn-secondary btn-sm" @click="searchQuery = ''">
        {{ t('nodes.clearFilter') }}
      </button>
    </div>
    <TransitionGroup
      v-else
      name="node-list"
      tag="div"
      class="cards-grid"
      @before-leave="onBeforeLeave"
    >
      <div v-for="node in filteredNodes" :key="node.id" class="node-card" :class="{ selected: selected.has(node.id) }">
        <div class="node-card-header">
          <div style="display: flex; align-items: center; gap: 8px">
            <input type="checkbox" :checked="selected.has(node.id)" @change="toggleSelect(node.id)"
              style="cursor: pointer" />
            <div class="node-title">{{ node.tag || node.node_name }}</div>
          </div>
          <span class="badge" :class="`badge-${node.protocol}`">{{ node.protocol?.toUpperCase() || '' }}</span>
        </div>
        <div class="node-details">
          <div class="detail-row">
            <span>{{ t('nodes.form.serverAddress') }}</span>
            <span class="value">{{ node.server_address }}:{{ node.server_port }}</span>
          </div>
          <div class="detail-row">
            <span>{{ t('nodes.form.security') }}</span>
            <span class="value">{{ node.security }}<template v-if="node.sni"> / {{ node.sni }}</template></span>
          </div>
          <div v-if="node.transport_type" class="detail-row">
            <span>{{ t('nodes.form.transport') }}</span>
            <span class="value">{{ node.transport_type }}<template v-if="node.path"> / {{ node.path }}</template></span>
          </div>
          <div v-if="node.congestion_control" class="detail-row">
            <span>{{ t('nodes.form.congestionControl') }}</span>
            <span class="value">{{ node.congestion_control }}</span>
          </div>
          <div v-if="node.remark" class="detail-row">
            <span>{{ t('nodes.form.remark') }}</span>
            <span class="value">{{ node.remark }}</span>
          </div>
        </div>
        <div class="node-card-actions">
          <IconButton icon="edit" :tip="t('common.edit')" variant="secondary" @click="openEdit(node)" />
          <IconButton icon="delete" :tip="t('common.delete')" variant="danger"
            @click="popover.show($event.currentTarget as Element, t('nodes.deleteNodeConfirm'), () => removeNode(node.id))" />
        </div>
      </div>
    </TransitionGroup>
  </section>

  <NodeFormModal :open="showModal" :editing="editingNode" @close="showModal = false" @saved="() => fetchNodes(true)" />
</template>

<style scoped>
.section-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
  background: var(--bg-input);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 0 10px;
  height: 34px;
  transition: all 0.2s ease;
}

.search-box:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.search-icon {
  color: var(--text-muted);
  flex-shrink: 0;
  margin-right: 8px;
}

.search-input {
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-main);
  font-size: 0.85rem;
  width: 220px;
  min-width: 140px;
}

.search-input::placeholder {
  color: var(--text-dim);
}

.search-clear-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border-radius: 50%;
  margin-left: 4px;
  transition: color 0.15s, background-color 0.15s;
}

.search-clear-btn:hover {
  color: var(--text-main);
  background-color: var(--bg-hover);
}

.search-badge {
  font-size: 0.72rem;
  color: var(--primary);
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.25);
  border-radius: 10px;
  padding: 1px 6px;
  margin-left: 6px;
  white-space: nowrap;
  font-family: var(--font-mono);
}

@media (max-width: 640px) {
  .section-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }
  .search-box {
    width: 100%;
  }
  .search-input {
    width: 100%;
  }
}
</style>
