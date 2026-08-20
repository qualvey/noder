<script setup lang="ts">
// 节点管理：卡片列表 / 全选批量删除（新增/编辑表单在 NodeFormModal 组件）
import { inject, onMounted, ref } from 'vue'
import { api } from '../api'
import type { Node } from '../types'
import NodeFormModal from '../components/NodeFormModal.vue'

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }
const updateMetrics = inject('metrics') as (nodes: number, users: number) => void

const nodes = ref<Node[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingNode = ref<Node | null>(null)
const loading = ref(false) // 如果组件挂载时立即请求，可初始化为 true

const sleep = (ms: number): Promise<void> => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};
async function fetchNodes() {
  loading.value = true
  await sleep(2000);
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

function removeNode(id: number) {
  api.nodes.remove(id).then(() => {
    toast('节点已删除')
    fetchNodes()
  }).catch((e) => toast(e.message, 'error'))
}

function toggleSelect(id: number) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleSelectAll() {
  if (selected.value.size === nodes.value.length) selected.value = new Set()
  else selected.value = new Set(nodes.value.map((n) => n.id))
}

function bulkDelete(e: MouseEvent) {
  const count = selected.value.size
  if (!count) return
  popover.show(e.currentTarget as Element, `⚠️ 确定批量删除已选中的 ${count} 个节点？`, async () => {
    for (const id of selected.value) {
      try {
        await api.nodes.remove(id)
      } catch {
        /* 继续删下一个 */
      }
    }
    selected.value = new Set()
    toast(`已批量删除 ${count} 个节点`)
    fetchNodes()
  })
}

onMounted(fetchNodes)
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">代理节点列表 (支持 TUIC / VLESS REALITY / AnyTLS)</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <label
          style="display: flex; align-items: center; gap: 6px; font-size: 0.82rem; color: var(--text-muted); cursor: pointer">
          <input type="checkbox" :checked="selected.size === nodes.length && nodes.length > 0"
            @change="toggleSelectAll" /> 全选节点
        </label>
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">
          🗑️ 批量删除 ({{ selected.size }})
        </button>
        <button class="btn btn-primary" @click="openCreate"><span>+</span> 新增节点</button>
      </div>
    </div>

    <div class="cards-grid">
      <div v-if="loading" class="loading-container">
        <!-- SVG 转圈 -->
        <svg class="spinner" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="spinner-track" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="spinner-head" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
        </svg>
        <span class="loading-text">正在加载节点数据...</span>
      </div>
      <div v-else-if="!nodes.length"
        style="grid-column: 1/-1; text-align: center; padding: 40px; color: var(--text-muted)">
        暂无代理节点，请点击右上角新增节点
      </div>
      <div v-for="node in nodes" :key="node.id" class="node-card" :class="{ selected: selected.has(node.id) }">
        <div class="node-card-header">
          <div style="display: flex; align-items: center; gap: 8px">
            <input type="checkbox" :checked="selected.has(node.id)" @change="toggleSelect(node.id)"
              style="cursor: pointer" />
            <div class="node-title">{{ node.tag || node.node_name }}</div>
          </div>
          <span class="badge" :class="`badge-${node.protocol}`">{{ node.protocol.toUpperCase() }}</span>
        </div>
        <div class="node-details">
          <div class="detail-row"><span>服务器地址:</span><span class="value">{{ node.server_address }}:{{ node.server_port
              }}</span></div>
          <div class="detail-row"><span>安全模式:</span><span class="value">{{ node.security }}<template v-if="node.sni"> /
                {{ node.sni }}</template></span>
          </div>
          <div class="detail-row"><span>传输方式:</span><span class="value">{{ node.transport_type }}<template
                v-if="node.path"> / {{ node.path }}</template></span></div>
          <div v-if="node.remark" class="detail-row"><span>备注:</span><span class="value">{{ node.remark }}</span></div>
        </div>
        <div class="node-card-actions">
          <button class="btn btn-secondary btn-sm" @click="openEdit(node)">编辑</button>
          <button class="btn btn-danger btn-sm"
            @click="popover.show($event.currentTarget as Element, '⚠️ 确定删除该节点？', () => removeNode(node.id))">删除</button>
        </div>
      </div>
    </div>
  </section>

  <NodeFormModal :open="showModal" :editing="editingNode" @close="showModal = false" @saved="fetchNodes" />
</template>
